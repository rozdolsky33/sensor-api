package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	// The error is deliberately ignored: the status line is already sent, so
	// there is nothing more we can do with an encode failure at this point.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, fields []FieldError) {
	writeJSON(w, status, ErrorResponse{Error: ErrorBody{Code: code, Message: message, Fields: fields}})
}

// writeErr maps any error coming out of decoding or the domain to an HTTP response.
func (h *handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var (
		reqErr *requestError
		valErr *sensor.ValidationError
	)
	switch {
	case errors.As(err, &reqErr):
		writeError(w, reqErr.status, reqErr.code, reqErr.message, nil)
	case errors.As(err, &valErr):
		writeError(w, http.StatusBadRequest, "validation_failed", "request is invalid", toFieldErrors(valErr.Fields))
	case errors.Is(err, sensor.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "sensor not found", nil)
	case errors.Is(err, sensor.ErrNoSensors):
		writeError(w, http.StatusNotFound, "not_found", "no sensors stored", nil)
	case errors.Is(err, sensor.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "a sensor with this name already exists", nil)
	case errors.Is(err, sensor.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "sensor was modified concurrently, retry the request", nil)
	default:
		h.log.LogAttrs(r.Context(), slog.LevelError, "unhandled error", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
	}
}

func toFieldErrors(in []sensor.FieldError) []FieldError {
	out := make([]FieldError, 0, len(in))
	for _, f := range in {
		out = append(out, FieldError{Field: f.Field, Message: f.Message})
	}
	return out
}
