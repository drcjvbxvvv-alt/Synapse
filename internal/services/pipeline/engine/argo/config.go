package argo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ExtraConfig holds Argo-specific settings from CIEngineConfig.ExtraJSON.
//
// Example:
//
//	{
//	  "workflow_template_name": "build-app",
//	  "namespace": "ci-argo",
//	  "service_account_name": "workflow-runner"
//	}
//
// WorkflowTemplateName is mandatory: the adapter creates a Workflow whose
// spec references this WorkflowTemplate via spec.workflowTemplateRef. This
// mirrors the Tekton adapter's "Pipeline reference" design — Synapse
// does not itself define the workflow DAG, just triggers it.
type ExtraConfig struct {
	WorkflowTemplateName string `json:"workflow_template_name"`
	Namespace            string `json:"namespace"`
	ServiceAccountName   string `json:"service_account_name,omitempty"`
}

// parseExtra parses ExtraJSON. An empty string yields a zero ExtraConfig.
func parseExtra(raw string) (*ExtraConfig, error) {
	cfg := &ExtraConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("argo: invalid extra_json: %w: %w", err, engine.ErrInvalidInput)
	}
	return cfg, nil
}

// requireTargets validates mandatory fields for operations that create /
// mutate workflows.
func (e *ExtraConfig) requireTargets() (templateName, namespace string, err error) {
	if e == nil {
		return "", "", fmt.Errorf("argo: extra config missing: %w", engine.ErrInvalidInput)
	}
	templateName = strings.TrimSpace(e.WorkflowTemplateName)
	namespace = strings.TrimSpace(e.Namespace)
	if templateName == "" {
		return "", "", fmt.Errorf("argo: workflow_template_name is required in extra_json: %w", engine.ErrInvalidInput)
	}
	if namespace == "" {
		return "", "", fmt.Errorf("argo: namespace is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return templateName, namespace, nil
}

// requireNamespace is the read-only variant (GetRun / Cancel / GetArtifacts).
func (e *ExtraConfig) requireNamespace() (string, error) {
	if e == nil {
		return "", fmt.Errorf("argo: extra config missing: %w", engine.ErrInvalidInput)
	}
	ns := strings.TrimSpace(e.Namespace)
	if ns == "" {
		return "", fmt.Errorf("argo: namespace is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return ns, nil
}
