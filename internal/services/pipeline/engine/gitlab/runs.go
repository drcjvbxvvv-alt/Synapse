package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetRun fetches current status from GitLab by combining
// GET /projects/:id/pipelines/:id with
// GET /projects/:id/pipelines/:id/jobs. The job list becomes the
// per-step breakdown in engine.RunStatus.Steps.
//
// runID must be the GitLab pipeline ID (decimal string) as returned by
// Trigger.ExternalID.
func (a *Adapter) GetRun(ctx context.Context, runID string) (*engine.RunStatus, error) {
	if runID == "" {
		return nil, fmt.Errorf("gitlab.GetRun: empty run id: %w", engine.ErrInvalidInput)
	}
	projectID, err := a.extra.requireProjectID()
	if err != nil {
		return nil, err
	}
	pipelineID, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gitlab.GetRun: runID %q is not a GitLab pipeline id: %w", runID, engine.ErrInvalidInput)
	}

	pipe, err := a.fetchPipeline(ctx, projectID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("gitlab.GetRun: %w", err)
	}
	jobs, err := a.fetchPipelineJobs(ctx, projectID, pipelineID)
	if err != nil {
		// Failure to enumerate jobs is non-fatal: status itself is still
		// useful (e.g. a freshly queued pipeline with no jobs yet). Log to
		// Raw rather than failing the whole call.
		return buildRunStatusNoJobs(runID, pipe, err), nil
	}

	return buildRunStatus(runID, pipe, jobs), nil
}

// fetchPipeline wraps GET /projects/:id/pipelines/:id.
func (a *Adapter) fetchPipeline(ctx context.Context, projectID, pipelineID int64) (*gitlabPipeline, error) {
	path := fmt.Sprintf("/projects/%d/pipelines/%d", projectID, pipelineID)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var pipe gitlabPipeline
	if err := a.c.doJSON(req, &pipe); err != nil {
		return nil, err
	}
	return &pipe, nil
}

// fetchPipelineJobs wraps GET /projects/:id/pipelines/:id/jobs. The default
// page size is 20 in GitLab; for M18b we accept that (most CI pipelines have
// <20 jobs). Pagination is a TODO for large pipelines (handled in M18b follow-up).
func (a *Adapter) fetchPipelineJobs(ctx context.Context, projectID, pipelineID int64) ([]gitlabJob, error) {
	path := fmt.Sprintf("/projects/%d/pipelines/%d/jobs", projectID, pipelineID)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var jobs []gitlabJob
	if err := a.c.doJSON(req, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// buildRunStatus materialises the unified RunStatus from the GitLab responses.
func buildRunStatus(runID string, pipe *gitlabPipeline, jobs []gitlabJob) *engine.RunStatus {
	status := &engine.RunStatus{
		RunID:      runID,
		ExternalID: strconv.FormatInt(pipe.ID, 10),
		Phase:      mapGitLabStatus(pipe.Status),
		Raw:        pipe.Status,
		StartedAt:  pipe.StartedAt,
		FinishedAt: pipe.FinishedAt,
	}
	steps := make([]engine.StepStatus, 0, len(jobs))
	for _, j := range jobs {
		steps = append(steps, engine.StepStatus{
			Name:       j.Name,
			Phase:      mapGitLabStatus(j.Status),
			Raw:        j.Status,
			StartedAt:  j.StartedAt,
			FinishedAt: j.FinishedAt,
		})
	}
	status.Steps = steps
	return status
}

// buildRunStatusNoJobs is used when the jobs list endpoint failed; we still
// return pipeline-level status so the UI can show something useful.
func buildRunStatusNoJobs(runID string, pipe *gitlabPipeline, listErr error) *engine.RunStatus {
	status := buildRunStatus(runID, pipe, nil)
	// Encode the list error in Raw so operators can see why Steps is empty
	// without turning a 500 into a hard failure.
	status.Raw = pipe.Status + " (jobs unavailable: " + listErr.Error() + ")"
	return status
}
