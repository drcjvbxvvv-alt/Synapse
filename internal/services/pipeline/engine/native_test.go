package engine

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// fakeRunner is a test double for NativeRunner that records invocations and
// lets individual tests control the return values.
type fakeRunner struct {
	triggerCalls int
	lastTrigger  *TriggerRequest
	triggerRes   *TriggerResult
	triggerErr   error

	getRunCalls int
	lastRunID   string
	getRunRes   *RunStatus
	getRunErr   error

	cancelCalls int
	cancelRunID string
	cancelErr   error
}

func (f *fakeRunner) Trigger(_ context.Context, req *TriggerRequest) (*TriggerResult, error) {
	f.triggerCalls++
	f.lastTrigger = req
	if f.triggerErr != nil {
		return nil, f.triggerErr
	}
	if f.triggerRes != nil {
		return f.triggerRes, nil
	}
	return &TriggerResult{RunID: "r-0", QueuedAt: time.Now()}, nil
}

func (f *fakeRunner) GetRun(_ context.Context, runID string) (*RunStatus, error) {
	f.getRunCalls++
	f.lastRunID = runID
	if f.getRunErr != nil {
		return nil, f.getRunErr
	}
	if f.getRunRes != nil {
		return f.getRunRes, nil
	}
	return &RunStatus{RunID: runID, Phase: RunPhaseRunning}, nil
}

func (f *fakeRunner) Cancel(_ context.Context, runID string) error {
	f.cancelCalls++
	f.cancelRunID = runID
	return f.cancelErr
}

func TestNativeAdapter_InterfaceSatisfaction(t *testing.T) {
	var _ CIEngineAdapter = (*NativeAdapter)(nil)
}

func TestNativeAdapter_TypeAndAvailability(t *testing.T) {
	a := NewNativeAdapter(&fakeRunner{}, "v1.2.3")
	if a.Type() != EngineNative {
		t.Fatalf("Type = %q, want native", a.Type())
	}
	if !a.IsAvailable(context.Background()) {
		t.Fatalf("IsAvailable must be true for native engine")
	}
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version err: %v", err)
	}
	if v != "v1.2.3" {
		t.Fatalf("Version = %q, want v1.2.3", v)
	}
}

func TestNativeAdapter_Version_DefaultsWhenEmpty(t *testing.T) {
	a := NewNativeAdapter(&fakeRunner{}, "")
	v, _ := a.Version(context.Background())
	if v != "unknown" {
		t.Fatalf("Version = %q, want 'unknown'", v)
	}
}

func TestNativeAdapter_Capabilities(t *testing.T) {
	a := NewNativeAdapter(&fakeRunner{}, "v1")
	caps := a.Capabilities()
	// Lock in capability contract for the Native engine. Changing any of
	// these flags represents a user-visible contract change — tests must
	// update in lock-step with the adapter.
	want := EngineCapabilities{
		SupportsDAG:          true,
		SupportsMatrix:       true,
		SupportsArtifacts:    true,
		SupportsSecrets:      true,
		SupportsCaching:      false,
		SupportsApprovals:    true,
		SupportsNotification: true,
		SupportsLiveLog:      true,
	}
	if caps != want {
		t.Fatalf("Capabilities mismatch\n got: %+v\nwant: %+v", caps, want)
	}
}

func TestNativeAdapter_Trigger_DelegatesToRunner(t *testing.T) {
	r := &fakeRunner{
		triggerRes: &TriggerResult{RunID: "r-42", ExternalID: "", QueuedAt: time.Now()},
	}
	a := NewNativeAdapter(r, "v1")
	res, err := a.Trigger(context.Background(), &TriggerRequest{
		PipelineID: 42, SnapshotID: 7, ClusterID: 1, Namespace: "default",
	})
	if err != nil {
		t.Fatalf("Trigger err: %v", err)
	}
	if r.triggerCalls != 1 {
		t.Fatalf("runner calls = %d, want 1", r.triggerCalls)
	}
	if r.lastTrigger.PipelineID != 42 {
		t.Fatalf("PipelineID not propagated: got %d", r.lastTrigger.PipelineID)
	}
	if res.RunID != "r-42" {
		t.Fatalf("RunID = %q, want r-42", res.RunID)
	}
}

func TestNativeAdapter_Trigger_NilRequest(t *testing.T) {
	a := NewNativeAdapter(&fakeRunner{}, "v1")
	_, err := a.Trigger(context.Background(), nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNativeAdapter_Trigger_NilRunner(t *testing.T) {
	a := NewNativeAdapter(nil, "v1")
	_, err := a.Trigger(context.Background(), &TriggerRequest{PipelineID: 1})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestNativeAdapter_Trigger_RunnerError(t *testing.T) {
	r := &fakeRunner{triggerErr: errors.New("boom")}
	a := NewNativeAdapter(r, "v1")
	_, err := a.Trigger(context.Background(), &TriggerRequest{PipelineID: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, r.triggerErr) {
		t.Fatalf("expected wrapped runner error, got %v", err)
	}
}

func TestNativeAdapter_GetRun_Delegates(t *testing.T) {
	r := &fakeRunner{getRunRes: &RunStatus{RunID: "r-1", Phase: RunPhaseSuccess}}
	a := NewNativeAdapter(r, "v1")
	res, err := a.GetRun(context.Background(), "r-1")
	if err != nil {
		t.Fatalf("GetRun err: %v", err)
	}
	if res.Phase != RunPhaseSuccess {
		t.Fatalf("phase = %q, want success", res.Phase)
	}
}

func TestNativeAdapter_GetRun_EmptyRunID(t *testing.T) {
	a := NewNativeAdapter(&fakeRunner{}, "v1")
	_, err := a.GetRun(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNativeAdapter_Cancel_Delegates(t *testing.T) {
	r := &fakeRunner{}
	a := NewNativeAdapter(r, "v1")
	if err := a.Cancel(context.Background(), "r-99"); err != nil {
		t.Fatalf("Cancel err: %v", err)
	}
	if r.cancelCalls != 1 {
		t.Fatalf("cancel not called")
	}
	if r.cancelRunID != "r-99" {
		t.Fatalf("runID = %q, want r-99", r.cancelRunID)
	}
}

func TestNativeAdapter_Cancel_RunnerError(t *testing.T) {
	r := &fakeRunner{cancelErr: errors.New("kaboom")}
	a := NewNativeAdapter(r, "v1")
	err := a.Cancel(context.Background(), "r-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, r.cancelErr) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestNativeAdapter_StreamLogs_ReturnsReaderEvenWithoutRunner(t *testing.T) {
	// The M18a native StreamLogs is a placeholder (returns empty reader).
	// Callers must still get a non-nil ReadCloser they can safely Close().
	a := NewNativeAdapter(nil, "v1")
	rc, err := a.StreamLogs(context.Background(), "r-1", "")
	if err != nil {
		t.Fatalf("StreamLogs err: %v", err)
	}
	if rc == nil {
		t.Fatal("ReadCloser is nil")
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(buf) != 0 {
		t.Fatalf("placeholder should return empty body, got %q", buf)
	}
}

func TestNativeAdapter_StreamLogs_EmptyRunID(t *testing.T) {
	a := NewNativeAdapter(nil, "v1")
	_, err := a.StreamLogs(context.Background(), "", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNativeAdapter_GetArtifacts_EmptyByDefault(t *testing.T) {
	a := NewNativeAdapter(&fakeRunner{}, "v1")
	arts, err := a.GetArtifacts(context.Background(), "r-1")
	if err != nil {
		t.Fatalf("GetArtifacts err: %v", err)
	}
	if arts == nil {
		t.Fatalf("must return non-nil empty slice (handler contract)")
	}
	if len(arts) != 0 {
		t.Fatalf("expected 0 artifacts, got %d", len(arts))
	}
}

func TestRegisterNative_WiresFactory(t *testing.T) {
	f := NewFactory()
	r := &fakeRunner{}
	if err := RegisterNative(f, r, "v1"); err != nil {
		t.Fatalf("RegisterNative: %v", err)
	}
	a, err := f.BuildNative()
	if err != nil {
		t.Fatalf("BuildNative: %v", err)
	}
	if a.Type() != EngineNative {
		t.Fatalf("Type = %q", a.Type())
	}
	// Second registration should fail (duplicate).
	if err := RegisterNative(f, r, "v1"); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestRegisterNative_NilFactory(t *testing.T) {
	if err := RegisterNative(nil, &fakeRunner{}, "v1"); err == nil {
		t.Fatalf("expected error for nil factory")
	}
}
