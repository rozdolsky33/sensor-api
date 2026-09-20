package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/volodymyrrozdolsky/sensor-api/internal/httpapi"
)

func TestEndToEnd_SensorLifecycle(t *testing.T) {
	srv := httptest.NewServer(newRouter(t))
	defer srv.Close()
	client := srv.Client()

	call := func(method, path, body string) (status int, header http.Header, respBody []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, b
	}

	// create
	status, header, body := call(http.MethodPost, "/v1/sensors", londonJSON)
	if status != http.StatusCreated {
		t.Fatalf("create: %d %s", status, body)
	}
	if header.Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}

	// get
	status, _, body = call(http.MethodGet, header.Get("Location"), "")
	if status != http.StatusOK {
		t.Fatalf("get: %d %s", status, body)
	}

	// patch
	status, _, body = call(http.MethodPatch, "/v1/sensors/london", `{"tags":["patched"]}`)
	var patched httpapi.SensorResponse
	if status != http.StatusOK || json.Unmarshal(body, &patched) != nil || strings.Join(patched.Tags, ",") != "patched" {
		t.Fatalf("patch: %d %s", status, body)
	}

	// nearest
	status, _, body = call(http.MethodGet, "/v1/sensors/nearest?lat=51.5&lon=-0.12", "")
	var nearest httpapi.NearestSensorResponse
	if status != http.StatusOK || json.Unmarshal(body, &nearest) != nil || nearest.Sensor.Name != "london" {
		t.Fatalf("nearest: %d %s", status, body)
	}

	// list
	status, _, body = call(http.MethodGet, "/v1/sensors?tag=patched", "")
	var list httpapi.SensorListResponse
	if status != http.StatusOK || json.Unmarshal(body, &list) != nil || len(list.Sensors) != 1 {
		t.Fatalf("list: %d %s", status, body)
	}

	// delete
	status, _, _ = call(http.MethodDelete, "/v1/sensors/london", "")
	if status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	status, _, _ = call(http.MethodGet, "/v1/sensors/london", "")
	if status != http.StatusNotFound {
		t.Fatalf("get after delete: %d", status)
	}
}
