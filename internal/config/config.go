// Package config loads server settings from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// Config holds runtime settings.
type Config struct {
	Port              int
	LogLevel          slog.Level
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
}

// Load reads configuration using getenv (normally os.Getenv), applying
// defaults for unset variables and rejecting malformed values.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:              8080,
		LogLevel:          slog.LevelInfo,
		ShutdownTimeout:   10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if v := getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("PORT: %q is not a valid port number", v)
		}
		cfg.Port = p
	}

	if v := getenv("LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}

	var err error
	if cfg.ShutdownTimeout, err = durationVar(getenv, "SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReadHeaderTimeout, err = durationVar(getenv, "READ_HEADER_TIMEOUT", cfg.ReadHeaderTimeout); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func durationVar(getenv func(string) string, name string, def time.Duration) (time.Duration, error) {
	v := getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive duration (e.g. \"10s\")", name, v)
	}
	return d, nil
}
