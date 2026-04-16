package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// fmtErrorf is a local alias; lives here to avoid importing fmt in *_test.go
// files that focus purely on types. Keeping it in the adapter test file makes
// it trivially available to types_test.go via the shared package scope.
func fmtErrorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

// stubAdapter is a test double that lets us assert the interface contract
// without pulling in a real engine implementation.
type stubAdapter struct {
	typ    EngineType
	caps   EngineCapabilities
	avail  bool
	trigFn func(ctx context.Context, req *TriggerRequest) (*TriggerResult, error)
	getFn  func(ctx context.Context, runID string) (*RunStatus, error)
}

func (s *stubAdapter) Type() EngineType                              { return s.typ }
func (s *stubAdapter) IsAvailable(context.Context) bool              { return s.avail }
func (s *stubAdapter) Version(context.Context) (string, error)       { return "test-1.0", nil }
func (s *stubAdapter) Capabilities() EngineCapabilities              { return s.caps }
func (s *stubAdapter) Cancel(context.Context, string) error          { return nil }
func (s *stubAdapter) GetArtifacts(context.Context, string) ([]*Artifact, error) {
	return []*Artifact{}, nil
}
func (s *stubAdapter) StreamLogs(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (s *stubAdapter) Trigger(ctx context.Context, req *TriggerRequest) (*TriggerResult, error) {
	if s.trigFn != nil {
		return s.trigFn(ctx, req)
	}
	return &TriggerResult{RunID: "r-1", QueuedAt: time.Now()}, nil
}
func (s *stubAdapter) GetRun(ctx context.Context, runID string) (*RunStatus, error) {
	if s.getFn != nil {
		return s.getFn(ctx, runID)
	}
	return &RunStatus{RunID: runID, Phase: RunPhasePending}, nil
}

// Compile-time assertion: stubAdapter satisfies CIEngineAdapter. The test will
// fail to build if the interface is accidentally changed without updating the
// test double.
var _ CIEngineAdapter = (*stubAdapter)(nil)

func TestAdapter_Contract_BasicCalls(t *testing.T) {
	ctx := context.Background()
	a := &stubAdapter{
		typ:   EngineNative,
		avail: true,
		caps:  EngineCapabilities{SupportsDAG: true, SupportsSecrets: true},
	}

	if a.Type() != EngineNative {
		t.Fatalf("Type() wrong")
	}
	if !a.IsAvailable(ctx) {
		t.Fatalf("IsAvailable() expected true")
	}
	v, err := a.Version(ctx)
	if err != nil {
		t.Fatalf("Version() err: %v", err)
	}
	if v == "" {
		t.Fatalf("Version() empty")
	}
	caps := a.Capabilities()
	if !caps.SupportsDAG {
		t.Fatalf("SupportsDAG expected true")
	}
}

func TestAdapter_Contract_Trigger(t *testing.T) {
	ctx := context.Background()
	called := false
	a := &stubAdapter{
		typ: EngineNative,
		trigFn: func(ctx context.Context, req *TriggerRequest) (*TriggerResult, error) {
			called = true
			if req.PipelineID != 99 {
				t.Fatalf("pipeline id not propagated")
			}
			return &TriggerResult{RunID: "r-99", QueuedAt: time.Now()}, nil
		},
	}
	res, err := a.Trigger(ctx, &TriggerRequest{PipelineID: 99})
	if err != nil {
		t.Fatalf("Trigger err: %v", err)
	}
	if !called {
		t.Fatalf("stub not invoked")
	}
	if res.RunID != "r-99" {
		t.Fatalf("RunID mismatch")
	}
}

func TestAdapter_Contract_ContextCancel(t *testing.T) {
	// Adapters must honor ctx cancellation; we simulate that here to lock in
	// the convention that tests should always use a context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := &stubAdapter{
		typ: EngineNative,
		trigFn: func(ctx context.Context, req *TriggerRequest) (*TriggerResult, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &TriggerResult{RunID: "r-1"}, nil
		},
	}
	_, err := a.Trigger(ctx, &TriggerRequest{PipelineID: 1})
	if err == nil {
		t.Fatalf("expected error from cancelled ctx")
	}
}
