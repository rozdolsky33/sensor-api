package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// maxBodyBytes caps request bodies at 1 MiB.
const maxBodyBytes = 1 << 20

// requestError is a client error detected before the domain layer.
type requestError struct {
	status  int
	code    string
	message string
}

func (e *requestError) Error() string { return e.message }

// decodeJSON reads a single JSON object from r into dst, enforcing the
// content type, the size limit and strict field matching.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return &requestError{http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return &requestError{http.StatusRequestEntityTooLarge, "payload_too_large",
				fmt.Sprintf("request body must not exceed %d bytes", maxBodyBytes)}
		case errors.Is(err, io.EOF):
			return &requestError{http.StatusBadRequest, "invalid_json", "request body must not be empty"}
		default:
			return &requestError{http.StatusBadRequest, "invalid_json", "request body is not valid JSON: " + err.Error()}
		}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &requestError{http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object"}
	}
	return nil
}

func isJSONContentType(header string) bool {
	mediaType, _, err := mime.ParseMediaType(header)
	return err == nil && mediaType == "application/json"
}
