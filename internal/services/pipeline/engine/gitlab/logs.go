package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// StreamLogs returns the log (trace) stream for a single GitLab job.
//
// Semantics:
//   - GitLab's unit of log is a **job**, not a pipeline. Therefore stepID
//     MUST be the numeric job id. Callers discover the job id via
//     GetRun().Steps (M18b follow-up: extend StepStatus with the external
//     step id so the UI can pass it through cleanly).
//   - When stepID is empty we return engine.ErrInvalidInput — callers must
//     pick which job they want to stream.
//   - Endpoint: GET /api/v4/projects/:id/jobs/:job_id/trace (plain text).
//   - GitLab does not support incremental streaming natively; this returns
//     the full trace snapshot at the moment of the call. M18b follow-up can
//     add `Range: bytes=N-` for incremental pulls — the caller just needs
//     to remember how many bytes it consumed.
//
// runID is accepted for symmetry with other Adapter methods but currently
// unused; the pipeline id is only needed when we later correlate jobs back
// to the pipeline-level URL.
func (a *Adapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) {
	if stepID == "" {
		return nil, fmt.Errorf("gitlab.StreamLogs: stepID (job id) is required: %w", engine.ErrInvalidInput)
	}
	projectID, err := a.extra.requireProjectID()
	if err != nil {
		return nil, err
	}
	jobID, err := strconv.ParseInt(stepID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gitlab.StreamLogs: stepID %q is not a GitLab job id: %w", stepID, engine.ErrInvalidInput)
	}

	path := fmt.Sprintf("/projects/%d/jobs/%d/trace", projectID, jobID)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab.StreamLogs: %w", err)
	}
	// GitLab returns text/plain, so we override Accept to match — otherwise
	// some older GitLab versions return HTML error pages on auth failures.
	req.Header.Set("Accept", "text/plain")

	rc, err := a.c.doRaw(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab.StreamLogs %s/%s: %w", runID, stepID, err)
	}
	return rc, nil
}
