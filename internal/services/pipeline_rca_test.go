package services

import (
	"strings"
	"testing"
)

func TestTailLines_UnderLimit(t *testing.T) {
	content := "line1\nline2\nline3"
	got := tailLines(content, 10)
	if got != content {
		t.Errorf("expected full content, got %q", got)
	}
}

func TestTailLines_OverLimit(t *testing.T) {
	lines := make([]string, 300)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	got := tailLines(content, 200)
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != 200 {
		t.Errorf("expected 200 lines, got %d", len(gotLines))
	}
}

func TestTailLines_Empty(t *testing.T) {
	got := tailLines("", 200)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncateStr_Short(t *testing.T) {
	got := truncateStr("hello", 10)
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestTruncateStr_Long(t *testing.T) {
	got := truncateStr("hello world this is long", 10)
	if got != "hello worl..." {
		t.Errorf("expected truncated string, got %q", got)
	}
}

func TestFormatPipelineRCAContext_Basic(t *testing.T) {
	exitCode := 1
	ctx := &PipelineRCAContext{
		PipelineName: "deploy-api",
		RunID:        42,
		TriggerType:  "webhook",
		RunError:     "step build-image failed",
		RunDuration:  "2m30s",
		StepSummary:  "3/3 steps: 2 success, 1 failed",
		FailedSteps: []PipelineRCAStepDetail{
			{
				StepName:   "build-image",
				StepType:   "build-image",
				Image:      "gcr.io/kaniko-project/executor:v1.23.2",
				Command:    "/kaniko/executor --context=dir:///workspace --dockerfile=Dockerfile",
				ExitCode:   &exitCode,
				Error:      "exit code 1",
				RetryCount: 2,
				Duration:   "1m45s",
				LogTail:    "error: failed to build: COPY failed\ncommand returned non-zero exit code",
				JobStatus:  "Active: 0, Succeeded: 0, Failed: 1",
				PodStatus:  "Phase: Failed, Node: worker-1\n- Container step: ready=false, restarts=0, exitCode=1, reason=Error",
				PodEvents:  "- [Warning] BackOff: Back-off restarting failed container (count: 3)",
			},
		},
	}

	result := FormatPipelineRCAContext(ctx)

	// Verify key sections present
	checks := []string{
		"## Pipeline Run Overview",
		"Pipeline: deploy-api",
		"Run ID: 42",
		"Trigger: webhook",
		"Duration: 2m30s",
		"Run Error: step build-image failed",
		"## Failed Step 1: build-image",
		"Type: build-image",
		"Exit Code: 1",
		"Retries: 2",
		"### K8s Job Status",
		"### K8s Pod Status",
		"### Pod Events",
		"### Step Logs",
		"COPY failed",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("expected context to contain %q", check)
		}
	}
}

func TestFormatPipelineRCAContext_NoOptionalFields(t *testing.T) {
	ctx := &PipelineRCAContext{
		PipelineName: "simple-pipe",
		RunID:        1,
		TriggerType:  "manual",
		StepSummary:  "1/1 steps: 1 failed",
		FailedSteps: []PipelineRCAStepDetail{
			{
				StepName: "run-tests",
				StepType: "run",
				Error:    "test failed",
			},
		},
	}

	result := FormatPipelineRCAContext(ctx)

	// Should NOT contain optional fields when empty
	if strings.Contains(result, "Duration:") {
		t.Error("should not contain Duration when empty")
	}
	if strings.Contains(result, "Run Error:") {
		t.Error("should not contain Run Error when empty")
	}
	if strings.Contains(result, "### K8s Job Status") {
		t.Error("should not contain Job Status when empty")
	}
	if strings.Contains(result, "### Step Logs") {
		t.Error("should not contain Logs when empty")
	}

	// Should still contain the basic info
	if !strings.Contains(result, "simple-pipe") {
		t.Error("should contain pipeline name")
	}
	if !strings.Contains(result, "run-tests") {
		t.Error("should contain step name")
	}
}

func TestFormatPipelineRCAContext_MultipleFailedSteps(t *testing.T) {
	ctx := &PipelineRCAContext{
		PipelineName: "multi-step",
		RunID:        10,
		TriggerType:  "cron",
		StepSummary:  "4/4 steps: 1 success, 2 failed, 1 cancelled",
		FailedSteps: []PipelineRCAStepDetail{
			{StepName: "build", StepType: "build-image", Error: "build failed"},
			{StepName: "test", StepType: "run", Error: "test failed"},
		},
	}

	result := FormatPipelineRCAContext(ctx)

	if !strings.Contains(result, "## Failed Step 1: build") {
		t.Error("should contain first failed step")
	}
	if !strings.Contains(result, "## Failed Step 2: test") {
		t.Error("should contain second failed step")
	}
}

func TestNewPipelineRCAService(t *testing.T) {
	svc := NewPipelineRCAService(nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
