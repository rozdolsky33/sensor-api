// Package httpapi exposes the sensor service over JSON/HTTP.
package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

type handler struct {
	svc *sensor.Service
	log *slog.Logger
}

// health godoc
//
//	@Summary	Liveness / readiness probe
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	HealthResponse
//	@Router		/healthz [get]
//	@Router		/readyz [get]
func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// createSensor godoc
//
//	@Summary		Create a sensor
//	@Description	Registers a new sensor. The name is the immutable identifier and must be unique.
//	@Tags			sensors
//	@Accept			json
//	@Produce		json
//	@Param			sensor	body		CreateSensorRequest	true	"Sensor to create"
//	@Success		201		{object}	SensorResponse
//	@Header			201		{string}	Location	"URL of the created sensor"
//	@Failure		400		{object}	ErrorResponse	"Validation failed or malformed JSON"
//	@Failure		409		{object}	ErrorResponse	"Name already exists"
//	@Failure		413		{object}	ErrorResponse
//	@Failure		415		{object}	ErrorResponse
//	@Router			/v1/sensors [post]
func (h *handler) createSensor(w http.ResponseWriter, r *http.Request) {
	var req CreateSensorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeErr(w, r, err)
		return
	}
	var ve sensor.ValidationError
	loc := req.Location.toLocation("location", &ve)
	if err := ve.Err(); err != nil {
		h.writeErr(w, r, err)
		return
	}
	created, err := h.svc.Create(r.Context(), sensor.CreateInput{Name: req.Name, Location: loc, Tags: req.Tags})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/sensors/"+created.Name)
	writeJSON(w, http.StatusCreated, toSensorResponse(created))
}

// getSensor godoc
//
//	@Summary	Get a sensor by name
//	@Tags		sensors
//	@Produce	json
//	@Param		name	path		string	true	"Sensor name"
//	@Success	200		{object}	SensorResponse
//	@Failure	404		{object}	ErrorResponse
//	@Router		/v1/sensors/{name} [get]
func (h *handler) getSensor(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context(), r.PathValue("name"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSensorResponse(s))
}

func (h *handler) notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "resource not found", nil)
}

// methodNotAllowed returns a handler that advertises the allowed methods.
func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed; see Allow header", nil)
	}
}

// listSensors godoc
//
//	@Summary	List sensors
//	@Tags		sensors
//	@Produce	json
//	@Param		tag	query		string	false	"Only sensors carrying this tag"
//	@Success	200	{object}	SensorListResponse
//	@Router		/v1/sensors [get]
func (h *handler) listSensors(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), sensor.ListFilter{Tag: strings.TrimSpace(r.URL.Query().Get("tag"))})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	out := make([]SensorResponse, 0, len(list))
	for _, s := range list {
		out = append(out, toSensorResponse(s))
	}
	writeJSON(w, http.StatusOK, SensorListResponse{Sensors: out})
}

// replaceSensor godoc
//
//	@Summary		Replace a sensor's metadata
//	@Description	Full replacement of location and tags. Omitted tags are cleared. The name cannot be changed.
//	@Tags			sensors
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string					true	"Sensor name"
//	@Param			sensor	body		ReplaceSensorRequest	true	"New metadata"
//	@Success		200		{object}	SensorResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse	"Concurrent modification"
//	@Failure		415		{object}	ErrorResponse
//	@Router			/v1/sensors/{name} [put]
func (h *handler) replaceSensor(w http.ResponseWriter, r *http.Request) {
	var req ReplaceSensorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeErr(w, r, err)
		return
	}
	var ve sensor.ValidationError
	loc := req.Location.toLocation("location", &ve)
	if err := ve.Err(); err != nil {
		h.writeErr(w, r, err)
		return
	}
	updated, err := h.svc.Replace(r.Context(), r.PathValue("name"), sensor.ReplaceInput{Location: loc, Tags: req.Tags})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSensorResponse(updated))
}

// patchSensor godoc
//
//	@Summary		Partially update a sensor
//	@Description	Updates only the fields present in the body. At least one field is required.
//	@Tags			sensors
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string				true	"Sensor name"
//	@Param			sensor	body		PatchSensorRequest	true	"Fields to change"
//	@Success		200		{object}	SensorResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse	"Concurrent modification"
//	@Failure		415		{object}	ErrorResponse
//	@Router			/v1/sensors/{name} [patch]
func (h *handler) patchSensor(w http.ResponseWriter, r *http.Request) {
	var req PatchSensorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeErr(w, r, err)
		return
	}
	in := sensor.PatchInput{Tags: req.Tags}
	if req.Location != nil {
		var ve sensor.ValidationError
		loc := req.Location.toLocation("location", &ve)
		if err := ve.Err(); err != nil {
			h.writeErr(w, r, err)
			return
		}
		in.Location = &loc
	}
	updated, err := h.svc.Patch(r.Context(), r.PathValue("name"), in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSensorResponse(updated))
}

// deleteSensor godoc
//
//	@Summary	Delete a sensor
//	@Tags		sensors
//	@Param		name	path	string	true	"Sensor name"
//	@Success	204	"No Content"
//	@Failure	404	{object}	ErrorResponse
//	@Router		/v1/sensors/{name} [delete]
func (h *handler) deleteSensor(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("name")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// nearestSensor godoc
//
//	@Summary		Find the nearest sensor
//	@Description	Returns the sensor closest to the given coordinate (great-circle distance) and the distance in metres.
//	@Tags			sensors
//	@Produce		json
//	@Param			lat	query		number	true	"Latitude in decimal degrees"	minimum(-90)	maximum(90)
//	@Param			lon	query		number	true	"Longitude in decimal degrees"	minimum(-180)	maximum(180)
//	@Success		200	{object}	NearestSensorResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse	"No sensors stored"
//	@Router			/v1/sensors/nearest [get]
func (h *handler) nearestSensor(w http.ResponseWriter, r *http.Request) {
	var ve sensor.ValidationError
	q := r.URL.Query()
	lat := parseCoordinate(&ve, "lat", q.Get("lat"))
	lon := parseCoordinate(&ve, "lon", q.Get("lon"))
	if err := ve.Err(); err != nil {
		h.writeErr(w, r, err)
		return
	}
	nearest, dist, err := h.svc.Nearest(r.Context(), sensor.Location{Latitude: lat, Longitude: lon})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, NearestSensorResponse{Sensor: toSensorResponse(nearest), DistanceMeters: dist})
}

// parseCoordinate parses a query parameter, recording "required" / "not a number" errors.
// Range checks are left to the service so the rules live in one place.
func parseCoordinate(ve *sensor.ValidationError, field, raw string) float64 {
	if raw == "" {
		ve.Add(field, "is required")
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		ve.Add(field, "must be a number")
		return 0
	}
	return v
}
