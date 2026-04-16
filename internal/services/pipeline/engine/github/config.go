package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ExtraConfig holds GitHub Actions-specific settings stored in
// CIEngineConfig.ExtraJSON.
//
// Example:
//
//	{
//	  "owner":       "my-org",
//	  "repo":        "my-repo",
//	  "workflow_id": "build.yml",
//	  "default_ref": "main"
//	}
//
// - Owner + Repo (required): identifies the repository.
// - WorkflowID (required):  can be either the workflow filename
//   (e.g. `build.yml`) or its numeric id. Both are accepted by
//   /actions/workflows/{workflow_id}/dispatches.
// - DefaultRef (optional): used when TriggerRequest.Ref is empty.
type ExtraConfig struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	WorkflowID string `json:"workflow_id"`
	DefaultRef string `json:"default_ref,omitempty"`
}

// parseExtra parses ExtraJSON. Empty → zero config.
func parseExtra(raw string) (*ExtraConfig, error) {
	cfg := &ExtraConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("github: invalid extra_json: %w: %w", err, engine.ErrInvalidInput)
	}
	return cfg, nil
}

// requireTargets validates mandatory fields for operations that hit the
// repository-scoped APIs. Returned values are trimmed.
func (e *ExtraConfig) requireTargets() (owner, repo, workflow string, err error) {
	if e == nil {
		return "", "", "", fmt.Errorf("github: extra config missing: %w", engine.ErrInvalidInput)
	}
	owner = strings.TrimSpace(e.Owner)
	repo = strings.TrimSpace(e.Repo)
	workflow = strings.TrimSpace(e.WorkflowID)
	if owner == "" {
		return "", "", "", fmt.Errorf("github: owner is required in extra_json: %w", engine.ErrInvalidInput)
	}
	if repo == "" {
		return "", "", "", fmt.Errorf("github: repo is required in extra_json: %w", engine.ErrInvalidInput)
	}
	if workflow == "" {
		return "", "", "", fmt.Errorf("github: workflow_id is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return owner, repo, workflow, nil
}

// requireOwnerRepo is the read-only variant used by Cancel / GetRun / etc.
// that don't need workflow_id.
func (e *ExtraConfig) requireOwnerRepo() (owner, repo string, err error) {
	if e == nil {
		return "", "", fmt.Errorf("github: extra config missing: %w", engine.ErrInvalidInput)
	}
	owner = strings.TrimSpace(e.Owner)
	repo = strings.TrimSpace(e.Repo)
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("github: owner and repo are required: %w", engine.ErrInvalidInput)
	}
	return owner, repo, nil
}
