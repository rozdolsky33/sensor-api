package httpapi

import (
	"log/slog"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	"github.com/volodymyrrozdolsky/sensor-api/internal/httpapi/middleware"
	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

// NewRouter builds the full HTTP handler: routes plus middleware.
func NewRouter(svc *sensor.Service, logger *slog.Logger) http.Handler {
	h := &handler{svc: svc, log: logger}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.health)

	mux.HandleFunc("POST /v1/sensors", h.createSensor)
	mux.HandleFunc("GET /v1/sensors", h.listSensors)
	mux.HandleFunc("GET /v1/sensors/nearest", h.nearestSensor) // literal segment beats {name}
	mux.HandleFunc("GET /v1/sensors/{name}", h.getSensor)
	mux.HandleFunc("PUT /v1/sensors/{name}", h.replaceSensor)
	mux.HandleFunc("PATCH /v1/sensors/{name}", h.patchSensor)
	mux.HandleFunc("DELETE /v1/sensors/{name}", h.deleteSensor)

	// Method-less fallbacks turn the mux's plain-text 405 into our JSON envelope.
	mux.Handle("/v1/sensors", methodNotAllowed("GET, POST"))
	mux.Handle("/v1/sensors/{name}", methodNotAllowed("GET, PUT, PATCH, DELETE"))

	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	mux.HandleFunc("/", h.notFound)

	return middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.RequestID,
		middleware.Logging(logger),
	)
}
