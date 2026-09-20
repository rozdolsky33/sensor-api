package memory_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
	"github.com/volodymyrrozdolsky/sensor-api/internal/storage/memory"
	"github.com/volodymyrrozdolsky/sensor-api/internal/storage/storagetest"
)

func TestStore_Contract(t *testing.T) {
	storagetest.Run(t, func(t *testing.T) sensor.Repository { return memory.New() })
}

func TestStore_ConcurrentAccess(t *testing.T) {
	const n = 64
	store := memory.New()
	ctx := context.Background()
	t0 := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := sensor.Sensor{
				Name:      fmt.Sprintf("s-%02d", i),
				Location:  sensor.Location{Latitude: float64(i), Longitude: float64(i)},
				Tags:      []string{"t"},
				CreatedAt: t0,
				UpdatedAt: t0,
			}
			if err := store.Create(ctx, s); err != nil {
				t.Errorf("Create: %v", err)
				return
			}
			s.UpdatedAt = t0.Add(time.Second)
			if err := store.Update(ctx, s, t0); err != nil {
				t.Errorf("Update: %v", err)
			}
			if _, err := store.Get(ctx, s.Name); err != nil {
				t.Errorf("Get: %v", err)
			}
			if _, err := store.List(ctx, sensor.ListFilter{Tag: "t"}); err != nil {
				t.Errorf("List: %v", err)
			}
			if _, _, err := store.Nearest(ctx, s.Location); err != nil {
				t.Errorf("Nearest: %v", err)
			}
		}()
	}
	wg.Wait()

	all, err := store.List(ctx, sensor.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != n {
		t.Fatalf("len(List) = %d, want %d", len(all), n)
	}
}

func TestStore_CancelledContext(t *testing.T) {
	store := memory.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Create(ctx, sensor.Sensor{Name: "a"}); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
