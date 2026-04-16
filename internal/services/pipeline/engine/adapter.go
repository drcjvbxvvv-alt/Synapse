package engine

import (
	"context"
	"io"
)

// CIEngineAdapter is the unified interface implemented by every CI engine
// (Native, GitLab, Jenkins, Tekton, …). See ADR-015 for design rationale.
//
// Implementation contract:
//
//   - All methods MUST be safe for concurrent use.
//   - Methods MUST respect the provided ctx for cancellation and timeout.
//   - Transient network errors are returned verbatim; callers decide retry.
//   - Domain errors (invalid input, not found) SHOULD wrap the sentinel errors
//     defined in errors.go so handlers can map them to HTTP status codes.
//   - Capabilities() MUST be idempotent and cheap (no I/O); the factory caches
//     the value.
type CIEngineAdapter interface {
	// ── Capability probing (Observer Pattern) ───────────────────────────────

	// Type returns the engine type this adapter implements.
	Type() EngineType

	// IsAvailable probes the engine for connectivity and authentication. It
	// MUST NOT return an error that blocks the UI; a failure is reported as
	// `false`. Intended timeout: 5s.
	IsAvailable(ctx context.Context) bool

	// Version returns the engine's reported version string, used for version
	// compatibility checks in the UI.
	Version(ctx context.Context) (string, error)

	// Capabilities returns the feature set supported by this adapter. The
	// returned value is process-stable and MAY be cached by callers.
	Capabilities() EngineCapabilities

	// ── Execution control ───────────────────────────────────────────────────

	// Trigger starts a new pipeline run. The adapter is responsible for
	// translating the unified TriggerRequest into its own invocation format.
	Trigger(ctx context.Context, req *TriggerRequest) (*TriggerResult, error)

	// GetRun returns the current status of a run identified by runID. The
	// runID format is adapter-specific; callers pass back whatever was
	// returned by Trigger().RunID.
	GetRun(ctx context.Context, runID string) (*RunStatus, error)

	// Cancel requests cancellation of a running pipeline. It is a no-op if
	// the run has already reached a terminal phase; callers should verify via
	// GetRun.
	Cancel(ctx context.Context, runID string) error

	// ── Logs & artifacts ────────────────────────────────────────────────────

	// StreamLogs opens a log stream for the given run/step. Passing an empty
	// stepID means "aggregated log for the whole run" where supported.
	//
	// The returned ReadCloser MUST be closed by the caller. Adapters that do
	// not support live streaming should return a reader that delivers the
	// current log snapshot and then EOF.
	StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error)

	// GetArtifacts returns the list of artifacts produced by a run. Returns
	// an empty slice (not nil, not error) when the engine has no concept of
	// artifacts or none have been produced yet.
	GetArtifacts(ctx context.Context, runID string) ([]*Artifact, error)
}
