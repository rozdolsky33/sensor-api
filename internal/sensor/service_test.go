package sensor_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
	"github.com/volodymyrrozdolsky/sensor-api/internal/storage/memory"
)

var base = time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

// newService returns a service whose clock advances one second per call.
func newService(t *testing.T) *sensor.Service {
	t.Helper()
	tick := 0
	clock := func() time.Time {
		tick++
		return base.Add(time.Duration(tick) * time.Second)
	}
	return sensor.NewService(memory.New(), clock)
}

func london() sensor.CreateInput {
	return sensor.CreateInput{
		Name:     "london",
		Location: sensor.Location{Latitude: 51.5074, Longitude: -0.1278},
		Tags:     []string{"city", " uk ", "city"},
	}
}

func TestService_Create(t *testing.T) {
	svc := newService(t)
	got, err := svc.Create(context.Background(), london())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !slices.Equal(got.Tags, []string{"city", "uk"}) {
		t.Fatalf("tags = %v", got.Tags)
	}
	if !got.CreatedAt.Equal(base.Add(time.Second)) || !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Fatalf("timestamps = %v / %v", got.CreatedAt, got.UpdatedAt)
	}
	stored, _ := svc.Get(context.Background(), "london")
	if stored.Name != "london" {
		t.Fatal("sensor not persisted")
	}
}

func TestService_CreateInvalid(t *testing.T) {
	svc := newService(t)
	in := london()
	in.Location.Latitude = 100
	_, err := svc.Create(context.Background(), in)
	var ve *sensor.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_CreateDuplicate(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, london()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, london()); !errors.Is(err, sensor.ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestService_Replace(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, london())

	got, err := svc.Replace(ctx, "london", sensor.ReplaceInput{
		Location: sensor.Location{Latitude: 1, Longitude: 2},
		Tags:     nil,
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if got.Location != (sensor.Location{Latitude: 1, Longitude: 2}) {
		t.Fatalf("location = %+v", got.Location)
	}
	if len(got.Tags) != 0 || got.Tags == nil {
		t.Fatalf("tags = %#v, want empty slice", got.Tags)
	}
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("Replace changed CreatedAt")
	}
	if !got.UpdatedAt.After(created.UpdatedAt) {
		t.Fatal("Replace did not advance UpdatedAt")
	}

	if _, err := svc.Replace(ctx, "missing", sensor.ReplaceInput{}); !errors.Is(err, sensor.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := svc.Replace(ctx, "london", sensor.ReplaceInput{Location: sensor.Location{Latitude: 91}}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestService_Patch(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, london())

	t.Run("location only keeps tags", func(t *testing.T) {
		got, err := svc.Patch(ctx, "london", sensor.PatchInput{Location: &sensor.Location{Latitude: 5, Longitude: 6}})
		if err != nil {
			t.Fatalf("Patch: %v", err)
		}
		if got.Location.Latitude != 5 || !slices.Equal(got.Tags, created.Tags) {
			t.Fatalf("got = %+v", got)
		}
		if !got.CreatedAt.Equal(created.CreatedAt) || !got.UpdatedAt.After(created.UpdatedAt) {
			t.Fatalf("timestamps = %v / %v", got.CreatedAt, got.UpdatedAt)
		}
	})

	t.Run("tags only keeps location", func(t *testing.T) {
		tags := []string{"new"}
		got, err := svc.Patch(ctx, "london", sensor.PatchInput{Tags: &tags})
		if err != nil {
			t.Fatalf("Patch: %v", err)
		}
		if got.Location.Latitude != 5 || !slices.Equal(got.Tags, []string{"new"}) {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("empty patch is a validation error", func(t *testing.T) {
		_, err := svc.Patch(ctx, "london", sensor.PatchInput{})
		var ve *sensor.ValidationError
		if !errors.As(err, &ve) || ve.Fields[0].Field != "body" {
			t.Fatalf("err = %v, want ValidationError on body", err)
		}
	})

	t.Run("invalid patch", func(t *testing.T) {
		bad := []string{""}
		_, err := svc.Patch(ctx, "london", sensor.PatchInput{Tags: &bad})
		var ve *sensor.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v, want ValidationError", err)
		}
	})

	t.Run("missing sensor", func(t *testing.T) {
		tags := []string{"x"}
		if _, err := svc.Patch(ctx, "missing", sensor.PatchInput{Tags: &tags}); !errors.Is(err, sensor.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestService_ListAndDelete(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	_, _ = svc.Create(ctx, london())
	_, _ = svc.Create(ctx, sensor.CreateInput{Name: "berlin", Location: sensor.Location{Latitude: 52.52, Longitude: 13.405}})

	all, err := svc.List(ctx, sensor.ListFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("List = %d sensors, err %v", len(all), err)
	}
	uk, _ := svc.List(ctx, sensor.ListFilter{Tag: "uk"})
	if len(uk) != 1 || uk[0].Name != "london" {
		t.Fatalf("List(uk) = %+v", uk)
	}
	if err := svc.Delete(ctx, "berlin"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(ctx, "berlin"); !errors.Is(err, sensor.ErrNotFound) {
		t.Fatalf("second Delete err = %v", err)
	}
}

func TestService_Nearest(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	if _, _, err := svc.Nearest(ctx, sensor.Location{}); !errors.Is(err, sensor.ErrNoSensors) {
		t.Fatalf("err = %v, want ErrNoSensors", err)
	}

	_, _, err := svc.Nearest(ctx, sensor.Location{Latitude: 91, Longitude: 0})
	var ve *sensor.ValidationError
	if !errors.As(err, &ve) || ve.Fields[0].Field != "lat" {
		t.Fatalf("err = %v, want ValidationError on lat", err)
	}

	_, _ = svc.Create(ctx, london())
	got, dist, err := svc.Nearest(ctx, sensor.Location{Latitude: 51.5, Longitude: -0.1})
	if err != nil || got.Name != "london" || dist <= 0 {
		t.Fatalf("Nearest = %s, %.0f, %v", got.Name, dist, err)
	}
}

func TestNewService_DefaultClock(t *testing.T) {
	svc := sensor.NewService(memory.New(), nil)
	got, err := svc.Create(context.Background(), london())
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedAt.IsZero() || got.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt = %v, want non-zero UTC", got.CreatedAt)
	}
}
