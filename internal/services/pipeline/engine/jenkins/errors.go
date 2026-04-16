package jenkins

import (
	"fmt"
	"net/http"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// mapHTTPStatus converts a Jenkins HTTP status into the engine package's
// sentinel errors. bodyPreview is a short diagnostic string (callers must
// truncate); it MUST NOT contain credentials — Jenkins occasionally echoes
// request bodies in error responses so upstream code should verify that the
// preview doesn't carry secrets.
func mapHTTPStatus(status int, bodyPreview string) error {
	switch {
	case status == http.StatusUnauthorized:
		return fmt.Errorf("jenkins returned 401: %s: %w", bodyPreview, engine.ErrUnauthorized)
	case status == http.StatusForbidden:
		// Jenkins returns 403 both for "token lacks permission" and for a
		// missing/invalid crumb. Treat both as unauthorized; the crumb
		// layer will re-fetch on its own retry path.
		return fmt.Errorf("jenkins returned 403: %s: %w", bodyPreview, engine.ErrUnauthorized)
	case status == http.StatusNotFound:
		return fmt.Errorf("jenkins returned 404: %s: %w", bodyPreview, engine.ErrNotFound)
	case status == http.StatusBadRequest:
		return fmt.Errorf("jenkins returned 400: %s: %w", bodyPreview, engine.ErrInvalidInput)
	case status >= 500:
		return fmt.Errorf("jenkins returned %d: %s: %w", status, bodyPreview, engine.ErrUnavailable)
	default:
		// Unusual 4xx (405, 409, 429, …) — surface as unavailable so callers
		// can retry; raw status preserved in the message.
		return fmt.Errorf("jenkins returned %d: %s: %w", status, bodyPreview, engine.ErrUnavailable)
	}
}

// isCSRFError reports whether err corresponds to a 403 response that may
// have been caused by an expired crumb. Used by the client to invalidate
// its cached crumb and retry once.
func isCSRFError(status int) bool { return status == http.StatusForbidden }
