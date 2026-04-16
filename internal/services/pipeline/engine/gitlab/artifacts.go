package gitlab

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetArtifacts returns the list of artifacts produced by jobs belonging to
// the given pipeline. GitLab stores artifacts at job level; we flatten them
// into engine.Artifact entries so the UI does not need to understand the
// job hierarchy.
//
// Note: this implementation inspects each job's `artifacts_file` metadata
// (filename + size) which is returned by the /pipelines/:id/jobs list
// endpoint. Downloading the actual artifact is NOT performed; the `URL`
// field is the GitLab job page (users can click through to download).
// Direct artifact download can be added in a M18b follow-up if needed.
func (a *Adapter) GetArtifacts(ctx context.Context, runID string) ([]*engine.Artifact, error) {
	if runID == "" {
		return nil, fmt.Errorf("gitlab.GetArtifacts: empty run id: %w", engine.ErrInvalidInput)
	}
	projectID, err := a.extra.requireProjectID()
	if err != nil {
		return nil, err
	}
	pipelineID, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gitlab.GetArtifacts: runID %q is not a GitLab pipeline id: %w", runID, engine.ErrInvalidInput)
	}

	jobs, err := a.fetchPipelineJobs(ctx, projectID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("gitlab.GetArtifacts: %w", err)
	}

	// Pre-size for the common case (most jobs don't produce artifacts).
	out := make([]*engine.Artifact, 0, len(jobs))
	for _, j := range jobs {
		if j.ArtifactsFile == nil || j.ArtifactsFile.Filename == "" {
			continue
		}
		createdAt := time.Time{}
		if j.FinishedAt != nil {
			createdAt = *j.FinishedAt
		} else if j.StartedAt != nil {
			createdAt = *j.StartedAt
		}
		out = append(out, &engine.Artifact{
			Name:      j.ArtifactsFile.Filename,
			Kind:      "file",
			SizeBytes: j.ArtifactsFile.Size,
			// Direct download URL is projectID/jobs/:id/artifacts — the
			// caller can construct a signed download via the GitLab UI.
			URL:       artifactJobURL(a.c.baseURL.Scheme, a.c.baseURL.Host, projectID, j.ID),
			CreatedAt: createdAt,
		})
	}
	return out, nil
}

// artifactJobURL returns the browser-facing URL to the job page. It is
// constructed from the api base URL's scheme + host (dropping the /api/v4
// path segment).
func artifactJobURL(scheme, host string, projectID, jobID int64) string {
	return fmt.Sprintf("%s://%s/-/jobs/%d", scheme, host, jobID)
	// (`projectID` intentionally omitted — GitLab's /-/jobs/:id route is
	// project-agnostic. Using the unambiguous path keeps URL construction
	// simple without needing the project slug, which isn't in the API
	// response.)
}
