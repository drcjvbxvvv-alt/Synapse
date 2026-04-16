package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Cancel cancels a workflow run via POST /actions/runs/{run_id}/cancel.
//
// Already-completed runs return 409 Conflict from GitHub; the adapter
// surfaces that as engine.ErrAlreadyTerminal so callers get uniform
// semantics across adapters.
//
// Placeholder RunIDs (from Trigger timeout) are rejected — there is
// nothing to cancel until the dispatch run has materialised.
func (a *Adapter) Cancel(ctx context.Context, runID string) error {
	id, _, _, err := parseRunID(runID)
	if err != nil {
		return fmt.Errorf("github.Cancel: %w", err)
	}
	if id == 0 {
		return fmt.Errorf("github.Cancel: run not yet resolved (placeholder %q): %w", runID, engine.ErrInvalidInput)
	}
	owner, repo, err := a.extra.requireOwnerRepo()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/cancel", owner, repo, id)
	req, err := a.c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("github.Cancel: %w", err)
	}
	if err := a.c.doJSON(req, nil); err != nil {
		// 409 Conflict → run already completed.
		if isHTTPConflict(err) {
			return fmt.Errorf("github.Cancel %d: %w", id, engine.ErrAlreadyTerminal)
		}
		return fmt.Errorf("github.Cancel %d: %w", id, err)
	}
	return nil
}

// isHTTPConflict checks whether the error was the 409 branch of
// mapHTTPStatus. Since 409 isn't specially mapped (falls into the default
// ErrUnavailable branch), we detect it via error message substring. This
// is the only adapter where 409 has distinct semantics, so a local helper
// is preferred over widening the engine's sentinel surface.
func isHTTPConflict(err error) bool {
	if err == nil {
		return false
	}
	// mapHTTPStatus's message is "github returned 409: ..."; embed-search
	// on that string keeps the check robust to whitespace/punctuation.
	_ = errors.Unwrap
	return containsStatusMarker(err.Error(), "409")
}

// containsStatusMarker is a small helper that avoids importing "strings"
// for a single Contains call (kept local to cancel.go to match CLAUDE §9's
// preference for minimal utility surfaces).
func containsStatusMarker(haystack, marker string) bool {
	// Look for "returned <marker>" so a 409 in a response body doesn't
	// trigger a false positive when the actual status was e.g. 500.
	needle := "returned " + marker
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

// indexOf is a trivial strings.Index replacement kept local for the same
// reason as containsStatusMarker.
func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
