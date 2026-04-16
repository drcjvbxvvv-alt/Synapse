package gitlab

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ExtraConfig holds GitLab-specific settings that live inside
// CIEngineConfig.ExtraJSON.
//
// Example JSON payload stored in DB:
//
//	{"project_id": 42, "default_ref": "main"}
//
// ProjectID is mandatory (the adapter refuses Trigger without it).
// DefaultRef is optional; when empty and the TriggerRequest doesn't supply a
// ref, the adapter returns ErrInvalidInput.
type ExtraConfig struct {
	ProjectID  int64  `json:"project_id"`
	DefaultRef string `json:"default_ref,omitempty"`
}

// parseExtra parses ExtraJSON. An empty string yields a zero ExtraConfig —
// callers can then still fail later if ProjectID == 0 for operations that
// require it.
func parseExtra(raw string) (*ExtraConfig, error) {
	cfg := &ExtraConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("gitlab: invalid extra_json: %w: %w", err, engine.ErrInvalidInput)
	}
	return cfg, nil
}

// requireProjectID returns ErrInvalidInput when ProjectID was not set.
// Used by execution methods (Trigger/GetRun/…) that cannot run without it.
func (e *ExtraConfig) requireProjectID() (int64, error) {
	if e == nil || e.ProjectID <= 0 {
		return 0, fmt.Errorf("gitlab: project_id is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return e.ProjectID, nil
}
