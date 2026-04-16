package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetArtifacts returns the list of archived artifacts for a Jenkins build.
//
// Jenkins inlines artifact metadata (filename + relative path inside the
// workspace) in the build JSON. The adapter converts each entry into an
// engine.Artifact with a URL pointing at Jenkins' download route:
//
//	/job/:path/:num/artifact/:relative_path
//
// Artifact sizes are NOT included in the inline metadata; callers that
// need size must make a HEAD request against the download URL. Leaving
// SizeBytes=0 signals "unknown" to the UI.
//
// runID must be a numeric build id; queue-prefixed IDs return
// ErrInvalidInput because queue items have no artifacts.
func (a *Adapter) GetArtifacts(ctx context.Context, runID string) ([]*engine.Artifact, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("jenkins.GetArtifacts: empty run id: %w", engine.ErrInvalidInput)
	}
	if strings.HasPrefix(runID, queueRunIDPrefix) {
		// Queued item → no artifacts yet. Return empty slice (not error)
		// so the UI can treat it the same as a running build with none
		// archived yet.
		return []*engine.Artifact{}, nil
	}
	jobPath, err := a.extra.requireJobPath()
	if err != nil {
		return nil, err
	}
	buildNum, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("jenkins.GetArtifacts: runID %q is not a build number: %w", runID, engine.ErrInvalidInput)
	}

	path := buildJobURLPath(jobPath) + "/" + strconv.FormatInt(buildNum, 10) + "/api/json"
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var b jenkinsBuild
	if err := a.c.doJSON(req, &b); err != nil {
		return nil, fmt.Errorf("jenkins.GetArtifacts: %w", err)
	}

	// Build timestamp approximates the artifact creation time.
	var createdAt time.Time
	if b.Timestamp > 0 {
		createdAt = time.UnixMilli(b.Timestamp).UTC()
	}

	out := make([]*engine.Artifact, 0, len(b.Artifacts))
	for _, art := range b.Artifacts {
		rel := art.RelativePath
		if rel == "" {
			rel = art.FileName
		}
		out = append(out, &engine.Artifact{
			Name:      art.FileName,
			Kind:      "file",
			URL:       a.artifactURL(jobPath, buildNum, rel),
			CreatedAt: createdAt,
		})
	}
	return out, nil
}

// artifactURL constructs the direct download URL for an archived file.
func (a *Adapter) artifactURL(jobPath string, buildNum int64, relPath string) string {
	return strings.TrimRight(a.c.baseURL.String(), "/") +
		buildJobURLPath(jobPath) + "/" +
		strconv.FormatInt(buildNum, 10) +
		"/artifact/" + relPath
}
