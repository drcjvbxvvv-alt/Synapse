package github

import (
	"fmt"
	"net/http"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// mapHTTPStatus converts a GitHub REST response code + short body preview
// into one of the engine sentinel errors.
//
// GitHub-specific gotchas:
//   - 403 often means "rate-limited" rather than "forbidden". The body
//     preview distinguishes them; the adapter treats both as
//     ErrUnauthorized for simplicity (rate-limit retry belongs at a higher
//     layer).
//   - 422 is used for validation failures on /dispatches (unknown input,
//     ref does not exist).
func mapHTTPStatus(status int, bodyPreview string) error {
	switch {
	case status == http.StatusUnauthorized:
		return fmt.Errorf("github returned 401: %s: %w", bodyPreview, engine.ErrUnauthorized)
	case status == http.StatusForbidden:
		return fmt.Errorf("github returned 403: %s: %w", bodyPreview, engine.ErrUnauthorized)
	case status == http.StatusNotFound:
		return fmt.Errorf("github returned 404: %s: %w", bodyPreview, engine.ErrNotFound)
	case status == http.StatusUnprocessableEntity, status == http.StatusBadRequest:
		return fmt.Errorf("github returned %d: %s: %w", status, bodyPreview, engine.ErrInvalidInput)
	case status >= 500:
		return fmt.Errorf("github returned %d: %s: %w", status, bodyPreview, engine.ErrUnavailable)
	default:
		return fmt.Errorf("github returned %d: %s: %w", status, bodyPreview, engine.ErrUnavailable)
	}
}
