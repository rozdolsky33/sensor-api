package httpapi

import (
	"time"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

// LocationRequest is a GPS position in a request body. Pointers distinguish
// "missing" from zero.
type LocationRequest struct {
	Latitude  *float64 `json:"latitude" example:"51.5074"`
	Longitude *float64 `json:"longitude" example:"-0.1278"`
}

// toLocation converts the request, recording missing fields under prefix.
func (l *LocationRequest) toLocation(prefix string, ve *sensor.ValidationError) sensor.Location {
	if l == nil {
		ve.Add(prefix, "is required")
		return sensor.Location{}
	}
	var loc sensor.Location
	if l.Latitude == nil {
		ve.Add(prefix+".latitude", "is required")
	} else {
		loc.Latitude = *l.Latitude
	}
	if l.Longitude == nil {
		ve.Add(prefix+".longitude", "is required")
	} else {
		loc.Longitude = *l.Longitude
	}
	return loc
}

// CreateSensorRequest is the body of POST /v1/sensors.
type CreateSensorRequest struct {
	Name     string           `json:"name" example:"roof-temp-01"`
	Location *LocationRequest `json:"location"`
	Tags     []string         `json:"tags" example:"temperature,roof"`
}

// ReplaceSensorRequest is the body of PUT /v1/sensors/{name}.
type ReplaceSensorRequest struct {
	Location *LocationRequest `json:"location"`
	Tags     []string         `json:"tags" example:"temperature,roof"`
}

// PatchSensorRequest is the body of PATCH /v1/sensors/{name}; every field is optional.
type PatchSensorRequest struct {
	Location *LocationRequest `json:"location,omitempty"`
	Tags     *[]string        `json:"tags,omitempty" example:"temperature,roof"`
}

// LocationResponse is a GPS position in a response body.
type LocationResponse struct {
	Latitude  float64 `json:"latitude" example:"51.5074"`
	Longitude float64 `json:"longitude" example:"-0.1278"`
}

// SensorResponse is the public representation of a sensor.
type SensorResponse struct {
	Name      string           `json:"name" example:"roof-temp-01"`
	Location  LocationResponse `json:"location"`
	Tags      []string         `json:"tags" example:"temperature,roof"`
	CreatedAt time.Time        `json:"created_at" example:"2026-09-18T10:00:00Z"`
	UpdatedAt time.Time        `json:"updated_at" example:"2026-09-18T10:00:00Z"`
}

func toSensorResponse(s sensor.Sensor) SensorResponse {
	tags := s.Tags
	if tags == nil {
		tags = []string{}
	}
	return SensorResponse{
		Name:      s.Name,
		Location:  LocationResponse{Latitude: s.Location.Latitude, Longitude: s.Location.Longitude},
		Tags:      tags,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// SensorListResponse wraps a list of sensors.
type SensorListResponse struct {
	Sensors []SensorResponse `json:"sensors"`
}

// NearestSensorResponse is the result of a nearest-sensor query.
type NearestSensorResponse struct {
	Sensor         SensorResponse `json:"sensor"`
	DistanceMeters float64        `json:"distance_meters" example:"1234.5"`
}

// HealthResponse is returned by the health endpoints.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// ErrorResponse is the envelope for every error.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries a machine-readable code, a human message and optional field errors.
type ErrorBody struct {
	Code    string       `json:"code" example:"validation_failed"`
	Message string       `json:"message" example:"request is invalid"`
	Fields  []FieldError `json:"fields,omitempty"`
}

// FieldError points at one invalid input field.
type FieldError struct {
	Field   string `json:"field" example:"location.latitude"`
	Message string `json:"message" example:"must be between -90 and 90"`
}
