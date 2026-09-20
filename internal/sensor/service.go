package sensor

import (
	"context"
	"time"
)

// Clock returns the current time; injected for deterministic tests.
type Clock func() time.Time

// Service implements the sensor use cases on top of a Repository.
type Service struct {
	repo Repository
	now  Clock
}

// NewService wires a Service. A nil clock defaults to time.Now in UTC.
func NewService(repo Repository, now Clock) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: repo, now: now}
}

// CreateInput is the data needed to register a sensor.
type CreateInput struct {
	Name     string
	Location Location
	Tags     []string
}

// ReplaceInput fully replaces a sensor's mutable metadata.
type ReplaceInput struct {
	Location Location
	Tags     []string
}

// PatchInput updates only the non-nil fields.
type PatchInput struct {
	Location *Location
	Tags     *[]string
}

// Create validates and stores a new sensor.
func (s *Service) Create(ctx context.Context, in CreateInput) (Sensor, error) {
	sn, err := Validate(Sensor{Name: in.Name, Location: in.Location, Tags: in.Tags})
	if err != nil {
		return Sensor{}, err
	}
	ts := s.now()
	sn.CreatedAt, sn.UpdatedAt = ts, ts
	if err := s.repo.Create(ctx, sn); err != nil {
		return Sensor{}, err
	}
	return sn, nil
}

// Get returns a sensor by name.
func (s *Service) Get(ctx context.Context, name string) (Sensor, error) {
	return s.repo.Get(ctx, name)
}

// List returns sensors matching f, sorted by name.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Sensor, error) {
	return s.repo.List(ctx, f)
}

// Delete removes a sensor by name.
func (s *Service) Delete(ctx context.Context, name string) error {
	return s.repo.Delete(ctx, name)
}

// Replace overwrites location and tags, preserving name and CreatedAt.
func (s *Service) Replace(ctx context.Context, name string, in ReplaceInput) (Sensor, error) {
	cur, err := s.repo.Get(ctx, name)
	if err != nil {
		return Sensor{}, err
	}
	next, err := Validate(Sensor{Name: cur.Name, Location: in.Location, Tags: in.Tags, CreatedAt: cur.CreatedAt})
	if err != nil {
		return Sensor{}, err
	}
	return s.update(ctx, next, cur.UpdatedAt)
}

// Patch applies the provided fields on top of the stored sensor.
func (s *Service) Patch(ctx context.Context, name string, in PatchInput) (Sensor, error) {
	if in.Location == nil && in.Tags == nil {
		ve := &ValidationError{}
		ve.Add("body", "at least one of location or tags must be provided")
		return Sensor{}, ve
	}
	cur, err := s.repo.Get(ctx, name)
	if err != nil {
		return Sensor{}, err
	}
	merged := cur
	if in.Location != nil {
		merged.Location = *in.Location
	}
	if in.Tags != nil {
		merged.Tags = *in.Tags
	}
	next, err := Validate(merged)
	if err != nil {
		return Sensor{}, err
	}
	return s.update(ctx, next, cur.UpdatedAt)
}

// Nearest returns the sensor closest to loc and the distance in metres.
func (s *Service) Nearest(ctx context.Context, loc Location) (Sensor, float64, error) {
	if err := ValidateLocation(loc, "lat", "lon"); err != nil {
		return Sensor{}, 0, err
	}
	return s.repo.Nearest(ctx, loc)
}

func (s *Service) update(ctx context.Context, next Sensor, expectedUpdatedAt time.Time) (Sensor, error) {
	next.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, next, expectedUpdatedAt); err != nil {
		return Sensor{}, err
	}
	return next, nil
}
