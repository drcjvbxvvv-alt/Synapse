package models

import (
	"encoding/json"
	"testing"
)

func TestPipeline_EngineFieldsJSON(t *testing.T) {
	id := uint(5)
	p := &Pipeline{
		ID:             1,
		Name:           "demo",
		EngineType:     "gitlab",
		EngineConfigID: &id,
	}
	buf, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Keys use snake_case per frontend contract.
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatal(err)
	}
	if m["engine_type"] != "gitlab" {
		t.Fatalf("engine_type = %v, want gitlab", m["engine_type"])
	}
	if got, ok := m["engine_config_id"].(float64); !ok || got != 5 {
		t.Fatalf("engine_config_id = %v, want 5", m["engine_config_id"])
	}
}

func TestPipeline_EngineConfigID_OmitEmptyWhenNil(t *testing.T) {
	p := &Pipeline{ID: 1, Name: "demo", EngineType: "native"}
	buf, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatal(err)
	}
	if _, present := m["engine_config_id"]; present {
		t.Fatalf("engine_config_id should be omitted when nil, got %v", m["engine_config_id"])
	}
}

func TestPipeline_EngineTypeDefaultsThroughGormTag(t *testing.T) {
	// The GORM default ('native') only kicks in at INSERT time; verify at
	// least that an explicitly unset field serializes as the empty string
	// (not some surprise value from a hook).
	p := &Pipeline{ID: 1, Name: "demo"}
	buf, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatal(err)
	}
	if m["engine_type"] != "" {
		t.Fatalf("engine_type without explicit value = %v, want empty", m["engine_type"])
	}
}
