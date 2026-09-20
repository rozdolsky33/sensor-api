package config_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/volodymyrrozdolsky/sensor-api/internal/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := config.Config{Port: 8080, LogLevel: slog.LevelInfo, ShutdownTimeout: 10 * time.Second, ReadHeaderTimeout: 5 * time.Second}
	if cfg != want {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{
		"PORT": "9090", "LOG_LEVEL": "debug", "SHUTDOWN_TIMEOUT": "3s", "READ_HEADER_TIMEOUT": "500ms",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := config.Config{Port: 9090, LogLevel: slog.LevelDebug, ShutdownTimeout: 3 * time.Second, ReadHeaderTimeout: 500 * time.Millisecond}
	if cfg != want {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Invalid(t *testing.T) {
	tests := []struct {
		key, value string
	}{
		{"PORT", "abc"}, {"PORT", "0"}, {"PORT", "70000"},
		{"LOG_LEVEL", "loud"},
		{"SHUTDOWN_TIMEOUT", "soon"}, {"SHUTDOWN_TIMEOUT", "-1s"}, {"SHUTDOWN_TIMEOUT", "0"},
		{"READ_HEADER_TIMEOUT", "5"},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			_, err := config.Load(env(map[string]string{tt.key: tt.value}))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("error %q does not mention %s", err, tt.key)
			}
		})
	}
}
