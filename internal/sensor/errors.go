package sensor

import (
	"errors"
	"strings"
)

// Sentinel errors returned by Repository and Service.
var (
	ErrNotFound      = errors.New("sensor not found")
	ErrAlreadyExists = errors.New("sensor already exists")
	ErrNoSensors     = errors.New("no sensors stored")
	ErrConflict      = errors.New("sensor was modified concurrently")
)

// FieldError describes a single invalid input field.
type FieldError struct {
	Field   string
	Message string
}

// ValidationError aggregates every invalid field found in one input.
type ValidationError struct {
	Fields []FieldError
}

// Error implements error.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Add records a field error.
func (e *ValidationError) Add(field, message string) {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: message})
}

// Err returns e as an error if any field failed, otherwise nil.
func (e *ValidationError) Err() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}
