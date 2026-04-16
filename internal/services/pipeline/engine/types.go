// Package engine defines the CI Engine Adapter abstraction.
//
// Multiple CI engines (Native K8s Job, GitLab CI, Jenkins, Tekton, etc.) can
// coexist by implementing the CIEngineAdapter interface. See ADR-015 for the
// architectural rationale.
package engine

import (
	"time"
)

// EngineType identifies a CI engine implementation.
type EngineType string

// Built-in engine types. Additional adapters register their own type constants
// via the factory Register() mechanism.
const (
	// EngineNative is the built-in K8s Job executor. Always available; no
	// external dependency (see ADR-001).
	EngineNative EngineType = "native"

	// EngineGitLab triggers pipelines on a remote GitLab instance (M18b).
	EngineGitLab EngineType = "gitlab"

	// EngineJenkins triggers jobs on a remote Jenkins controller (M18c).
	EngineJenkins EngineType = "jenkins"

	// EngineTekton creates Tekton PipelineRun CRDs in a target cluster (M18d).
	EngineTekton EngineType = "tekton"

	// EngineArgo creates Argo Workflows in a target cluster (M18e).
	EngineArgo EngineType = "argo"

	// EngineGitHub triggers workflow_dispatch on GitHub Actions (M18e).
	EngineGitHub EngineType = "github"
)

// IsValid reports whether t is a known engine type. Adapter implementations
// should use this for input validation before dispatching to the factory.
func (t EngineType) IsValid() bool {
	switch t {
	case EngineNative, EngineGitLab, EngineJenkins, EngineTekton, EngineArgo, EngineGitHub:
		return true
	}
	return false
}

// String implements fmt.Stringer for logging.
func (t EngineType) String() string { return string(t) }

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

// EngineCapabilities describes the feature set supported by an adapter. The
// UI hides unsupported options; services use this to validate user input.
//
// Adapters MUST return a stable capabilities value (per-process constant); the
// factory may cache it.
type EngineCapabilities struct {
	// SupportsDAG indicates support for non-linear step topology.
	SupportsDAG bool `json:"supports_dag"`
	// SupportsMatrix indicates support for matrix builds.
	SupportsMatrix bool `json:"supports_matrix"`
	// SupportsArtifacts indicates the engine persists artifacts across steps.
	SupportsArtifacts bool `json:"supports_artifacts"`
	// SupportsSecrets indicates the engine can inject secrets into steps.
	SupportsSecrets bool `json:"supports_secrets"`
	// SupportsCaching indicates the engine supports inter-run caching.
	SupportsCaching bool `json:"supports_caching"`
	// SupportsApprovals indicates the engine supports manual approval gates.
	SupportsApprovals bool `json:"supports_approvals"`
	// SupportsNotification indicates the engine emits its own notifications.
	// When false, Synapse NotifyChannel bridges events on the engine's behalf.
	SupportsNotification bool `json:"supports_notification"`
	// SupportsLiveLog indicates the engine supports real-time log streaming.
	SupportsLiveLog bool `json:"supports_live_log"`
}

// ---------------------------------------------------------------------------
// Trigger
// ---------------------------------------------------------------------------

// TriggerRequest is the unified input for starting a pipeline run. Adapters
// may ignore fields they do not support; see EngineCapabilities for gating.
type TriggerRequest struct {
	// PipelineID refers to the logical Synapse pipeline (models.Pipeline.ID).
	PipelineID uint `json:"pipeline_id"`
	// SnapshotID refers to the immutable pipeline version to execute.
	// Required for Native; informational for external engines.
	SnapshotID uint `json:"snapshot_id"`
	// ClusterID is the Synapse-managed cluster to deploy into (if relevant).
	ClusterID uint `json:"cluster_id"`
	// Namespace is the target K8s namespace (if relevant).
	Namespace string `json:"namespace"`

	// Ref is the Git ref (branch/tag/sha) that triggered the run.
	Ref string `json:"ref,omitempty"`
	// CommitSHA is the concrete commit hash.
	CommitSHA string `json:"commit_sha,omitempty"`

	// TriggerType is one of models.TriggerType* constants.
	TriggerType string `json:"trigger_type"`
	// TriggeredByUser is the Synapse user id initiating the run.
	TriggeredByUser uint `json:"triggered_by_user"`

	// Variables are engine-specific free-form inputs (e.g. GitLab pipeline
	// variables, Jenkins build parameters).
	Variables map[string]string `json:"variables,omitempty"`
}

// TriggerResult is returned from CIEngineAdapter.Trigger.
type TriggerResult struct {
	// RunID is the Synapse-assigned run id (always populated).
	RunID string `json:"run_id"`
	// ExternalID is the engine-specific identifier (e.g. GitLab pipeline id,
	// Jenkins build number). Empty for Native.
	ExternalID string `json:"external_id,omitempty"`
	// URL, when non-empty, points to the engine's own UI for this run.
	URL string `json:"url,omitempty"`
	// QueuedAt records when the run was accepted by the engine.
	QueuedAt time.Time `json:"queued_at"`
}

// ---------------------------------------------------------------------------
// Run status
// ---------------------------------------------------------------------------

// RunPhase is the normalized lifecycle state. Engine-specific states are
// mapped to one of these values; the raw state is preserved in RunStatus.Raw.
type RunPhase string

const (
	RunPhasePending   RunPhase = "pending"
	RunPhaseRunning   RunPhase = "running"
	RunPhaseSuccess   RunPhase = "success"
	RunPhaseFailed    RunPhase = "failed"
	RunPhaseCancelled RunPhase = "cancelled"
	RunPhaseUnknown   RunPhase = "unknown"
)

// IsTerminal reports whether the run has reached a final state.
func (p RunPhase) IsTerminal() bool {
	switch p {
	case RunPhaseSuccess, RunPhaseFailed, RunPhaseCancelled:
		return true
	}
	return false
}

// RunStatus is the unified status snapshot returned by CIEngineAdapter.GetRun.
type RunStatus struct {
	RunID      string     `json:"run_id"`
	ExternalID string     `json:"external_id,omitempty"`
	Phase      RunPhase   `json:"phase"`
	Raw        string     `json:"raw,omitempty"` // engine-specific state string
	Message    string     `json:"message,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`

	// Steps lists per-step status when available.
	Steps []StepStatus `json:"steps,omitempty"`
}

// StepStatus is a per-step breakdown used by RunStatus.
type StepStatus struct {
	Name       string     `json:"name"`
	Phase      RunPhase   `json:"phase"`
	Raw        string     `json:"raw,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
}

// ---------------------------------------------------------------------------
// Artifact
// ---------------------------------------------------------------------------

// Artifact is a unified artifact descriptor returned by GetArtifacts.
type Artifact struct {
	Name      string    `json:"name"`
	Kind      string    `json:"kind"` // image / file / scan-report / ...
	URL       string    `json:"url,omitempty"`
	SizeBytes int64     `json:"size_bytes,omitempty"`
	Digest    string    `json:"digest,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
