package tekton

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestCancel_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), "")
	if err := a.Cancel(context.Background(), "pr-1"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCancel_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "pr-1"); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestCancel_NotFound(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "missing"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCancel_AlreadyTerminal_Success(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "True"},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "pr-1"); !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestCancel_AlreadyTerminal_Failed(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "False", "reason": "SomeError"},
		},
	})
	a := newAdapter(t, newDynamicResolver(t, pr), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "pr-1"); !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("expected ErrAlreadyTerminal, got %v", err)
	}
}

func TestCancel_RunningRun_PatchesSpecStatus(t *testing.T) {
	pr := newPipelineRun("pr-1", "ci", map[string]any{
		"conditions": []any{
			map[string]any{"type": "Succeeded", "status": "Unknown", "reason": "Running"},
		},
	})
	resolver := newDynamicResolver(t, pr)
	a := newAdapter(t, resolver, `{"namespace":"ci"}`)

	if err := a.Cancel(context.Background(), "pr-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Verify spec.status was patched to "Cancelled".
	dyn, _ := resolver.Dynamic(1)
	pr2, err := dyn.Resource(gvrPipelineRun).Namespace("ci").Get(
		context.Background(), "pr-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	spec, _ := pr2.Object["spec"].(map[string]any)
	if spec["status"] != cancelSpecStatus {
		t.Fatalf("spec.status = %v, want %q", spec["status"], cancelSpecStatus)
	}
}
