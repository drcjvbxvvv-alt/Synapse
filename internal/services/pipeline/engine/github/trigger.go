package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Trigger — workflow_dispatch + polling to discover the run id
// ---------------------------------------------------------------------------
//
// GitHub's trigger endpoint (POST /workflows/{id}/dispatches) returns 204
// with **no run id** in the response. The canonical way to discover the
// created run is to list /actions/runs filtered by event=workflow_dispatch
// and created>=<cutoff> and pick the first match.
//
// The cutoff is slightly before the dispatch call to account for clock
// skew between Synapse and GitHub. We look up to `maxRunsPage` recent runs
// per poll.
//
// If the run doesn't appear within `dispatchPollTimeout`, the adapter
// returns a result with RunID="dispatch:<ref>@<epoch>" — callers can poll
// GetRun later (which knows the prefix and can still resolve).

const (
	dispatchPollInterval = 1 * time.Second
	dispatchPollTimeout  = 10 * time.Second
	dispatchPrefix       = "dispatch:"
	maxRunsPage          = 10
)

// Trigger creates a workflow dispatch event.
func (a *Adapter) Trigger(ctx context.Context, req *engine.TriggerRequest) (*engine.TriggerResult, error) {
	if req == nil {
		return nil, fmt.Errorf("github.Trigger: nil request: %w", engine.ErrInvalidInput)
	}
	owner, repo, workflowID, err := a.extra.requireTargets()
	if err != nil {
		return nil, err
	}

	ref := req.Ref
	if ref == "" {
		ref = a.extra.DefaultRef
	}
	if ref == "" {
		return nil, fmt.Errorf("github.Trigger: ref missing (neither TriggerRequest.Ref nor extra.default_ref set): %w", engine.ErrInvalidInput)
	}

	// Record cutoff slightly BEFORE the dispatch so polling can match by
	// created_at. 10s window handles modest clock drift.
	cutoff := time.Now().Add(-10 * time.Second).UTC()

	body, err := json.Marshal(dispatchRequest{Ref: ref, Inputs: req.Variables})
	if err != nil {
		return nil, fmt.Errorf("github.Trigger: marshal body: %w", err)
	}
	dispatchPath := fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/dispatches",
		owner, repo, url.PathEscape(workflowID))
	r, err := a.c.newRequest(ctx, http.MethodPost, dispatchPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("github.Trigger: %w", err)
	}
	// 204 No Content is the success path — doJSON handles it gracefully.
	if err := a.c.doJSON(r, nil); err != nil {
		return nil, fmt.Errorf("github.Trigger: %w", err)
	}

	// Try to resolve the run id by polling /actions/runs.
	runID, htmlURL, err := a.discoverRunID(ctx, owner, repo, workflowID, ref, cutoff)
	if err != nil {
		return nil, err
	}
	queuedAt := time.Now().UTC()
	if runID > 0 {
		return &engine.TriggerResult{
			RunID:      strconv.FormatInt(runID, 10),
			ExternalID: strconv.FormatInt(runID, 10),
			URL:        htmlURL,
			QueuedAt:   queuedAt,
		}, nil
	}
	// Timeout: encode the context for caller-side polling of GetRun.
	placeholder := fmt.Sprintf("%s%s@%d", dispatchPrefix, ref, cutoff.Unix())
	return &engine.TriggerResult{
		RunID:      placeholder,
		ExternalID: placeholder,
		QueuedAt:   queuedAt,
	}, nil
}

// discoverRunID polls /actions/workflows/{id}/runs until a workflow_dispatch
// run appears on the given ref with created_at >= cutoff. Returns (0, "", nil)
// on timeout (caller surfaces the placeholder RunID).
func (a *Adapter) discoverRunID(ctx context.Context, owner, repo, workflowID, ref string, cutoff time.Time) (int64, string, error) {
	deadline, cancel := context.WithTimeout(ctx, dispatchPollTimeout)
	defer cancel()
	ticker := time.NewTicker(dispatchPollInterval)
	defer ticker.Stop()

	path := fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/runs?event=workflow_dispatch&branch=%s&per_page=%d",
		owner, repo, url.PathEscape(workflowID), url.QueryEscape(ref), maxRunsPage)

	for {
		req, err := a.c.newRequest(deadline, http.MethodGet, path, nil)
		if err != nil {
			return 0, "", err
		}
		var list workflowRunList
		if err := a.c.doJSON(req, &list); err != nil {
			return 0, "", fmt.Errorf("github.Trigger: poll runs: %w", err)
		}
		// GitHub returns newest-first; find the first run created at or after
		// cutoff with matching event (we filtered via query, but double-check).
		for _, wr := range list.WorkflowRuns {
			if wr.Event != "" && wr.Event != "workflow_dispatch" {
				continue
			}
			if wr.CreatedAt == nil {
				continue
			}
			if !wr.CreatedAt.Before(cutoff) {
				return wr.ID, wr.HTMLURL, nil
			}
		}
		select {
		case <-deadline.Done():
			return 0, "", nil
		case <-ticker.C:
			// next poll
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers shared between Trigger and GetRun
// ---------------------------------------------------------------------------

// parseRunID returns the numeric GitHub run id given a RunID that is
// either a plain number or the "dispatch:<ref>@<epoch>" placeholder. For
// the placeholder it returns (0, ref, cutoff, nil) so callers can
// re-resolve via discoverRunID().
func parseRunID(runID string) (id int64, placeholderRef string, cutoff time.Time, err error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return 0, "", time.Time{}, fmt.Errorf("empty run id: %w", engine.ErrInvalidInput)
	}
	if strings.HasPrefix(runID, dispatchPrefix) {
		rest := strings.TrimPrefix(runID, dispatchPrefix)
		atIdx := strings.LastIndex(rest, "@")
		if atIdx == -1 {
			return 0, "", time.Time{}, fmt.Errorf("malformed placeholder %q: %w", runID, engine.ErrInvalidInput)
		}
		epoch, err := strconv.ParseInt(rest[atIdx+1:], 10, 64)
		if err != nil {
			return 0, "", time.Time{}, fmt.Errorf("placeholder epoch: %w: %w", err, engine.ErrInvalidInput)
		}
		return 0, rest[:atIdx], time.Unix(epoch, 0).UTC(), nil
	}
	n, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("runID %q is not a number: %w", runID, engine.ErrInvalidInput)
	}
	return n, "", time.Time{}, nil
}
