package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/volodymyrrozdolsky/sensor-api/docs"
	"github.com/volodymyrrozdolsky/sensor-api/internal/httpapi"
	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
	"github.com/volodymyrrozdolsky/sensor-api/internal/storage/memory"
)

var fixedTime = time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	svc := sensor.NewService(memory.New(), func() time.Time { return fixedTime })
	return httpapi.NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// do performs a request; a non-empty body is sent as application/json.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body is not JSON (%v): %s", err, rec.Body.String())
	}
	return v
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, want, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); rec.Code != http.StatusNoContent && ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string, fields ...string) {
	t.Helper()
	assertStatus(t, rec, status)
	body := decodeBody[httpapi.ErrorResponse](t, rec)
	if body.Error.Code != code {
		t.Fatalf("code = %q, want %q; body: %s", body.Error.Code, code, rec.Body.String())
	}
	got := make([]string, 0, len(body.Error.Fields))
	for _, f := range body.Error.Fields {
		got = append(got, f.Field)
	}
	if strings.Join(got, ",") != strings.Join(fields, ",") {
		t.Fatalf("fields = %v, want %v", got, fields)
	}
}

const londonJSON = `{"name":"london","location":{"latitude":51.5074,"longitude":-0.1278},"tags":["city","uk","city"]}`

func createLondon(t *testing.T, h http.Handler) {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/v1/sensors", londonJSON)
	assertStatus(t, rec, http.StatusCreated)
}

// failingRepo wraps a memory.Store so Get and Update can be forced to fail,
// to exercise the untested rows of handler.writeErr (ErrConflict -> 409,
// and an arbitrary error -> 500 without leaking its message).
type failingRepo struct {
	*memory.Store
	failUpdate bool
	failGet    bool
}

func (f *failingRepo) Update(ctx context.Context, s sensor.Sensor, expectedUpdatedAt time.Time) error {
	if f.failUpdate {
		return sensor.ErrConflict
	}
	return f.Store.Update(ctx, s, expectedUpdatedAt)
}

func (f *failingRepo) Get(ctx context.Context, name string) (sensor.Sensor, error) {
	if f.failGet {
		return sensor.Sensor{}, errors.New("boom")
	}
	return f.Store.Get(ctx, name)
}

func TestWriteErr_Conflict(t *testing.T) {
	stub := &failingRepo{Store: memory.New(), failUpdate: true}
	h := httpapi.NewRouter(sensor.NewService(stub, func() time.Time { return fixedTime }), slog.New(slog.NewTextHandler(io.Discard, nil)))
	createLondon(t, h)

	rec := do(t, h, http.MethodPatch, "/v1/sensors/london", `{"tags":["x"]}`)
	assertError(t, rec, http.StatusConflict, "conflict")
}

func TestWriteErr_Internal(t *testing.T) {
	stub := &failingRepo{Store: memory.New(), failGet: true}
	h := httpapi.NewRouter(sensor.NewService(stub, func() time.Time { return fixedTime }), slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := do(t, h, http.MethodGet, "/v1/sensors/london", "")
	assertError(t, rec, http.StatusInternalServerError, "internal")
	body := decodeBody[httpapi.ErrorResponse](t, rec)
	if body.Error.Message != "internal server error" {
		t.Fatalf("message = %q", body.Error.Message)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("body leaks internal error: %s", rec.Body.String())
	}
}

func TestHealth(t *testing.T) {
	h := newRouter(t)
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := do(t, h, http.MethodGet, path, "")
		assertStatus(t, rec, http.StatusOK)
		if rec.Body.String() != `{"status":"ok"}`+"\n" {
			t.Fatalf("%s body = %s", path, rec.Body.String())
		}
	}
}

func TestCreateSensor(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPost, "/v1/sensors", londonJSON)
	assertStatus(t, rec, http.StatusCreated)
	if loc := rec.Header().Get("Location"); loc != "/v1/sensors/london" {
		t.Fatalf("Location = %q", loc)
	}
	got := decodeBody[httpapi.SensorResponse](t, rec)
	want := httpapi.SensorResponse{
		Name:      "london",
		Location:  httpapi.LocationResponse{Latitude: 51.5074, Longitude: -0.1278},
		Tags:      []string{"city", "uk"},
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}
	if got.Name != want.Name || got.Location != want.Location || strings.Join(got.Tags, ",") != "city,uk" ||
		!got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !strings.Contains(rec.Body.String(), `"created_at":"2026-09-18T10:00:00Z"`) {
		t.Fatalf("timestamps not RFC3339 UTC: %s", rec.Body.String())
	}
}

func TestCreateSensor_TagsOmittedBecomesEmptyArray(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPost, "/v1/sensors", `{"name":"a","location":{"latitude":0,"longitude":0}}`)
	assertStatus(t, rec, http.StatusCreated)
	if !strings.Contains(rec.Body.String(), `"tags":[]`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestCreateSensor_Errors(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		status      int
		code        string
		fields      []string
	}{
		{"invalid latitude", `{"name":"a","location":{"latitude":91,"longitude":0}}`, "application/json", 400, "validation_failed", []string{"location.latitude"}},
		{"invalid name and tags", `{"name":"bad name","location":{"latitude":0,"longitude":0},"tags":[""]}`, "application/json", 400, "validation_failed", []string{"name", "tags[0]"}},
		{"reserved name", `{"name":"nearest","location":{"latitude":0,"longitude":0}}`, "application/json", 400, "validation_failed", []string{"name"}},
		{"missing location", `{"name":"a"}`, "application/json", 400, "validation_failed", []string{"location"}},
		{"missing longitude", `{"name":"a","location":{"latitude":1}}`, "application/json", 400, "validation_failed", []string{"location.longitude"}},
		{"unknown field", `{"name":"a","location":{"latitude":0,"longitude":0},"colour":"red"}`, "application/json", 400, "invalid_json", nil},
		{"wrong type", `{"name":"a","location":{"latitude":"north","longitude":0}}`, "application/json", 400, "invalid_json", nil},
		{"malformed", `{"name":`, "application/json", 400, "invalid_json", nil},
		{"empty body", ``, "application/json", 400, "invalid_json", nil},
		{"two documents", `{"name":"a","location":{"latitude":0,"longitude":0}} {}`, "application/json", 400, "invalid_json", nil},
		{"wrong content type", londonJSON, "text/plain", 415, "unsupported_media_type", nil},
		{"no content type", londonJSON, "", 415, "unsupported_media_type", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRouter(t)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/sensors", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assertError(t, rec, tt.status, tt.code, tt.fields...)
		})
	}
}

func TestCreateSensor_ContentTypeWithCharset(t *testing.T) {
	h := newRouter(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/sensors", strings.NewReader(londonJSON))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assertStatus(t, rec, http.StatusCreated)
}

func TestCreateSensor_Duplicate(t *testing.T) {
	h := newRouter(t)
	createLondon(t, h)
	rec := do(t, h, http.MethodPost, "/v1/sensors", londonJSON)
	assertError(t, rec, http.StatusConflict, "already_exists")
}

func TestCreateSensor_PayloadTooLarge(t *testing.T) {
	h := newRouter(t)
	huge := `{"name":"a","location":{"latitude":0,"longitude":0},"tags":["` + strings.Repeat("x", 1<<20) + `"]}`
	rec := do(t, h, http.MethodPost, "/v1/sensors", huge)
	assertError(t, rec, http.StatusRequestEntityTooLarge, "payload_too_large")
}

func TestGetSensor(t *testing.T) {
	h := newRouter(t)
	createLondon(t, h)

	rec := do(t, h, http.MethodGet, "/v1/sensors/london", "")
	assertStatus(t, rec, http.StatusOK)
	if got := decodeBody[httpapi.SensorResponse](t, rec); got.Name != "london" {
		t.Fatalf("got %+v", got)
	}

	assertError(t, do(t, h, http.MethodGet, "/v1/sensors/missing", ""), http.StatusNotFound, "not_found")
}

func TestUnknownRoute(t *testing.T) {
	h := newRouter(t)
	assertError(t, do(t, h, http.MethodGet, "/nope", ""), http.StatusNotFound, "not_found")
	assertError(t, do(t, h, http.MethodGet, "/v1/sensors/london/extra", ""), http.StatusNotFound, "not_found")
}

func TestMethodNotAllowed(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPut, "/v1/sensors", `{}`)
	assertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if allow := rec.Header().Get("Allow"); allow != "GET, POST" {
		t.Fatalf("Allow = %q", allow)
	}
	rec = do(t, h, http.MethodPost, "/v1/sensors/london", `{}`)
	assertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if allow := rec.Header().Get("Allow"); allow != "GET, PUT, PATCH, DELETE" {
		t.Fatalf("Allow = %q", allow)
	}
}

func TestRequestIDHeader(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodGet, "/healthz", "")
	if len(rec.Header().Get("X-Request-ID")) != 32 {
		t.Fatalf("X-Request-ID = %q", rec.Header().Get("X-Request-ID"))
	}
}

func TestListSensors(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodGet, "/v1/sensors", "")
	assertStatus(t, rec, http.StatusOK)
	if rec.Body.String() != `{"sensors":[]}`+"\n" {
		t.Fatalf("empty list body = %s", rec.Body.String())
	}

	createLondon(t, h)
	do(t, h, http.MethodPost, "/v1/sensors", `{"name":"berlin","location":{"latitude":52.52,"longitude":13.405},"tags":["city","de"]}`)

	all := decodeBody[httpapi.SensorListResponse](t, do(t, h, http.MethodGet, "/v1/sensors", ""))
	if len(all.Sensors) != 2 || all.Sensors[0].Name != "berlin" || all.Sensors[1].Name != "london" {
		t.Fatalf("list = %+v", all.Sensors)
	}

	uk := decodeBody[httpapi.SensorListResponse](t, do(t, h, http.MethodGet, "/v1/sensors?tag=uk", ""))
	if len(uk.Sensors) != 1 || uk.Sensors[0].Name != "london" {
		t.Fatalf("list?tag=uk = %+v", uk.Sensors)
	}
}

func TestReplaceSensor(t *testing.T) {
	h := newRouter(t)
	createLondon(t, h)

	rec := do(t, h, http.MethodPut, "/v1/sensors/london", `{"location":{"latitude":1,"longitude":2},"tags":["moved"]}`)
	assertStatus(t, rec, http.StatusOK)
	got := decodeBody[httpapi.SensorResponse](t, rec)
	if got.Location != (httpapi.LocationResponse{Latitude: 1, Longitude: 2}) || strings.Join(got.Tags, ",") != "moved" {
		t.Fatalf("got %+v", got)
	}

	rec = do(t, h, http.MethodPut, "/v1/sensors/london", `{"location":{"latitude":1,"longitude":2}}`)
	assertStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"tags":[]`) {
		t.Fatalf("omitted tags should clear: %s", rec.Body.String())
	}

	assertError(t, do(t, h, http.MethodPut, "/v1/sensors/london", `{"name":"renamed","location":{"latitude":1,"longitude":2}}`),
		http.StatusBadRequest, "invalid_json")
	assertError(t, do(t, h, http.MethodPut, "/v1/sensors/london", `{"tags":["x"]}`),
		http.StatusBadRequest, "validation_failed", "location")
	assertError(t, do(t, h, http.MethodPut, "/v1/sensors/london", `{"location":{"latitude":95,"longitude":2}}`),
		http.StatusBadRequest, "validation_failed", "location.latitude")
	assertError(t, do(t, h, http.MethodPut, "/v1/sensors/missing", `{"location":{"latitude":1,"longitude":2}}`),
		http.StatusNotFound, "not_found")
}

func TestPatchSensor(t *testing.T) {
	h := newRouter(t)
	createLondon(t, h)

	rec := do(t, h, http.MethodPatch, "/v1/sensors/london", `{"location":{"latitude":1,"longitude":2}}`)
	assertStatus(t, rec, http.StatusOK)
	got := decodeBody[httpapi.SensorResponse](t, rec)
	if got.Location.Latitude != 1 || strings.Join(got.Tags, ",") != "city,uk" {
		t.Fatalf("location-only patch: %+v", got)
	}

	rec = do(t, h, http.MethodPatch, "/v1/sensors/london", `{"tags":[]}`)
	assertStatus(t, rec, http.StatusOK)
	got = decodeBody[httpapi.SensorResponse](t, rec)
	if got.Location.Latitude != 1 || len(got.Tags) != 0 {
		t.Fatalf("tags-only patch: %+v", got)
	}

	assertError(t, do(t, h, http.MethodPatch, "/v1/sensors/london", `{}`), http.StatusBadRequest, "validation_failed", "body")
	assertError(t, do(t, h, http.MethodPatch, "/v1/sensors/london", `{"location":{"latitude":1}}`),
		http.StatusBadRequest, "validation_failed", "location.longitude")
	assertError(t, do(t, h, http.MethodPatch, "/v1/sensors/london", `{"tags":[""]}`),
		http.StatusBadRequest, "validation_failed", "tags[0]")
	assertError(t, do(t, h, http.MethodPatch, "/v1/sensors/missing", `{"tags":["x"]}`), http.StatusNotFound, "not_found")
}

func TestDeleteSensor(t *testing.T) {
	h := newRouter(t)
	createLondon(t, h)

	rec := do(t, h, http.MethodDelete, "/v1/sensors/london", "")
	assertStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Fatalf("204 body = %s", rec.Body.String())
	}
	assertError(t, do(t, h, http.MethodGet, "/v1/sensors/london", ""), http.StatusNotFound, "not_found")
	assertError(t, do(t, h, http.MethodDelete, "/v1/sensors/london", ""), http.StatusNotFound, "not_found")
}

func TestNearestSensor(t *testing.T) {
	h := newRouter(t)
	assertError(t, do(t, h, http.MethodGet, "/v1/sensors/nearest?lat=0&lon=0", ""), http.StatusNotFound, "not_found")

	createLondon(t, h)
	do(t, h, http.MethodPost, "/v1/sensors", `{"name":"berlin","location":{"latitude":52.52,"longitude":13.405}}`)

	rec := do(t, h, http.MethodGet, "/v1/sensors/nearest?lat=52.5&lon=13.4", "")
	assertStatus(t, rec, http.StatusOK)
	got := decodeBody[httpapi.NearestSensorResponse](t, rec)
	if got.Sensor.Name != "berlin" || got.DistanceMeters <= 0 || got.DistanceMeters > 5000 {
		t.Fatalf("got %+v", got)
	}

	tests := []struct {
		name   string
		query  string
		fields []string
	}{
		{"missing both", "", []string{"lat", "lon"}},
		{"missing lon", "?lat=1", []string{"lon"}},
		{"non-numeric", "?lat=abc&lon=1", []string{"lat"}},
		{"out of range", "?lat=91&lon=181", []string{"lat", "lon"}},
		{"nan", "?lat=NaN&lon=0", []string{"lat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, do(t, h, http.MethodGet, "/v1/sensors/nearest"+tt.query, ""),
				http.StatusBadRequest, "validation_failed", tt.fields...)
		})
	}
}

func TestNearestIsNotShadowedByNameRoute(t *testing.T) {
	h := newRouter(t)
	// A sensor literally called "nearest" cannot be created, so GET /v1/sensors/nearest is always the query.
	assertError(t, do(t, h, http.MethodPost, "/v1/sensors", `{"name":"nearest","location":{"latitude":0,"longitude":0}}`),
		http.StatusBadRequest, "validation_failed", "name")
	assertError(t, do(t, h, http.MethodGet, "/v1/sensors/nearest", ""), http.StatusBadRequest, "validation_failed", "lat", "lon")
}

func TestSwagger(t *testing.T) {
	h := newRouter(t)

	rec := do(t, h, http.MethodGet, "/swagger/doc.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("doc.json status = %d", rec.Code)
	}
	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("doc.json is not JSON: %v", err)
	}
	for _, p := range []string{"/v1/sensors", "/v1/sensors/{name}", "/v1/sensors/nearest", "/healthz"} {
		if _, ok := spec.Paths[p]; !ok {
			t.Errorf("spec missing path %s", p)
		}
	}

	rec = do(t, h, http.MethodGet, "/swagger/index.html", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Fatalf("index.html status = %d", rec.Code)
	}
}
