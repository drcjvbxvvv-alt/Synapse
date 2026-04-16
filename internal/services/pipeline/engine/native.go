package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// Native adapter — thin shim over Synapse's built-in K8s Job pipeline engine.
// ---------------------------------------------------------------------------
//
// The native adapter does NOT re-implement the existing PipelineExecutor; it
// adapts it to the CIEngineAdapter contract. Execution, scheduling and log
// persistence remain in internal/services/pipeline_* (unchanged).
//
// To avoid a circular import between this package and the pipeline service
// (which depends on many other internal packages), we inject the execution
// backend via the NativeRunner interface. The CIEngineService at startup time
// supplies a concrete implementation that forwards to pipeline_service.
//
// Methods not yet supported by the native backend return ErrUnsupported so
// handlers can map to HTTP 501. This matches the "capability gating" pattern
// documented in ADR-015.

// NativeRunner abstracts the operations the Native adapter needs from the
// legacy pipeline engine. Implementations live outside this package to avoid
// import cycles.
type NativeRunner interface {
	// Trigger enqueues a new run and returns the assigned run id. Maps to
	// pipeline_service.TriggerPipeline.
	Trigger(ctx context.Context, req *TriggerRequest) (*TriggerResult, error)

	// GetRun returns current status. Maps to pipeline_service.GetRunStatus.
	GetRun(ctx context.Context, runID string) (*RunStatus, error)

	// Cancel requests cancellation. Maps to pipeline_service.CancelRun.
	Cancel(ctx context.Context, runID string) error
}

// NativeAdapter satisfies CIEngineAdapter for the built-in engine.
type NativeAdapter struct {
	runner NativeRunner
	// version is process-scoped; intentionally a plain string so build info
	// can be injected at startup (synapse.Version) without a package import.
	version string
}

// NewNativeAdapter returns an adapter. Passing a nil runner is legal — the
// adapter will still report capabilities and IsAvailable() will still return
// true — but mutating operations (Trigger / GetRun / Cancel) will fail with
// ErrUnavailable. This keeps the "UI does not blow up when wiring is
// incomplete" guarantee.
func NewNativeAdapter(runner NativeRunner, version string) *NativeAdapter {
	if strings.TrimSpace(version) == "" {
		version = "unknown"
	}
	return &NativeAdapter{runner: runner, version: version}
}

// NativeAdapter implements CIEngineAdapter.
var _ CIEngineAdapter = (*NativeAdapter)(nil)

// Type returns EngineNative.
func (a *NativeAdapter) Type() EngineType { return EngineNative }

// IsAvailable always returns true: the native engine is the in-process
// implementation; it cannot be "offline" relative to Synapse itself.
func (a *NativeAdapter) IsAvailable(context.Context) bool { return true }

// Version returns the Synapse build version provided at construction.
func (a *NativeAdapter) Version(context.Context) (string, error) {
	return a.version, nil
}

// Capabilities describes the feature set of the Synapse K8s-Job engine.
// Kept in sync with the actual capabilities of pipeline_* services. When new
// features land in M13b, update the corresponding flag here.
func (a *NativeAdapter) Capabilities() EngineCapabilities {
	return EngineCapabilities{
		SupportsDAG:          true,  // step dependencies via DependsOn
		SupportsMatrix:       true,  // pipeline_matrix
		SupportsArtifacts:    true,  // PipelineArtifact
		SupportsSecrets:      true,  // PipelineSecret (ADR-006)
		SupportsCaching:      false, // planned M13b+
		SupportsApprovals:    true,  // approval_service integration
		SupportsNotification: true,  // NotifyChannel routing
		SupportsLiveLog:      true,  // SSE streaming
	}
}

// Trigger forwards to the injected runner.
func (a *NativeAdapter) Trigger(ctx context.Context, req *TriggerRequest) (*TriggerResult, error) {
	if req == nil {
		return nil, fmt.Errorf("native.Trigger: request is nil: %w", ErrInvalidInput)
	}
	if a.runner == nil {
		return nil, fmt.Errorf("native.Trigger: runner not configured: %w", ErrUnavailable)
	}
	res, err := a.runner.Trigger(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("native.Trigger: %w", err)
	}
	return res, nil
}

// GetRun forwards to the injected runner.
func (a *NativeAdapter) GetRun(ctx context.Context, runID string) (*RunStatus, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("native.GetRun: empty run id: %w", ErrInvalidInput)
	}
	if a.runner == nil {
		return nil, fmt.Errorf("native.GetRun: runner not configured: %w", ErrUnavailable)
	}
	res, err := a.runner.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("native.GetRun %s: %w", runID, err)
	}
	return res, nil
}

// Cancel forwards to the injected runner.
func (a *NativeAdapter) Cancel(ctx context.Context, runID string) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("native.Cancel: empty run id: %w", ErrInvalidInput)
	}
	if a.runner == nil {
		return fmt.Errorf("native.Cancel: runner not configured: %w", ErrUnavailable)
	}
	if err := a.runner.Cancel(ctx, runID); err != nil {
		return fmt.Errorf("native.Cancel %s: %w", runID, err)
	}
	return nil
}

// StreamLogs is a placeholder until M13b wires the real SSE bridge. Returns
// an empty reader so callers get well-defined behavior (EOF immediately).
// The existing pipeline_log_handler continues to serve live logs directly.
func (a *NativeAdapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("native.StreamLogs: empty run id: %w", ErrInvalidInput)
	}
	// Intentionally returns an empty reader (not ErrUnsupported): the adapter
	// *is* capable of live logs, the unified SSE bridge is just not wired in
	// M18a. Callers should use the existing /api/v1/pipeline-runs/:id/logs
	// endpoint until M18b.
	_ = ctx
	_ = stepID
	return io.NopCloser(strings.NewReader("")), nil
}

// GetArtifacts returns an empty slice for M18a. The existing artifact store
// will be wired through the adapter in a later milestone.
func (a *NativeAdapter) GetArtifacts(context.Context, string) ([]*Artifact, error) {
	return []*Artifact{}, nil
}

// ---------------------------------------------------------------------------
// Registration helpers
// ---------------------------------------------------------------------------
//
// The native adapter is registered explicitly (not via init()) so tests can
// construct isolated factories. Callers typically do:
//
//	engine.RegisterNative(engine.Default(), runner, synapse.Version)

var registerNativeMu sync.Mutex

// RegisterNative registers the Native adapter builder with f. The builder
// ignores its cfg argument (the Native engine needs no connection config)
// and always returns the same adapter instance, which is safe because
// NativeAdapter has no per-call mutable state.
func RegisterNative(f *Factory, runner NativeRunner, version string) error {
	if f == nil {
		return fmt.Errorf("RegisterNative: factory is nil")
	}
	registerNativeMu.Lock()
	defer registerNativeMu.Unlock()

	adapter := NewNativeAdapter(runner, version)
	builder := func(_ *models.CIEngineConfig) (CIEngineAdapter, error) {
		return adapter, nil
	}
	return f.Register(EngineNative, builder)
}
