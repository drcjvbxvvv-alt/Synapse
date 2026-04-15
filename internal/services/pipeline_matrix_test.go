package services

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ExpandMatrix tests
// ---------------------------------------------------------------------------

func TestExpandMatrix_Nil(t *testing.T) {
	combos := ExpandMatrix(nil)
	if combos != nil {
		t.Errorf("expected nil for nil matrix, got %v", combos)
	}
}

func TestExpandMatrix_Empty(t *testing.T) {
	combos := ExpandMatrix(map[string][]string{})
	if combos != nil {
		t.Errorf("expected nil for empty matrix, got %v", combos)
	}
}

func TestExpandMatrix_SingleDimension(t *testing.T) {
	matrix := map[string][]string{
		"go_version": {"1.21", "1.22", "1.23"},
	}
	combos := ExpandMatrix(matrix)
	if len(combos) != 3 {
		t.Fatalf("expected 3 combos, got %d", len(combos))
	}

	versions := make([]string, len(combos))
	for i, c := range combos {
		versions[i] = c.Values["go_version"]
	}
	sort.Strings(versions)
	if versions[0] != "1.21" || versions[1] != "1.22" || versions[2] != "1.23" {
		t.Errorf("unexpected versions: %v", versions)
	}
}

func TestExpandMatrix_TwoDimensions(t *testing.T) {
	matrix := map[string][]string{
		"go_version": {"1.21", "1.22"},
		"os":         {"linux", "darwin"},
	}
	combos := ExpandMatrix(matrix)
	// 2 x 2 = 4 combinations
	if len(combos) != 4 {
		t.Fatalf("expected 4 combos, got %d", len(combos))
	}

	// Verify all combinations exist
	found := make(map[string]bool)
	for _, c := range combos {
		key := c.Values["go_version"] + "-" + c.Values["os"]
		found[key] = true
	}
	expected := []string{"1.21-linux", "1.21-darwin", "1.22-linux", "1.22-darwin"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("missing combination: %s", e)
		}
	}
}

func TestExpandMatrix_ThreeDimensions(t *testing.T) {
	matrix := map[string][]string{
		"a": {"1", "2"},
		"b": {"x", "y"},
		"c": {"p", "q", "r"},
	}
	combos := ExpandMatrix(matrix)
	// 2 x 2 x 3 = 12
	if len(combos) != 12 {
		t.Fatalf("expected 12 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_Labels(t *testing.T) {
	matrix := map[string][]string{
		"go_version": {"1.21"},
		"os":         {"linux"},
	}
	combos := ExpandMatrix(matrix)
	if len(combos) != 1 {
		t.Fatalf("expected 1 combo, got %d", len(combos))
	}
	// Label should contain both values joined by '-'
	label := combos[0].Label
	if !strings.Contains(label, "1.21") || !strings.Contains(label, "linux") {
		t.Errorf("label should contain both values, got: %s", label)
	}
}

func TestExpandMatrix_EmptyValues(t *testing.T) {
	matrix := map[string][]string{
		"a": {"1", "2"},
		"b": {}, // empty values
	}
	combos := ExpandMatrix(matrix)
	// Empty dimension is skipped, so only 2 combos
	if len(combos) != 2 {
		t.Fatalf("expected 2 combos (empty dim skipped), got %d", len(combos))
	}
}

func TestExpandMatrix_DeterministicOrder(t *testing.T) {
	matrix := map[string][]string{
		"z": {"1"},
		"a": {"2"},
		"m": {"3"},
	}
	combos1 := ExpandMatrix(matrix)
	combos2 := ExpandMatrix(matrix)

	if combos1[0].Label != combos2[0].Label {
		t.Errorf("expected deterministic order, got %s vs %s", combos1[0].Label, combos2[0].Label)
	}
}

// ---------------------------------------------------------------------------
// IsMatrixStep tests
// ---------------------------------------------------------------------------

func TestIsMatrixStep(t *testing.T) {
	if IsMatrixStep(StepDef{Name: "test"}) {
		t.Error("expected false for non-matrix step")
	}
	if !IsMatrixStep(StepDef{Name: "test", Matrix: map[string][]string{"a": {"1"}}}) {
		t.Error("expected true for matrix step")
	}
}

// ---------------------------------------------------------------------------
// injectMatrixEnv tests
// ---------------------------------------------------------------------------

func TestInjectMatrixEnv_Empty(t *testing.T) {
	result := injectMatrixEnv("", map[string]string{"go_version": "1.22"})
	var cfg StepConfig
	if err := json.Unmarshal([]byte(result), &cfg); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if cfg.Env["MATRIX_GO_VERSION"] != "1.22" {
		t.Errorf("expected MATRIX_GO_VERSION=1.22, got %q", cfg.Env["MATRIX_GO_VERSION"])
	}
}

func TestInjectMatrixEnv_PreservesExisting(t *testing.T) {
	existing := StepConfig{
		Env: map[string]string{"EXISTING": "value"},
	}
	data, _ := json.Marshal(existing)

	result := injectMatrixEnv(string(data), map[string]string{"os": "linux"})
	var cfg StepConfig
	if err := json.Unmarshal([]byte(result), &cfg); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if cfg.Env["EXISTING"] != "value" {
		t.Errorf("expected existing env preserved, got %q", cfg.Env["EXISTING"])
	}
	if cfg.Env["MATRIX_OS"] != "linux" {
		t.Errorf("expected MATRIX_OS=linux, got %q", cfg.Env["MATRIX_OS"])
	}
}

func TestInjectMatrixEnv_HyphenToUnderscore(t *testing.T) {
	result := injectMatrixEnv("", map[string]string{"go-version": "1.22"})
	var cfg StepConfig
	if err := json.Unmarshal([]byte(result), &cfg); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if cfg.Env["MATRIX_GO_VERSION"] != "1.22" {
		t.Errorf("expected MATRIX_GO_VERSION=1.22, got env: %v", cfg.Env)
	}
}
