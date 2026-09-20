package sensor

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validation limits.
const (
	MaxNameLength = 64
	MaxTags       = 32
	MaxTagLength  = 64
)

var (
	nameRE        = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	reservedNames = map[string]struct{}{"nearest": {}}
)

// Validate checks every field of s and returns a normalised copy: tags are
// trimmed, de-duplicated (order preserved) and never nil. On failure it returns
// a *ValidationError listing every problem.
func Validate(s Sensor) (Sensor, error) {
	var ve ValidationError
	validateName(&ve, "name", s.Name)
	validateLocation(&ve, "location.latitude", "location.longitude", s.Location)
	s.Tags = normalizeTags(&ve, "tags", s.Tags)
	if err := ve.Err(); err != nil {
		return Sensor{}, err
	}
	return s, nil
}

// ValidateLocation checks a location, reporting problems under the given field names.
func ValidateLocation(loc Location, latField, lonField string) error {
	var ve ValidationError
	validateLocation(&ve, latField, lonField, loc)
	return ve.Err()
}

func validateName(ve *ValidationError, field, name string) {
	switch {
	case name == "":
		ve.Add(field, "is required")
	case utf8.RuneCountInString(name) > MaxNameLength:
		ve.Add(field, fmt.Sprintf("must be at most %d characters", MaxNameLength))
	case !nameRE.MatchString(name):
		ve.Add(field, "must start with a letter or digit and contain only letters, digits, '.', '_' or '-'")
	default:
		if _, reserved := reservedNames[name]; reserved {
			ve.Add(field, fmt.Sprintf("%q is reserved", name))
		}
	}
}

func validateLocation(ve *ValidationError, latField, lonField string, loc Location) {
	validateCoordinate(ve, latField, loc.Latitude, 90)
	validateCoordinate(ve, lonField, loc.Longitude, 180)
}

func validateCoordinate(ve *ValidationError, field string, v, limit float64) {
	switch {
	case math.IsNaN(v) || math.IsInf(v, 0):
		ve.Add(field, "must be a finite number")
	case v < -limit || v > limit:
		ve.Add(field, fmt.Sprintf("must be between %g and %g", -limit, limit))
	}
}

// normalizeTags trims and de-duplicates tags, recording any invalid entries.
func normalizeTags(ve *ValidationError, field string, tags []string) []string {
	if len(tags) > MaxTags {
		ve.Add(field, fmt.Sprintf("must contain at most %d tags", MaxTags))
		return []string{}
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for i, raw := range tags {
		tag := strings.TrimSpace(raw)
		entry := fmt.Sprintf("%s[%d]", field, i)
		switch {
		case tag == "":
			ve.Add(entry, "must not be empty")
			continue
		case utf8.RuneCountInString(tag) > MaxTagLength:
			ve.Add(entry, fmt.Sprintf("must be at most %d characters", MaxTagLength))
			continue
		}
		if _, dup := seen[tag]; dup {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}
