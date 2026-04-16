package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetRun returns the current status of a GitHub Actions run.
//
// runID formats:
//   - "<number>"                 → direct run id, fetched via /actions/runs/:id
//   - "dispatch:<ref>@<epoch>"   → placeholder from Trigger() when the run id
//     hadn't materialised in time; GetRun attempts to resolve it, then
//     delegates to the number path.
func (a *Adapter) GetRun(ctx context.Context, runID string) (*engine.RunStatus, error) {
	id, ref, cutoff, err := parseRunID(runID)
	if err != nil {
		return nil, fmt.Errorf("github.GetRun: %w", err)
	}
	owner, repo, err := a.extra.requireOwnerRepo()
	if err != nil {
		return nil, err
	}

	if id == 0 {
		// Placeholder — try to resolve once.
		workflowID := a.extra.WorkflowID
		if workflowID == "" {
			return nil, fmt.Errorf("github.GetRun: workflow_id required to resolve placeholder: %w", engine.ErrInvalidInput)
		}
		resolved, _, err := a.discoverRunID(ctx, owner, repo, workflowID, ref, cutoff)
		if err != nil {
			return nil, err
		}
		if resolved == 0 {
			// Still not resolvable — report as pending so the UI renders
			// something useful.
			return &engine.RunStatus{
				RunID:      runID,
				ExternalID: runID,
				Phase:      engine.RunPhasePending,
				Raw:        "dispatch-pending",
				Message:    "workflow_dispatch created; run not yet visible via API",
			}, nil
		}
		id = resolved
		runID = strconv.FormatInt(id, 10)
	}

	run, err := a.fetchRun(ctx, owner, repo, id)
	if err != nil {
		return nil, err
	}
	rs := buildRunStatus(strconv.FormatInt(id, 10), run)

	// Best-effort: pull the jobs to surface per-step status. Failure to list
	// is non-fatal; we attach the error to Raw and return pipeline-level
	// status so the UI stays stable.
	if jobs, jerr := a.fetchJobs(ctx, owner, repo, id); jerr == nil {
		rs.Steps = stepsFromJobs(jobs)
	} else {
		rs.Raw += " (jobs unavailable: " + jerr.Error() + ")"
	}
	return rs, nil
}

// fetchRun reads GET /repos/:owner/:repo/actions/runs/:id.
func (a *Adapter) fetchRun(ctx context.Context, owner, repo string, id int64) (*workflowRun, error) {
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, id)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var wr workflowRun
	if err := a.c.doJSON(req, &wr); err != nil {
		return nil, fmt.Errorf("github.GetRun %d: %w", id, err)
	}
	return &wr, nil
}

// fetchJobs reads GET /repos/:owner/:repo/actions/runs/:id/jobs.
func (a *Adapter) fetchJobs(ctx context.Context, owner, repo string, id int64) ([]workflowJob, error) {
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs?per_page=100", owner, repo, id)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var list workflowJobList
	if err := a.c.doJSON(req, &list); err != nil {
		return nil, err
	}
	return list.Jobs, nil
}

// buildRunStatus projects workflowRun into engine.RunStatus.
func buildRunStatus(runID string, wr *workflowRun) *engine.RunStatus {
	phase := mapGitHubStatus(wr.Status, wr.Conclusion)
	raw := wr.Status
	if wr.Conclusion != "" {
		raw = wr.Status + "/" + wr.Conclusion
	}
	var started, finished *time.Time
	if wr.RunStartedAt != nil {
		t := wr.RunStartedAt.UTC()
		started = &t
	} else if wr.CreatedAt != nil {
		t := wr.CreatedAt.UTC()
		started = &t
	}
	if wr.Status == "completed" && wr.UpdatedAt != nil {
		t := wr.UpdatedAt.UTC()
		finished = &t
	}
	return &engine.RunStatus{
		RunID:      runID,
		ExternalID: runID,
		Phase:      phase,
		Raw:        raw,
		StartedAt:  started,
		FinishedAt: finished,
	}
}

// stepsFromJobs converts each workflowJob into a StepStatus.
func stepsFromJobs(jobs []workflowJob) []engine.StepStatus {
	if len(jobs) == 0 {
		return []engine.StepStatus{}
	}
	out := make([]engine.StepStatus, 0, len(jobs))
	for _, j := range jobs {
		raw := j.Status
		if j.Conclusion != "" {
			raw = j.Status + "/" + j.Conclusion
		}
		out = append(out, engine.StepStatus{
			Name:       j.Name,
			Phase:      mapGitHubStatus(j.Status, j.Conclusion),
			Raw:        raw,
			StartedAt:  j.StartedAt,
			FinishedAt: j.CompletedAt,
		})
	}
	return out
}
