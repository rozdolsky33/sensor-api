// Command server runs the sensor metadata HTTP API.
//
//	@title			Sensor Metadata API
//	@version		1.0
//	@description	JSON REST API for storing and querying sensor metadata: name, GPS location and tags.
//	@BasePath		/
//	@accept			json
//	@produce		json
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/volodymyrrozdolsky/sensor-api/docs" // registers the generated OpenAPI spec
	"github.com/volodymyrrozdolsky/sensor-api/internal/config"
	"github.com/volodymyrrozdolsky/sensor-api/internal/httpapi"
	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
	"github.com/volodymyrrozdolsky/sensor-api/internal/storage/memory"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Getenv, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run starts the server and blocks until ctx is cancelled or the server fails.
func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	svc := sensor.NewService(memory.New(), nil)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewRouter(svc, logger),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
