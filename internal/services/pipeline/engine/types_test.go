package engine

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestEngineType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		t    EngineType
		want bool
	}{
		{"native valid", EngineNative, true},
		{"gitlab valid", EngineGitLab, true},
		{"jenkins valid", EngineJenkins, true},
		{"tekton valid", EngineTekton, true},
		{"argo valid", EngineArgo, true},
		{"github valid", EngineGitHub, true},
		{"empty invalid", EngineType(""), false},
		{"unknown invalid", EngineType("circleci"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.t.IsValid(); got != tc.want {
				t.Fatalf("IsValid(%q) = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}

func TestEngineType_String(t *testing.T) {
	if EngineNative.String() != "native" {
		t.Fatalf("EngineNative.String() = %q, want %q", EngineNative.String(), "native")
	}
}

func TestRunPhase_IsTerminal(t *testing.T) {
	terminal := []RunPhase{RunPhaseSuccess, RunPhaseFailed, RunPhaseCancelled}
	for _, p := range terminal {
		if !p.IsTerminal() {
			t.Fatalf("expected %s to be terminal", p)
		}
	}
	nonTerminal := []RunPhase{RunPhasePending, RunPhaseRunning, RunPhaseUnknown}
	for _, p := range nonTerminal {
		if p.IsTerminal() {
			t.Fatalf("expected %s to be non-terminal", p)
		}
	}
}

func TestCapabilities_JSONRoundTrip(t *testing.T) {
	cap := EngineCapabilities{
		SupportsDAG:          true,
		SupportsMatrix:       false,
		SupportsArtifacts:    true,
		SupportsSecrets:      true,
		SupportsCaching:      false,
		SupportsApprovals:    true,
		SupportsNotification: true,
		SupportsLiveLog:      true,
	}
	buf, err := json.Marshal(cap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back EngineCapabilities
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back != cap {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", back, cap)
	}
	// JSON keys must be snake_case (frontend contract).
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"supports_dag", "supports_matrix", "supports_artifacts",
		"supports_secrets", "supports_caching", "supports_approvals",
		"supports_notification", "supports_live_log",
	} {
		if _, ok := m[key]; !ok {
			t.Fatalf("expected key %q in JSON, got %v", key, m)
		}
	}
}

func TestTriggerRequest_Variables(t *testing.T) {
	req := &TriggerRequest{
		PipelineID:      42,
		SnapshotID:      7,
		ClusterID:       1,
		Namespace:       "default",
		Ref:             "refs/heads/main",
		CommitSHA:       "abc123",
		TriggerType:     "manual",
		TriggeredByUser: 100,
		Variables:       map[string]string{"ENV": "staging"},
	}
	buf, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var back TriggerRequest
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatal(err)
	}
	if back.Variables["ENV"] != "staging" {
		t.Fatalf("variable lost in round-trip")
	}
}

func TestRunStatus_Zero(t *testing.T) {
	// Zero-value RunStatus is serializable and Phase defaults to empty (not
	// "unknown"); callers must set Phase explicitly.
	var rs RunStatus
	buf, err := json.Marshal(rs)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) == "" {
		t.Fatal("expected non-empty JSON")
	}
}

func TestArtifact_TimeField(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	a := &Artifact{
		Name:      "app.jar",
		Kind:      "file",
		SizeBytes: 1024,
		CreatedAt: now,
	}
	buf, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var back Artifact
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatal(err)
	}
	if !back.CreatedAt.Equal(now) {
		t.Fatalf("time mismatch: got %v want %v", back.CreatedAt, now)
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	all := []error{
		ErrNotFound, ErrInvalidInput, ErrUnauthorized,
		ErrUnavailable, ErrUnsupported, ErrAlreadyTerminal,
	}
	// Each sentinel should only match itself via errors.Is.
	for i, a := range all {
		for j, b := range all {
			if i == j {
				if !errors.Is(a, b) {
					t.Fatalf("errors.Is(%v, itself) = false", a)
				}
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("errors.Is(%v, %v) should be false", a, b)
			}
		}
	}
}

func TestSentinelErrors_WrapUnwrap(t *testing.T) {
	// Adapter implementations wrap sentinels with fmt.Errorf(...%w...); handlers
	// must still be able to detect the original sentinel.
	wrapped := errForTest("fetch run 42: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Fatalf("wrapped error should still match ErrNotFound via errors.Is")
	}
	if errors.Is(wrapped, ErrUnauthorized) {
		t.Fatalf("wrapped error should not match unrelated sentinel")
	}
}

// errForTest is a tiny helper that mirrors what adapters will do in practice.
func errForTest(format string, a ...any) error {
	// Using fmt.Errorf via shim keeps the test self-contained and exercises
	// the %w wrapping contract.
	return fmtErrorf(format, a...)
}
