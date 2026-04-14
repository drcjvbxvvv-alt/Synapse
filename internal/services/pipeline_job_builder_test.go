package services

import (
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

func TestBuildJob_EnvFromSecretRef(t *testing.T) {
	builder := NewJobBuilder()

	input := &BuildJobInput{
		Run: &models.PipelineRun{
			ID: 1,
		},
		StepRun: &models.StepRun{
			ID:       10,
			StepName: "build",
			StepType: "build-image",
			Image:    "docker.io/library/golang:1.22",
			Command:  "go build .",
		},
		Namespace:  "default",
		SecretName: "pr-1-step-10-secrets",
	}

	job, err := builder.BuildJob(input)
	if err != nil {
		t.Fatalf("BuildJob failed: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]

	// Verify envFrom contains the secret ref
	found := false
	for _, ef := range container.EnvFrom {
		if ef.SecretRef != nil && ef.SecretRef.Name == "pr-1-step-10-secrets" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected envFrom to contain SecretRef 'pr-1-step-10-secrets'")
	}
}

func TestBuildJob_NoSecretName_NoEnvFrom(t *testing.T) {
	builder := NewJobBuilder()

	input := &BuildJobInput{
		Run: &models.PipelineRun{
			ID: 2,
		},
		StepRun: &models.StepRun{
			ID:       20,
			StepName: "test",
			StepType: "test",
			Image:    "golang:1.22",
		},
		Namespace: "default",
		// No SecretName
	}

	job, err := builder.BuildJob(input)
	if err != nil {
		t.Fatalf("BuildJob failed: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	if len(container.EnvFrom) != 0 {
		t.Errorf("expected no envFrom, got %d", len(container.EnvFrom))
	}
}

func TestBuildJob_PodSecurityBaseline(t *testing.T) {
	builder := NewJobBuilder()

	input := &BuildJobInput{
		Run:     &models.PipelineRun{ID: 3},
		StepRun: &models.StepRun{ID: 30, StepName: "lint", StepType: "lint", Image: "golangci-lint:latest"},
		Namespace: "ci",
	}

	job, err := builder.BuildJob(input)
	if err != nil {
		t.Fatalf("BuildJob failed: %v", err)
	}

	podSec := job.Spec.Template.Spec.SecurityContext
	if podSec == nil {
		t.Fatal("expected pod security context")
	}
	if podSec.RunAsNonRoot == nil || !*podSec.RunAsNonRoot {
		t.Error("expected runAsNonRoot = true")
	}
	if podSec.SeccompProfile == nil || podSec.SeccompProfile.Type != "RuntimeDefault" {
		t.Error("expected seccomp RuntimeDefault")
	}

	cSec := job.Spec.Template.Spec.Containers[0].SecurityContext
	if cSec == nil {
		t.Fatal("expected container security context")
	}
	if cSec.AllowPrivilegeEscalation == nil || *cSec.AllowPrivilegeEscalation {
		t.Error("expected allowPrivilegeEscalation = false")
	}
	if cSec.Capabilities == nil || len(cSec.Capabilities.Drop) == 0 {
		t.Error("expected capabilities drop ALL")
	}
}

func TestBuildJob_Labels(t *testing.T) {
	builder := NewJobBuilder()

	input := &BuildJobInput{
		Run:     &models.PipelineRun{ID: 5},
		StepRun: &models.StepRun{ID: 50, StepName: "deploy", StepType: "deploy", Image: "kubectl:latest"},
		Namespace: "prod",
	}

	job, err := builder.BuildJob(input)
	if err != nil {
		t.Fatalf("BuildJob failed: %v", err)
	}

	labels := job.Labels
	if labels["synapse.io/pipeline-run-id"] != "5" {
		t.Errorf("expected pipeline-run-id=5, got %s", labels["synapse.io/pipeline-run-id"])
	}
	if labels["synapse.io/step-run-id"] != "50" {
		t.Errorf("expected step-run-id=50, got %s", labels["synapse.io/step-run-id"])
	}
	if labels["synapse.io/step-type"] != "deploy" {
		t.Errorf("expected step-type=deploy, got %s", labels["synapse.io/step-type"])
	}
}

func TestBuildJob_IstioAnnotation(t *testing.T) {
	builder := NewJobBuilder()

	tests := []struct {
		stepType     string
		expectIstio  bool
	}{
		{"build-image", true},
		{"push-image", true},
		{"trivy-scan", true},
		{"build-jar", true},
		{"deploy", false},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.stepType, func(t *testing.T) {
			input := &BuildJobInput{
				Run:     &models.PipelineRun{ID: 1},
				StepRun: &models.StepRun{ID: 1, StepName: "s", StepType: tt.stepType, Image: "img"},
				Namespace: "ns",
			}
			job, err := builder.BuildJob(input)
			if err != nil {
				t.Fatalf("BuildJob failed: %v", err)
			}
			_, hasAnno := job.Spec.Template.Annotations["ambient.istio.io/redirection"]
			if hasAnno != tt.expectIstio {
				t.Errorf("stepType=%s: expected istio annotation=%v, got=%v",
					tt.stepType, tt.expectIstio, hasAnno)
			}
		})
	}
}

func TestBuildJob_ImagePullPolicy(t *testing.T) {
	builder := NewJobBuilder()

	tests := []struct {
		image  string
		expect string
	}{
		{"golang:1.22", "IfNotPresent"},
		{"golang:latest", "Always"},
		{"golang", "Always"},       // no tag = latest
		{"myrepo/img:v1.2.3", "IfNotPresent"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			input := &BuildJobInput{
				Run:     &models.PipelineRun{ID: 1},
				StepRun: &models.StepRun{ID: 1, StepName: "s", StepType: "test", Image: tt.image},
				Namespace: "ns",
			}
			job, err := builder.BuildJob(input)
			if err != nil {
				t.Fatalf("BuildJob failed: %v", err)
			}
			got := string(job.Spec.Template.Spec.Containers[0].ImagePullPolicy)
			if got != tt.expect {
				t.Errorf("image=%s: got pullPolicy=%s, want=%s", tt.image, got, tt.expect)
			}
		})
	}
}

func TestBuildJob_TerminationGracePeriod(t *testing.T) {
	builder := NewJobBuilder()
	input := &BuildJobInput{
		Run:     &models.PipelineRun{ID: 1},
		StepRun: &models.StepRun{ID: 1, StepName: "s", StepType: "test", Image: "img:v1"},
		Namespace: "ns",
	}
	job, err := builder.BuildJob(input)
	if err != nil {
		t.Fatalf("BuildJob failed: %v", err)
	}
	grace := job.Spec.Template.Spec.TerminationGracePeriodSeconds
	if grace == nil || *grace != 30 {
		t.Errorf("expected terminationGracePeriodSeconds=30, got %v", grace)
	}
}

func TestSanitizeK8sName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"build-image", "build-image"},
		{"Build_Image", "build-image"},
		{"My Step Name!", "my-step-name"},
		{"---leading---", "leading"},
		{"a-very-long-name-that-exceeds-thirty-characters-limit", "a-very-long-name-that-exceeds-"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeK8sName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeK8sName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
