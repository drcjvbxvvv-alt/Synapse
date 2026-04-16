package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Cancel stops a running Jenkins build.
//
// Jenkins exposes two cancellation paths:
//   - /job/:path/:build/stop          — graceful stop (preferred)
//   - /queue/cancelItem?id=:id        — removes an item from the queue
//     before it becomes a build
//
// This method dispatches based on the runID format (see trigger.go):
//   - "queue:<id>"       → queue/cancelItem
//   - "<build number>"   → /job/:path/:num/stop
//
// Errors:
//   - If the build has already finished (GET returns a build with
//     `result != ""` and `!building`), returns ErrAlreadyTerminal.
//   - 404 on the build/queue endpoint → ErrNotFound.
//   - Other non-2xx mapped via mapHTTPStatus.
func (a *Adapter) Cancel(ctx context.Context, runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("jenkins.Cancel: empty run id: %w", engine.ErrInvalidInput)
	}
	jobPath, err := a.extra.requireJobPath()
	if err != nil {
		return err
	}

	// Queue variant.
	if strings.HasPrefix(runID, queueRunIDPrefix) {
		idStr := strings.TrimPrefix(runID, queueRunIDPrefix)
		qid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("jenkins.Cancel: invalid queue id in %q: %w", runID, engine.ErrInvalidInput)
		}
		path := fmt.Sprintf("/queue/cancelItem?id=%d", qid)
		if err := a.c.doMutation(ctx, http.MethodPost, path, nil, nil); err != nil {
			return fmt.Errorf("jenkins.Cancel queue %d: %w", qid, err)
		}
		return nil
	}

	// Build-number variant: check current status first so we can distinguish
	// "already terminal" from a normal cancel.
	buildNum, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return fmt.Errorf("jenkins.Cancel: runID %q is not a build number: %w", runID, engine.ErrInvalidInput)
	}
	rs, err := a.getRunByBuildNumber(ctx, jobPath, buildNum)
	if err != nil {
		// Let ErrNotFound bubble up verbatim; wrap other errors for context.
		return fmt.Errorf("jenkins.Cancel %d: %w", buildNum, err)
	}
	if rs.Phase.IsTerminal() {
		return fmt.Errorf("jenkins.Cancel %d: phase=%s: %w", buildNum, rs.Phase, engine.ErrAlreadyTerminal)
	}

	stopPath := buildJobURLPath(jobPath) + "/" + strconv.FormatInt(buildNum, 10) + "/stop"
	if err := a.c.doMutation(ctx, http.MethodPost, stopPath, nil, nil); err != nil {
		return fmt.Errorf("jenkins.Cancel %d: %w", buildNum, err)
	}
	return nil
}
