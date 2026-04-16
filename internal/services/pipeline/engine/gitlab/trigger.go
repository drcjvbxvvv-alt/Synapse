package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Trigger creates a new pipeline on the configured GitLab project.
//
// GitLab endpoint: POST /api/v4/projects/:id/pipeline
//
// Ref resolution order:
//  1. engine.TriggerRequest.Ref (if non-empty)
//  2. ExtraConfig.DefaultRef (if set in CIEngineConfig.ExtraJSON)
//  3. error (GitLab requires ref)
//
// Variables are passed through verbatim.
func (a *Adapter) Trigger(ctx context.Context, req *engine.TriggerRequest) (*engine.TriggerResult, error) {
	if req == nil {
		return nil, fmt.Errorf("gitlab.Trigger: nil request: %w", engine.ErrInvalidInput)
	}
	projectID, err := a.extra.requireProjectID()
	if err != nil {
		return nil, err
	}
	ref := req.Ref
	if ref == "" {
		ref = a.extra.DefaultRef
	}
	if ref == "" {
		return nil, fmt.Errorf("gitlab.Trigger: ref missing (neither TriggerRequest.Ref nor extra.default_ref set): %w", engine.ErrInvalidInput)
	}

	body := triggerRequest{
		Ref:       ref,
		Variables: convertVariables(req.Variables),
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gitlab.Trigger: marshal body: %w", err)
	}

	path := fmt.Sprintf("/projects/%d/pipeline", projectID)
	httpReq, err := a.c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("gitlab.Trigger: %w", err)
	}

	var pipe gitlabPipeline
	if err := a.c.doJSON(httpReq, &pipe); err != nil {
		return nil, fmt.Errorf("gitlab.Trigger: %w", err)
	}

	queuedAt := time.Now().UTC()
	if pipe.CreatedAt != nil {
		queuedAt = *pipe.CreatedAt
	}

	// External ID = GitLab pipeline id; callers store it in Synapse's
	// PipelineRun.ExternalID and use it to poll / cancel later.
	externalID := strconv.FormatInt(pipe.ID, 10)

	return &engine.TriggerResult{
		RunID:      externalID, // adapter uses the external id as its opaque run handle
		ExternalID: externalID,
		URL:        pipe.WebURL,
		QueuedAt:   queuedAt,
	}, nil
}

// convertVariables translates the engine-level map into GitLab's array form.
// Keeps iteration order deterministic for easier testing.
func convertVariables(vars map[string]string) []triggerVariable {
	if len(vars) == 0 {
		return nil
	}
	out := make([]triggerVariable, 0, len(vars))
	for k, v := range vars {
		out = append(out, triggerVariable{Key: k, Value: v})
	}
	return out
}
