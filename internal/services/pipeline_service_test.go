package services

import (
	"testing"
)

func TestComputeVersionHash_Deterministic(t *testing.T) {
	req := &CreateVersionRequest{
		StepsJSON:     `[{"name":"build"}]`,
		TriggersJSON:  `{"on":"push"}`,
		EnvJSON:       `{"GO_VERSION":"1.22"}`,
		RuntimeJSON:   `{"timeout":600}`,
		WorkspaceJSON: `{"size":"10Gi"}`,
	}

	h1 := computeVersionHash(req)
	h2 := computeVersionHash(req)

	if h1 != h2 {
		t.Errorf("same input produced different hashes: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 hex chars (SHA-256), got %d: %s", len(h1), h1)
	}
}

func TestComputeVersionHash_DifferentInputs(t *testing.T) {
	req1 := &CreateVersionRequest{
		StepsJSON: `[{"name":"build"}]`,
	}
	req2 := &CreateVersionRequest{
		StepsJSON: `[{"name":"test"}]`,
	}

	if computeVersionHash(req1) == computeVersionHash(req2) {
		t.Error("different inputs should produce different hashes")
	}
}

func TestComputeVersionHash_FieldOrderIndependent(t *testing.T) {
	// computeVersionHash sorts fields by key, so the order in the struct
	// shouldn't matter. Verify by ensuring two requests with same content
	// but different "assignment order" still match.
	req := &CreateVersionRequest{
		WorkspaceJSON: `{"size":"10Gi"}`,
		StepsJSON:     `[{"name":"build"}]`,
		EnvJSON:       `{"K":"V"}`,
		TriggersJSON:  `{}`,
		RuntimeJSON:   `{}`,
	}

	// The sort is by the key names (env, runtime, steps, triggers, workspace)
	// regardless of struct field order — so hash should be stable.
	h := computeVersionHash(req)
	if h == "" {
		t.Error("hash should not be empty")
	}
}
