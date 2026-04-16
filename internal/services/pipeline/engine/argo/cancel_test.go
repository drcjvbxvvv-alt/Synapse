package argo

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

func TestCancel_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), ""); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	if err := a.Cancel(context.Background(), "wf-1"); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "wf-1"); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_NotFound(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "missing"); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_AlreadyTerminal_Success(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{"phase": "Succeeded"})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "wf-1"); !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_AlreadyTerminal_Failed(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{"phase": "Failed"})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	if err := a.Cancel(context.Background(), "wf-1"); !errors.Is(err, engine.ErrAlreadyTerminal) {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_Running_PatchesSpecShutdown(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{"phase": "Running"})
	resolver := newResolverArgoInstalled(t, wf)
	a := newAdapter(t, resolver, `{"namespace":"ci"}`)

	if err := a.Cancel(context.Background(), "wf-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	dyn, _ := resolver.Dynamic(1)
	after, err := dyn.Resource(gvrWorkflow).Namespace("ci").Get(
		context.Background(), "wf-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	spec, _ := after.Object["spec"].(map[string]any)
	if spec["shutdown"] != shutdownTerminate {
		t.Fatalf("spec.shutdown = %v, want %q", spec["shutdown"], shutdownTerminate)
	}
}
