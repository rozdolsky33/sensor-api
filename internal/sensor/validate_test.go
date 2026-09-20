package sensor_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/volodymyrrozdolsky/sensor-api/internal/sensor"
)

func validSensor() sensor.Sensor {
	return sensor.Sensor{
		Name:     "roof-temp-01",
		Location: sensor.Location{Latitude: 51.5074, Longitude: -0.1278},
		Tags:     []string{"temperature", "roof"},
	}
}

// fieldErrors returns the failing field names from err, or fails the test if err is not a ValidationError.
func fieldErrors(t *testing.T, err error) []string {
	t.Helper()
	var ve *sensor.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *sensor.ValidationError, got %T (%v)", err, err)
	}
	fields := make([]string, 0, len(ve.Fields))
	for _, f := range ve.Fields {
		fields = append(fields, f.Field)
	}
	return fields
}

func TestValidate_Valid(t *testing.T) {
	got, err := sensor.Validate(validSensor())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got.Tags, []string{"temperature", "roof"}) {
		t.Fatalf("tags = %v", got.Tags)
	}
}

func TestValidate_NormalisesTags(t *testing.T) {
	s := validSensor()
	s.Tags = []string{" a ", "b", "a", "B", "b "}
	got, err := sensor.Validate(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"a", "b", "B"}; !slices.Equal(got.Tags, want) {
		t.Fatalf("tags = %v, want %v", got.Tags, want)
	}
}

func TestValidate_NilTagsBecomeEmptySlice(t *testing.T) {
	s := validSensor()
	s.Tags = nil
	got, err := sensor.Validate(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Tags == nil || len(got.Tags) != 0 {
		t.Fatalf("tags = %#v, want empty non-nil slice", got.Tags)
	}
}

func TestValidate_Name(t *testing.T) {
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"simple", "a", true},
		{"with separators", "roof.temp_01-x", true},
		{"max length", strings.Repeat("a", 64), true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 65), false},
		{"leading dash", "-abc", false},
		{"space", "a b", false},
		{"slash", "a/b", false},
		{"unicode", "sensör", false},
		{"reserved nearest", "nearest", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSensor()
			s.Name = tt.value
			_, err := sensor.Validate(s)
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok {
				if got := fieldErrors(t, err); !slices.Equal(got, []string{"name"}) {
					t.Fatalf("fields = %v, want [name]", got)
				}
			}
		})
	}
}

func TestValidate_Location(t *testing.T) {
	tests := []struct {
		name     string
		lat, lon float64
		want     []string
	}{
		{"boundaries", 90, 180, nil},
		{"negative boundaries", -90, -180, nil},
		{"lat too big", 90.0001, 0, []string{"location.latitude"}},
		{"lat too small", -91, 0, []string{"location.latitude"}},
		{"lon too big", 0, 180.5, []string{"location.longitude"}},
		{"lon too small", 0, -181, []string{"location.longitude"}},
		{"nan lat", math.NaN(), 0, []string{"location.latitude"}},
		{"inf lon", 0, math.Inf(1), []string{"location.longitude"}},
		{"both bad", 100, -200, []string{"location.latitude", "location.longitude"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSensor()
			s.Location = sensor.Location{Latitude: tt.lat, Longitude: tt.lon}
			_, err := sensor.Validate(s)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if got := fieldErrors(t, err); !slices.Equal(got, tt.want) {
				t.Fatalf("fields = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate_Tags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{"empty tag", []string{"ok", ""}, []string{"tags[1]"}},
		{"whitespace tag", []string{"   "}, []string{"tags[0]"}},
		{"too long", []string{strings.Repeat("x", 65)}, []string{"tags[0]"}},
		{"too many", make([]string, 33), []string{"tags"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSensor()
			s.Tags = tt.tags
			_, err := sensor.Validate(s)
			if got := fieldErrors(t, err); !slices.Equal(got, tt.want) {
				t.Fatalf("fields = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate_Tags_MultibyteLengthCountedInRunes(t *testing.T) {
	s := validSensor()
	tag := strings.Repeat("ä", 40) // 40 runes, 80 bytes: accepted, since MaxTagLength counts runes
	s.Tags = []string{tag}
	got, err := sensor.Validate(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got.Tags, []string{tag}) {
		t.Fatalf("tags = %v", got.Tags)
	}
}

func TestValidate_Tags_MaxTagsBoundary(t *testing.T) {
	tags := make([]string, sensor.MaxTags)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d", i)
	}
	s := validSensor()
	s.Tags = tags
	got, err := sensor.Validate(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Tags) != sensor.MaxTags {
		t.Fatalf("tags = %v, want %d entries", got.Tags, sensor.MaxTags)
	}
}

func TestValidate_CollectsAllErrors(t *testing.T) {
	s := sensor.Sensor{Name: "", Location: sensor.Location{Latitude: 999, Longitude: 999}, Tags: []string{""}}
	_, err := sensor.Validate(s)
	got := fieldErrors(t, err)
	want := []string{"name", "location.latitude", "location.longitude", "tags[0]"}
	if !slices.Equal(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	if !strings.Contains(err.Error(), "name: is required") {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestValidateLocation_UsesGivenFieldNames(t *testing.T) {
	err := sensor.ValidateLocation(sensor.Location{Latitude: 91, Longitude: 0}, "lat", "lon")
	if got := fieldErrors(t, err); !slices.Equal(got, []string{"lat"}) {
		t.Fatalf("fields = %v, want [lat]", got)
	}
	if err := sensor.ValidateLocation(sensor.Location{}, "lat", "lon"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSensorClone(t *testing.T) {
	orig := validSensor()
	c := orig.Clone()
	c.Tags[0] = "changed"
	if orig.Tags[0] != "temperature" {
		t.Fatal("Clone shares the tags slice")
	}
	var empty sensor.Sensor
	if empty.Clone().Tags == nil {
		t.Fatal("Clone of nil tags should be a non-nil empty slice")
	}
}
