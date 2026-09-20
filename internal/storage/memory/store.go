// Package memory is an in-process sensor.Repository backed by a map.
package memory

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/volodymyrrozdolsky/sensor-api/internal/geo"
	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

// Store keeps sensors in memory. It is safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	sensors map[string]sensor.Sensor
}

var _ sensor.Repository = (*Store)(nil)

// New returns an empty Store.
func New() *Store {
	return &Store{sensors: make(map[string]sensor.Sensor)}
}

// Create implements sensor.Repository.
func (s *Store) Create(ctx context.Context, sn sensor.Sensor) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.sensors[sn.Name]; exists {
		return sensor.ErrAlreadyExists
	}
	s.sensors[sn.Name] = sn.Clone()
	return nil
}

// Get implements sensor.Repository.
func (s *Store) Get(ctx context.Context, name string) (sensor.Sensor, error) {
	if err := ctx.Err(); err != nil {
		return sensor.Sensor{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sn, ok := s.sensors[name]
	if !ok {
		return sensor.Sensor{}, sensor.ErrNotFound
	}
	return sn.Clone(), nil
}

// List implements sensor.Repository.
func (s *Store) List(ctx context.Context, f sensor.ListFilter) ([]sensor.Sensor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]sensor.Sensor, 0, len(s.sensors))
	for _, sn := range s.sensors {
		if f.Tag != "" && !slices.Contains(sn.Tags, f.Tag) {
			continue
		}
		out = append(out, sn.Clone())
	}
	slices.SortFunc(out, func(a, b sensor.Sensor) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Update implements sensor.Repository.
func (s *Store) Update(ctx context.Context, sn sensor.Sensor, expectedUpdatedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.sensors[sn.Name]
	if !ok {
		return sensor.ErrNotFound
	}
	if !cur.UpdatedAt.Equal(expectedUpdatedAt) {
		return sensor.ErrConflict
	}
	s.sensors[sn.Name] = sn.Clone()
	return nil
}

// Delete implements sensor.Repository.
func (s *Store) Delete(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sensors[name]; !ok {
		return sensor.ErrNotFound
	}
	delete(s.sensors, name)
	return nil
}

// Nearest implements sensor.Repository with an O(n) haversine scan.
func (s *Store) Nearest(ctx context.Context, loc sensor.Location) (sensor.Sensor, float64, error) {
	if err := ctx.Err(); err != nil {
		return sensor.Sensor{}, 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.sensors) == 0 {
		return sensor.Sensor{}, 0, sensor.ErrNoSensors
	}
	from := geo.Point{Lat: loc.Latitude, Lon: loc.Longitude}
	var (
		best     sensor.Sensor
		bestDist = -1.0
	)
	for _, sn := range s.sensors {
		d := geo.Distance(from, geo.Point{Lat: sn.Location.Latitude, Lon: sn.Location.Longitude})
		if bestDist < 0 || d < bestDist || (d == bestDist && sn.Name < best.Name) {
			best, bestDist = sn, d
		}
	}
	return best.Clone(), bestDist, nil
}
