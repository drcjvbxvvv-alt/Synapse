package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Cancel cancels a running GitLab pipeline via
// POST /api/v4/projects/:id/pipelines/:id/cancel.
//
// Behaviour:
//   - Already-terminal pipelines (success / failed / canceled) → returns
//     engine.ErrAlreadyTerminal so callers can treat it informationally.
//   - Unknown pipeline id → engine.ErrNotFound (via mapHTTPStatus).
func (a *Adapter) Cancel(ctx context.Context, runID string) error {
	if runID == "" {
		return fmt.Errorf("gitlab.Cancel: empty run id: %w", engine.ErrInvalidInput)
	}
	projectID, err := a.extra.requireProjectID()
	if err != nil {
		return err
	}
	pipelineID, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return fmt.Errorf("gitlab.Cancel: runID %q is not a GitLab pipeline id: %w", runID, engine.ErrInvalidInput)
	}

	// GitLab's cancel endpoint is idempotent: calling it on a finished
	// pipeline returns the pipeline with status unchanged (not an error).
	// We detect this by re-reading the resulting pipeline status and
	// surfacing ErrAlreadyTerminal to callers when appropriate.
	path := fmt.Sprintf("/projects/%d/pipelines/%d/cancel", projectID, pipelineID)
	req, err := a.c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("gitlab.Cancel: %w", err)
	}

	var pipe gitlabPipeline
	if err := a.c.doJSON(req, &pipe); err != nil {
		// 400 from GitLab sometimes means "cannot be canceled (already
		// finished)" — distinct from ErrInvalidInput usage elsewhere.
		if errors.Is(err, engine.ErrInvalidInput) {
			return fmt.Errorf("gitlab.Cancel %s: %w", runID, engine.ErrAlreadyTerminal)
		}
		return fmt.Errorf("gitlab.Cancel %s: %w", runID, err)
	}

	phase := mapGitLabStatus(pipe.Status)
	if phase.IsTerminal() && phase != engine.RunPhaseCancelled {
		// Pipeline finished before cancellation took effect.
		return fmt.Errorf("gitlab.Cancel %s: status=%s: %w", runID, pipe.Status, engine.ErrAlreadyTerminal)
	}
	return nil
}
