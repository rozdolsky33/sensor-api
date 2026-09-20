package sensor

import (
	"context"
	"time"
)

// ListFilter narrows a List call. Zero value means "everything".
type ListFilter struct {
	// Tag, when non-empty, restricts results to sensors carrying this exact tag.
	Tag string
}

// Repository is the persistence contract. Implementations must be safe for
// concurrent use and must never return slices that alias internal state.
type Repository interface {
	// Create stores a new sensor. Returns ErrAlreadyExists if the name is taken.
	Create(ctx context.Context, s Sensor) error
	// Get returns the sensor with the given name or ErrNotFound.
	Get(ctx context.Context, name string) (Sensor, error)
	// List returns matching sensors sorted by name ascending.
	List(ctx context.Context, f ListFilter) ([]Sensor, error)
	// Update replaces the stored sensor with s if the stored UpdatedAt equals
	// expectedUpdatedAt. Returns ErrNotFound or ErrConflict.
	Update(ctx context.Context, s Sensor, expectedUpdatedAt time.Time) error
	// Delete removes the sensor or returns ErrNotFound.
	Delete(ctx context.Context, name string) error
	// Nearest returns the sensor closest to loc and the distance in metres.
	// Ties are broken by the lexicographically smaller name. Returns
	// ErrNoSensors when the repository is empty.
	Nearest(ctx context.Context, loc Location) (Sensor, float64, error)
}
