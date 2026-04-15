package services

import (
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// Rerun-from-failed: isBeforeStep tests
// ---------------------------------------------------------------------------

func TestIsBeforeStep_Found(t *testing.T) {
	sorted := []StepDef{
		{Name: "build"},
		{Name: "test"},
		{Name: "deploy"},
	}

	s := &PipelineScheduler{}

	// "build" is before "deploy"
	if !s.isBeforeStep(sorted, "build", "deploy") {
		t.Error("expected build to be before deploy")
	}
	// "test" is before "deploy"
	if !s.isBeforeStep(sorted, "test", "deploy") {
		t.Error("expected test to be before deploy")
	}
}

func TestIsBeforeStep_SameStep(t *testing.T) {
	sorted := []StepDef{
		{Name: "build"},
		{Name: "test"},
	}
	s := &PipelineScheduler{}

	if !s.isBeforeStep(sorted, "build", "build") {
		t.Error("same step should return true")
	}
}

// ---------------------------------------------------------------------------
// allDependenciesMet tests (with skipped support for rerun)
// ---------------------------------------------------------------------------

func TestAllDependenciesMet_Success(t *testing.T) {
	s := &PipelineScheduler{}
	stepRuns := map[string]*models.StepRun{
		"a": {StepName: "a", Status: models.StepRunStatusSuccess},
		"b": {StepName: "b", Status: models.StepRunStatusSuccess},
	}
	if !s.allDependenciesMet(stepRuns, []string{"a", "b"}) {
		t.Error("expected deps met when all succeeded")
	}
}

func TestAllDependenciesMet_SkippedCountsAsMet(t *testing.T) {
	s := &PipelineScheduler{}
	stepRuns := map[string]*models.StepRun{
		"a": {StepName: "a", Status: models.StepRunStatusSkipped},
		"b": {StepName: "b", Status: models.StepRunStatusSuccess},
	}
	if !s.allDependenciesMet(stepRuns, []string{"a", "b"}) {
		t.Error("expected deps met when skipped (rerun reuse) + success")
	}
}

func TestAllDependenciesMet_FailedNotMet(t *testing.T) {
	s := &PipelineScheduler{}
	stepRuns := map[string]*models.StepRun{
		"a": {StepName: "a", Status: models.StepRunStatusFailed},
	}
	if s.allDependenciesMet(stepRuns, []string{"a"}) {
		t.Error("expected deps NOT met when dependency failed")
	}
}

func TestAllDependenciesMet_PendingNotMet(t *testing.T) {
	s := &PipelineScheduler{}
	stepRuns := map[string]*models.StepRun{
		"a": {StepName: "a", Status: models.StepRunStatusPending},
	}
	if s.allDependenciesMet(stepRuns, []string{"a"}) {
		t.Error("expected deps NOT met when dependency pending")
	}
}

func TestAllDependenciesMet_MissingNotMet(t *testing.T) {
	s := &PipelineScheduler{}
	stepRuns := map[string]*models.StepRun{}
	if s.allDependenciesMet(stepRuns, []string{"a"}) {
		t.Error("expected deps NOT met when dependency missing")
	}
}

func TestAllDependenciesMet_EmptyDeps(t *testing.T) {
	s := &PipelineScheduler{}
	if !s.allDependenciesMet(nil, nil) {
		t.Error("expected met for empty deps")
	}
}

// ---------------------------------------------------------------------------
// notifyRunCompletion nil-safe tests
// ---------------------------------------------------------------------------

func TestNotifyRunCompletion_NilNotifier(t *testing.T) {
	s := &PipelineScheduler{notifier: nil}
	run := &models.PipelineRun{Status: models.PipelineRunStatusSuccess}
	// Should not panic
	s.notifyRunCompletion(nil, run)
}

// ---------------------------------------------------------------------------
// Matrix StepDef integration
// ---------------------------------------------------------------------------

func TestStepDef_MatrixField(t *testing.T) {
	step := StepDef{
		Name: "build",
		Type: "run-script",
		Matrix: map[string][]string{
			"go_version": {"1.21", "1.22"},
			"os":         {"linux", "darwin"},
		},
		Command: "go test ./...",
	}

	if !IsMatrixStep(step) {
		t.Error("expected IsMatrixStep to return true")
	}

	combos := ExpandMatrix(step.Matrix)
	if len(combos) != 4 {
		t.Errorf("expected 4 matrix combos, got %d", len(combos))
	}
}

func TestStepDef_NoMatrix(t *testing.T) {
	step := StepDef{
		Name:    "build",
		Type:    "run-script",
		Command: "echo hello",
	}
	if IsMatrixStep(step) {
		t.Error("expected IsMatrixStep false for non-matrix step")
	}
}
