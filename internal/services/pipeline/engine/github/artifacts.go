package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetArtifacts returns artifacts produced by a run, listed via
// GET /repos/{owner}/{repo}/actions/runs/{run_id}/artifacts.
//
// GitHub returns names + sizes + archive_download_url inline. Placeholder
// RunIDs (from Trigger timeout) are treated as "no artifacts yet" and
// return an empty slice rather than an error, matching the Jenkins adapter.
func (a *Adapter) GetArtifacts(ctx context.Context, runID string) ([]*engine.Artifact, error) {
	id, _, _, err := parseRunID(runID)
	if err != nil {
		return nil, fmt.Errorf("github.GetArtifacts: %w", err)
	}
	if id == 0 {
		// Placeholder — no run yet, so no artifacts.
		return []*engine.Artifact{}, nil
	}
	owner, repo, err := a.extra.requireOwnerRepo()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/artifacts?per_page=100", owner, repo, id)
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("github.GetArtifacts: %w", err)
	}
	var list artifactsList
	if err := a.c.doJSON(req, &list); err != nil {
		return nil, fmt.Errorf("github.GetArtifacts %d: %w", id, err)
	}
	if len(list.Artifacts) == 0 {
		return []*engine.Artifact{}, nil
	}
	out := make([]*engine.Artifact, 0, len(list.Artifacts))
	for _, e := range list.Artifacts {
		createdAt := time.Time{}
		if e.CreatedAt != nil {
			createdAt = e.CreatedAt.UTC()
		}
		out = append(out, &engine.Artifact{
			Name:      e.Name,
			Kind:      "file",
			URL:       e.ArchiveDownloadURL,
			SizeBytes: e.SizeInBytes,
			Digest:    strconv.FormatInt(e.ID, 10), // surface artifact id so UI can deep-link
			CreatedAt: createdAt,
		})
	}
	return out, nil
}
