package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRun_InvalidConfig(t *testing.T) {
	err := run(context.Background(), func(k string) string {
		if k == "PORT" {
			return "nope"
		}
		return ""
	}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Fatalf("err = %v", err)
	}
}

// freePort asks the kernel for an unused TCP port.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func TestRun_ServesAndShutsDownOnContextCancel(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, func(k string) string {
			if k == "PORT" {
				return port
			}
			return ""
		}, &out)
	}()

	// Poll until the server accepts connections.
	var resp *http.Response
	for range 50 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.DefaultClient.Do(req)
		if err == nil {
			resp = r
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal("server never became ready")
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after cancel")
	}
	if !strings.Contains(out.String(), "server listening") || !strings.Contains(out.String(), "shutting down") {
		t.Fatalf("logs = %s", out.String())
	}
}
