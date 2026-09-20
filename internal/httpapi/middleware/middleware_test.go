package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/volodymyrrozdolsky/sensor-api/internal/httpapi/middleware"
)

func TestChain_OrdersOuterToInner(t *testing.T) {
	var order []string
	tag := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := middleware.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "handler") }),
		tag("first"), tag("second"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if got := strings.Join(order, ","); got != "first,second,handler" {
		t.Fatalf("order = %s", got)
	}
}

func TestRequestID(t *testing.T) {
	var seen string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = middleware.RequestIDFrom(r.Context())
	}))

	t.Run("generates when absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
		if got := rec.Header().Get(middleware.RequestIDHeader); len(got) != 32 || got != seen {
			t.Fatalf("header = %q, ctx = %q", got, seen)
		}
	})

	t.Run("echoes client value", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.Header.Set(middleware.RequestIDHeader, "client-123")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get(middleware.RequestIDHeader); got != "client-123" || seen != "client-123" {
			t.Fatalf("header = %q, ctx = %q", got, seen)
		}
	})

	t.Run("replaces oversized client value", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.Header.Set(middleware.RequestIDHeader, strings.Repeat("x", 129))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get(middleware.RequestIDHeader); len(got) != 32 {
			t.Fatalf("header = %q", got)
		}
	})

	if middleware.RequestIDFrom(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil).Context()) != "" {
		t.Fatal("RequestIDFrom on bare context should be empty")
	}
}

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	h := middleware.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("short and stout"))
	}), middleware.RequestID, middleware.Logging(logger))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/sensors?x=1", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %s", buf.String())
	}
	if entry["method"] != "POST" || entry["path"] != "/v1/sensors" || entry["status"] != float64(418) || entry["bytes"] != float64(15) {
		t.Fatalf("entry = %v", entry)
	}
	if id, _ := entry["request_id"].(string); len(id) != 32 {
		t.Fatalf("request_id = %v", entry["request_id"])
	}
}

func TestLogging_DefaultsTo200WhenHandlerOnlyWrites(t *testing.T) {
	var buf bytes.Buffer
	h := middleware.Logging(slog.New(slog.NewJSONHandler(&buf, nil)))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !strings.Contains(buf.String(), `"status":200`) {
		t.Fatalf("log = %s", buf.String())
	}
}

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	h := middleware.Recover(slog.New(slog.NewJSONHandler(&buf, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != `{"error":{"code":"internal","message":"internal server error"}}`+"\n" {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Fatalf("panic not logged: %s", buf.String())
	}
}

func TestRecover_RethrowsAbortHandler(t *testing.T) {
	h := middleware.Recover(slog.New(slog.NewJSONHandler(io.Discard, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if r := recover(); r == nil || !errors.Is(r.(error), http.ErrAbortHandler) {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", r)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
}
