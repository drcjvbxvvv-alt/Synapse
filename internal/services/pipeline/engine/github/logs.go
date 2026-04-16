package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// StreamLogs returns the console log for a specific GitHub Actions job.
//
// stepID semantics: the GitHub job id (numeric). GitHub exposes logs at
// job granularity through /actions/jobs/:id/logs, which returns plain
// text. A run-level /runs/:id/logs endpoint also exists but returns a zip
// archive, which is less convenient for live streaming; M18e therefore
// requires callers to pass a stepID.
//
// runID is accepted for interface symmetry but only used in error
// messages — GitHub's job log endpoint doesn't need the run id.
func (a *Adapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) {
	if stepID == "" {
		return nil, fmt.Errorf("github.StreamLogs: stepID (job id) is required: %w", engine.ErrInvalidInput)
	}
	jobID, err := strconv.ParseInt(stepID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("github.StreamLogs: stepID %q is not a GitHub job id: %w", stepID, engine.ErrInvalidInput)
	}
	owner, repo, err := a.extra.requireOwnerRepo()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, jobID)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("github.StreamLogs: %w", err)
	}
	// Override Accept so older proxies don't return HTML error pages.
	req.Header.Set("Accept", "text/plain")

	// GitHub responds with 302 Found → signed S3-style URL. Go's default
	// http.Client follows redirects by default; we rely on that so callers
	// receive the underlying log stream.
	rc, err := a.c.doRaw(req)
	if err != nil {
		return nil, fmt.Errorf("github.StreamLogs %s/%s: %w", runID, stepID, err)
	}
	return rc, nil
}
