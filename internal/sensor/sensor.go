// Package sensor holds the sensor domain model, validation rules, the
// repository contract and the application service.
package sensor

import "time"

// Location is a WGS84 coordinate in decimal degrees.
type Location struct {
	Latitude  float64
	Longitude float64
}

// Sensor is the metadata stored for a single sensor. Name is the immutable identifier.
type Sensor struct {
	Name      string
	Location  Location
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Clone returns a deep copy of s. Tags is always a non-nil slice in the copy.
func (s Sensor) Clone() Sensor {
	s.Tags = cloneTags(s.Tags)
	return s
}

func cloneTags(tags []string) []string {
	out := make([]string, len(tags))
	copy(out, tags)
	return out
}
