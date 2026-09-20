package geo_test

import (
	"math"
	"testing"

	"github.com/volodymyrrozdolsky/sensor-api/internal/geo"
)

func TestDistance(t *testing.T) {
	london := geo.Point{Lat: 51.5074, Lon: -0.1278}
	paris := geo.Point{Lat: 48.8566, Lon: 2.3522}

	tests := []struct {
		name      string
		a, b      geo.Point
		want      float64
		tolerance float64 // absolute, metres
	}{
		{"same point", london, london, 0, 0},
		{"london to paris", london, paris, 343_556, 2_000},
		{"antipodes on equator", geo.Point{0, 0}, geo.Point{0, 180}, math.Pi * geo.EarthRadiusMeters, 1},
		{"pole to pole", geo.Point{90, 0}, geo.Point{-90, 0}, math.Pi * geo.EarthRadiusMeters, 1},
		{"one degree of longitude at equator", geo.Point{0, 0}, geo.Point{0, 1}, 111_195, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := geo.Distance(tt.a, tt.b)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Fatalf("Distance() = %.1f, want %.1f ± %.1f", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestDistanceIsSymmetric(t *testing.T) {
	a := geo.Point{Lat: 35.6762, Lon: 139.6503}
	b := geo.Point{Lat: -33.8688, Lon: 151.2093}
	if d1, d2 := geo.Distance(a, b), geo.Distance(b, a); d1 != d2 {
		t.Fatalf("Distance not symmetric: %f vs %f", d1, d2)
	}
}
