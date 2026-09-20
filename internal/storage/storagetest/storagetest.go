// Package storagetest provides a behavioural test suite that every
// sensor.Repository implementation must pass.
package storagetest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

var (
	t0 = time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Minute)
)

func mk(name string, lat, lon float64, tags ...string) sensor.Sensor {
	if tags == nil {
		tags = []string{}
	}
	return sensor.Sensor{
		Name:      name,
		Location:  sensor.Location{Latitude: lat, Longitude: lon},
		Tags:      tags,
		CreatedAt: t0,
		UpdatedAt: t0,
	}
}

func mustCreate(t *testing.T, repo sensor.Repository, sensors ...sensor.Sensor) {
	t.Helper()
	for _, s := range sensors {
		if err := repo.Create(context.Background(), s); err != nil {
			t.Fatalf("Create(%s): %v", s.Name, err)
		}
	}
}

// Run executes the contract suite against repositories produced by newRepo.
// newRepo must return a fresh, empty repository on each call.
func Run(t *testing.T, newRepo func(t *testing.T) sensor.Repository) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, repo sensor.Repository)
	}{
		{"CreateThenGet", testCreateThenGet},
		{"CreateDuplicate", testCreateDuplicate},
		{"ReturnsCopies", testReturnsCopies},
		{"ListSortedAndFiltered", testListSortedAndFiltered},
		{"Update", testUpdate},
		{"Delete", testDelete},
		{"Nearest", testNearest},
		{"NearestTieBreak", testNearestTieBreak},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.fn(t, newRepo(t)) })
	}
}

func testCreateThenGet(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	want := mk("london", 51.5074, -0.1278, "city")
	mustCreate(t, repo, want)

	got, err := repo.Get(ctx, "london")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get = %+v, want %+v", got, want)
	}

	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, sensor.ErrNotFound) {
		t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
	}
}

func testCreateDuplicate(t *testing.T, repo sensor.Repository) {
	mustCreate(t, repo, mk("a", 0, 0))
	if err := repo.Create(context.Background(), mk("a", 1, 1)); !errors.Is(err, sensor.ErrAlreadyExists) {
		t.Fatalf("second Create err = %v, want ErrAlreadyExists", err)
	}
}

func testReturnsCopies(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	in := mk("a", 0, 0, "original")
	mustCreate(t, repo, in)
	in.Tags[0] = "mutated-input"

	got, err := repo.Get(ctx, "a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Tags[0] != "original" {
		t.Fatal("repository stored the caller's slice instead of a copy")
	}
	got.Tags[0] = "mutated-output"

	again, _ := repo.Get(ctx, "a")
	if again.Tags[0] != "original" {
		t.Fatal("repository returned its internal slice instead of a copy")
	}

	list, _ := repo.List(ctx, sensor.ListFilter{})
	list[0].Tags[0] = "mutated-list"
	again, _ = repo.Get(ctx, "a")
	if again.Tags[0] != "original" {
		t.Fatal("List returned internal slices")
	}
}

func testListSortedAndFiltered(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	empty, err := repo.List(ctx, sensor.ListFilter{})
	if err != nil || len(empty) != 0 {
		t.Fatalf("List(empty) = %v, %v", empty, err)
	}

	mustCreate(t, repo,
		mk("charlie", 0, 0, "temp"),
		mk("alpha", 0, 0, "temp", "roof"),
		mk("bravo", 0, 0, "humidity"),
	)

	all, err := repo.List(ctx, sensor.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := names(all); !reflect.DeepEqual(got, []string{"alpha", "bravo", "charlie"}) {
		t.Fatalf("List order = %v", got)
	}

	temp, err := repo.List(ctx, sensor.ListFilter{Tag: "temp"})
	if err != nil {
		t.Fatalf("List(tag): %v", err)
	}
	if got := names(temp); !reflect.DeepEqual(got, []string{"alpha", "charlie"}) {
		t.Fatalf("List(tag=temp) = %v", got)
	}

	none, _ := repo.List(ctx, sensor.ListFilter{Tag: "nope"})
	if len(none) != 0 {
		t.Fatalf("List(tag=nope) = %v", names(none))
	}
}

func testUpdate(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	mustCreate(t, repo, mk("a", 0, 0, "old"))

	next := mk("a", 10, 20, "new")
	next.UpdatedAt = t1
	if err := repo.Update(ctx, next, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.Get(ctx, "a")
	if !reflect.DeepEqual(got, next) {
		t.Fatalf("after Update Get = %+v, want %+v", got, next)
	}

	if err := repo.Update(ctx, next, t0); !errors.Is(err, sensor.ErrConflict) {
		t.Fatalf("stale Update err = %v, want ErrConflict", err)
	}
	if err := repo.Update(ctx, mk("missing", 0, 0), t0); !errors.Is(err, sensor.ErrNotFound) {
		t.Fatalf("Update(missing) err = %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	mustCreate(t, repo, mk("a", 0, 0))
	if err := repo.Delete(ctx, "a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, "a"); !errors.Is(err, sensor.ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, "a"); !errors.Is(err, sensor.ErrNotFound) {
		t.Fatalf("second Delete err = %v, want ErrNotFound", err)
	}
}

func testNearest(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	if _, _, err := repo.Nearest(ctx, sensor.Location{}); !errors.Is(err, sensor.ErrNoSensors) {
		t.Fatalf("Nearest(empty) err = %v, want ErrNoSensors", err)
	}

	mustCreate(t, repo,
		mk("paris", 48.8566, 2.3522),
		mk("london", 51.5074, -0.1278),
		mk("berlin", 52.52, 13.405),
	)
	got, dist, err := repo.Nearest(ctx, sensor.Location{Latitude: 51.5, Longitude: -0.1})
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if got.Name != "london" {
		t.Fatalf("Nearest = %s, want london", got.Name)
	}
	if dist <= 0 || dist > 5_000 {
		t.Fatalf("distance = %.0f m, want between 0 and 5000", dist)
	}
}

func testNearestTieBreak(t *testing.T, repo sensor.Repository) {
	ctx := context.Background()
	mustCreate(t, repo, mk("zulu", 10, 10), mk("alpha", 10, 10), mk("mike", 10, 10))
	got, dist, err := repo.Nearest(ctx, sensor.Location{Latitude: 10, Longitude: 10})
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if got.Name != "alpha" || dist != 0 {
		t.Fatalf("Nearest = %s (%.0f m), want alpha (0 m)", got.Name, dist)
	}
}

func names(ss []sensor.Sensor) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Name)
	}
	return out
}
