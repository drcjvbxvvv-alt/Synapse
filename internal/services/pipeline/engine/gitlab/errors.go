package gitlab

import (
	"fmt"
	"net/http"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// mapHTTPStatus converts a GitLab HTTP response code + body preview into one
// of the engine package's sentinel errors so that the HTTP handler layer can
// map them uniformly to the client.
//
// bodyPreview MUST be short (already truncated by the caller) — we include it
// in the error message for diagnostic purposes and MUST NOT contain secrets.
//
// For 2xx responses, callers never call this function.
func mapHTTPStatus(status int, bodyPreview string) error {
	switch {
	case status == http.StatusUnauthorized:
		return fmt.Errorf("gitlab returned 401: %s: %w", bodyPreview, engine.ErrUnauthorized)
	case status == http.StatusForbidden:
		// GitLab returns 403 for both "forbidden" and "token lacks scope";
		// treat them both as unauthorized from the adapter's perspective.
		return fmt.Errorf("gitlab returned 403: %s: %w", bodyPreview, engine.ErrUnauthorized)
	case status == http.StatusNotFound:
		return fmt.Errorf("gitlab returned 404: %s: %w", bodyPreview, engine.ErrNotFound)
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return fmt.Errorf("gitlab returned %d: %s: %w", status, bodyPreview, engine.ErrInvalidInput)
	case status >= 500:
		return fmt.Errorf("gitlab returned %d: %s: %w", status, bodyPreview, engine.ErrUnavailable)
	default:
		// Unusual 4xx (e.g. 409, 429) — surface as unavailable so callers
		// retry / show a friendly error; the raw code is preserved in the
		// message.
		return fmt.Errorf("gitlab returned %d: %s: %w", status, bodyPreview, engine.ErrUnavailable)
	}
}
