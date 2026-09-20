// Package geo provides great-circle distance calculations on the WGS84 sphere.
package geo

import "math"

// EarthRadiusMeters is the IUGG mean Earth radius.
const EarthRadiusMeters = 6371008.8

// Point is a coordinate in decimal degrees.
type Point struct {
	Lat float64
	Lon float64
}

// Distance returns the great-circle distance in metres between a and b,
// computed with the haversine formula.
func Distance(a, b Point) float64 {
	lat1 := radians(a.Lat)
	lat2 := radians(b.Lat)
	dLat := lat2 - lat1
	dLon := radians(b.Lon - a.Lon)

	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLon*sinLon

	// Clamp guards against floating-point drift pushing sqrt(h) above 1.
	return 2 * EarthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(h)))
}

func radians(deg float64) float64 { return deg * math.Pi / 180 }
