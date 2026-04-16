package models

import (
	"encoding/json"
	"testing"
)

// TestTriggerTypeRollback verifies the constant value matches the DB/API contract.
func TestTriggerTypeRollback(t *testing.T) {
	if TriggerTypeRollback != "rollback" {
		t.Fatalf("TriggerTypeRollback = %q, want %q", TriggerTypeRollback, "rollback")
	}
}

// TestPipelineRun_RollbackOfRunID_OmitWhenNil verifies rollback_of_run_id is
// omitted from JSON when nil (normal / rerun runs should not expose the field).
func TestPipelineRun_RollbackOfRunID_OmitWhenNil(t *testing.T) {
	run := &PipelineRun{
		ID:          1,
		PipelineID:  2,
		TriggerType: TriggerTypeManual,
	}
	buf, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := m["rollback_of_run_id"]; present {
		t.Fatalf("rollback_of_run_id should be omitted when nil, got %v", m["rollback_of_run_id"])
	}
}

// TestPipelineRun_RollbackOfRunID_PresentWhenSet verifies rollback_of_run_id
// serialises correctly when a rollback run is created.
func TestPipelineRun_RollbackOfRunID_PresentWhenSet(t *testing.T) {
	sourceRunID := uint(42)
	run := &PipelineRun{
		ID:              10,
		PipelineID:      2,
		TriggerType:     TriggerTypeRollback,
		RollbackOfRunID: &sourceRunID,
	}
	buf, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := m["rollback_of_run_id"].(float64)
	if !ok {
		t.Fatalf("rollback_of_run_id missing or wrong type, got %v", m["rollback_of_run_id"])
	}
	if uint(got) != sourceRunID {
		t.Fatalf("rollback_of_run_id = %v, want %d", got, sourceRunID)
	}
	if m["trigger_type"] != TriggerTypeRollback {
		t.Fatalf("trigger_type = %v, want %q", m["trigger_type"], TriggerTypeRollback)
	}
}
