package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetPipelineSchemaV1_NotEmpty(t *testing.T) {
	schema := GetPipelineSchemaV1()
	if len(schema) == 0 {
		t.Fatal("expected non-empty schema")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	// Verify key fields
	if parsed["$schema"] == nil {
		t.Error("schema missing $schema field")
	}
	if parsed["$defs"] == nil {
		t.Error("schema missing $defs field")
	}
}

func TestGetPipelineSchemaV1_HasStepTypes(t *testing.T) {
	schema := GetPipelineSchemaV1()
	var parsed map[string]interface{}
	json.Unmarshal(schema, &parsed)

	defs := parsed["$defs"].(map[string]interface{})
	step := defs["Step"].(map[string]interface{})
	props := step["properties"].(map[string]interface{})
	typeField := props["type"].(map[string]interface{})
	enumList := typeField["enum"].([]interface{})

	// Verify all registered step types are in the schema
	for typeName := range stepTypeRegistry {
		found := false
		for _, e := range enumList {
			if e.(string) == typeName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("step type %q registered but not in schema enum", typeName)
		}
	}
}

func TestValidatePipelineYAML_Valid(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata: PipelineYAMLMetadata{
			Name:      "my-pipeline",
			Cluster:   "prod",
			Namespace: "default",
		},
		Spec: PipelineYAMLSpec{
			Steps: []PipelineYAMLStep{
				{Name: "build", Type: "build-image"},
				{Name: "deploy", Type: "deploy", DependsOn: []string{"build"}},
			},
		},
	}
	errs := ValidatePipelineYAML(doc)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidatePipelineYAML_MissingAPIVersion(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "wrong/v2",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec:       PipelineYAMLSpec{Steps: []PipelineYAMLStep{{Name: "a", Type: "shell"}}},
	}
	errs := ValidatePipelineYAML(doc)
	if len(errs) == 0 {
		t.Error("expected error for wrong apiVersion")
	}
}

func TestValidatePipelineYAML_MissingMetadata(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{},
		Spec:       PipelineYAMLSpec{Steps: []PipelineYAMLStep{{Name: "a", Type: "shell"}}},
	}
	errs := ValidatePipelineYAML(doc)
	if len(errs) != 3 { // name, cluster, namespace
		t.Errorf("expected 3 metadata errors, got %d: %v", len(errs), errs)
	}
}

func TestValidatePipelineYAML_NoSteps(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec:       PipelineYAMLSpec{Steps: nil},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if e == "spec.steps must contain at least one step" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'must contain at least one step' error")
	}
}

func TestValidatePipelineYAML_DuplicateStepNames(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "build", Type: "build-image"},
			{Name: "build", Type: "deploy"},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"duplicated") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate name error, got %v", errs)
	}
}

func TestValidatePipelineYAML_InvalidStepType(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "build", Type: "nonexistent-type"},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"not a valid step type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid step type error, got %v", errs)
	}
}

func TestValidatePipelineYAML_SelfDependency(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "build", Type: "build-image", DependsOn: []string{"build"}},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"depend on itself") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected self-dependency error, got %v", errs)
	}
}

func TestValidatePipelineYAML_MissingDependency(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "deploy", Type: "deploy", DependsOn: []string{"build"}},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"does not exist") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing dependency error, got %v", errs)
	}
}

func TestValidatePipelineYAML_CycleDetection(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "a", Type: "shell", DependsOn: []string{"b"}},
			{Name: "b", Type: "shell", DependsOn: []string{"c"}},
			{Name: "c", Type: "shell", DependsOn: []string{"a"}},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"cycle detected") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cycle error, got %v", errs)
	}
}

func TestValidatePipelineYAML_NoCycle(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "a", Type: "shell"},
			{Name: "b", Type: "shell", DependsOn: []string{"a"}},
			{Name: "c", Type: "shell", DependsOn: []string{"a", "b"}},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	for _, e := range errs {
		if strings.Contains(e,"cycle") {
			t.Errorf("unexpected cycle error: %s", e)
		}
	}
}

func TestValidatePipelineYAML_InvalidConcurrencyPolicy(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{
			Concurrency: &PipelineYAMLConcurrency{Policy: "invalid"},
			Steps:       []PipelineYAMLStep{{Name: "a", Type: "shell"}},
		},
	}
	errs := ValidatePipelineYAML(doc)
	if len(errs) == 0 {
		t.Error("expected error for invalid concurrency policy")
	}
}

func TestValidatePipelineYAML_InvalidOnFailure(t *testing.T) {
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{Steps: []PipelineYAMLStep{
			{Name: "a", Type: "shell", OnFailure: "explode"},
		}},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"on_failure") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected on_failure error, got %v", errs)
	}
}

func TestValidatePipelineYAMLFromJSON_Valid(t *testing.T) {
	data := []byte(`{
		"apiVersion": "synapse.io/v1",
		"kind": "Pipeline",
		"metadata": {"name": "test", "cluster": "c", "namespace": "ns"},
		"spec": {"steps": [{"name": "a", "type": "shell"}]}
	}`)
	errs, err := ValidatePipelineYAMLFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got %v", errs)
	}
}

func TestValidatePipelineYAMLFromJSON_InvalidJSON(t *testing.T) {
	_, err := ValidatePipelineYAMLFromJSON([]byte("not-json"))
	if err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestDetectStepCycle_Diamond(t *testing.T) {
	// Diamond graph: A → B, A → C, B → D, C → D (no cycle)
	steps := []PipelineYAMLStep{
		{Name: "a", Type: "shell"},
		{Name: "b", Type: "shell", DependsOn: []string{"a"}},
		{Name: "c", Type: "shell", DependsOn: []string{"a"}},
		{Name: "d", Type: "shell", DependsOn: []string{"b", "c"}},
	}
	if msg := detectStepCycle(steps); msg != "" {
		t.Errorf("diamond graph should not have cycle, got: %s", msg)
	}
}

func TestDetectStepCycle_WithCycle(t *testing.T) {
	steps := []PipelineYAMLStep{
		{Name: "a", Type: "shell", DependsOn: []string{"c"}},
		{Name: "b", Type: "shell", DependsOn: []string{"a"}},
		{Name: "c", Type: "shell", DependsOn: []string{"b"}},
	}
	if msg := detectStepCycle(steps); msg == "" {
		t.Error("expected cycle to be detected")
	}
}

func TestValidatePipelineYAML_MaxConcurrentRunsOutOfRange(t *testing.T) {
	v := 100
	doc := &PipelineYAMLDoc{
		APIVersion: "synapse.io/v1",
		Kind:       "Pipeline",
		Metadata:   PipelineYAMLMetadata{Name: "x", Cluster: "c", Namespace: "ns"},
		Spec: PipelineYAMLSpec{
			MaxConcurrentRuns: &v,
			Steps:             []PipelineYAMLStep{{Name: "a", Type: "shell"}},
		},
	}
	errs := ValidatePipelineYAML(doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e,"max_concurrent_runs") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected max_concurrent_runs error, got %v", errs)
	}
}
