package tekton

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ExtraConfig holds Tekton-specific settings that live in
// CIEngineConfig.ExtraJSON.
//
// Example:
//
//	{
//	  "pipeline_name": "build-saas-java-a",
//	  "namespace": "ci-tekton",
//	  "service_account_name": "pipeline-runner"
//	}
//
// - PipelineName (mandatory): name of a Tekton Pipeline CR in the target
//   namespace — referenced via spec.pipelineRef.name when creating a
//   PipelineRun.
// - Namespace (mandatory): where PipelineRun and its TaskRun children live.
// - ServiceAccountName (optional): set on spec.taskRunTemplate.serviceAccountName
//   so Tekton uses a non-default SA.
type ExtraConfig struct {
	PipelineName       string `json:"pipeline_name"`
	Namespace          string `json:"namespace"`
	ServiceAccountName string `json:"service_account_name,omitempty"`
}

// parseExtra parses ExtraJSON. An empty string yields a zero ExtraConfig —
// callers fail later when they need specific fields.
func parseExtra(raw string) (*ExtraConfig, error) {
	cfg := &ExtraConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("tekton: invalid extra_json: %w: %w", err, engine.ErrInvalidInput)
	}
	return cfg, nil
}

// requireTargets validates that all mandatory fields are present and
// returns them in normalised form (leading/trailing whitespace stripped).
func (e *ExtraConfig) requireTargets() (pipelineName, namespace string, err error) {
	if e == nil {
		return "", "", fmt.Errorf("tekton: extra config missing: %w", engine.ErrInvalidInput)
	}
	pipelineName = strings.TrimSpace(e.PipelineName)
	namespace = strings.TrimSpace(e.Namespace)
	if pipelineName == "" {
		return "", "", fmt.Errorf("tekton: pipeline_name is required in extra_json: %w", engine.ErrInvalidInput)
	}
	if namespace == "" {
		return "", "", fmt.Errorf("tekton: namespace is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return pipelineName, namespace, nil
}

// requireNamespace is the lightweight variant used by read-only methods
// (GetRun, StreamLogs, GetArtifacts) that don't need pipeline_name.
func (e *ExtraConfig) requireNamespace() (string, error) {
	if e == nil {
		return "", fmt.Errorf("tekton: extra config missing: %w", engine.ErrInvalidInput)
	}
	ns := strings.TrimSpace(e.Namespace)
	if ns == "" {
		return "", fmt.Errorf("tekton: namespace is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return ns, nil
}
