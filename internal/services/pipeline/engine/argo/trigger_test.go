package argo

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// newWorkflow builds an unstructured Workflow for seeding.
func newWorkflow(name, namespace string, status map[string]any) *unstructured.Unstructured {
	w := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"workflowTemplateRef": map[string]any{"name": "wt"},
		},
	}}
	if status != nil {
		w.Object["status"] = status
	}
	return w
}

// ---------------------------------------------------------------------------
// Trigger — validation
// ---------------------------------------------------------------------------

func TestTrigger_NilRequest(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"workflow_template_name":"wt","namespace":"ci"}`)
	if _, err := a.Trigger(context.Background(), nil); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestTrigger_MissingTemplate(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestTrigger_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"workflow_template_name":"wt"}`)
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestTrigger_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"workflow_template_name":"wt","namespace":"ci"}`)
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Trigger — happy paths
// ---------------------------------------------------------------------------

func TestTrigger_Success(t *testing.T) {
	resolver := newResolverArgoInstalled(t)
	a := newAdapter(t, resolver, `{"workflow_template_name":"build-app","namespace":"ci","service_account_name":"runner"}`)

	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{
		PipelineID: 42,
		SnapshotID: 7,
		Variables:  map[string]string{"ENV": "staging"},
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if res.RunID == "" || res.RunID != res.ExternalID {
		t.Fatalf("RunID: %+v", res)
	}

	dyn, _ := resolver.Dynamic(1)
	list, err := dyn.Resource(gvrWorkflow).Namespace("ci").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 Workflow, got %d", len(list.Items))
	}
	wf := list.Items[0]
	labels := wf.GetLabels()
	if labels[managedByLabelKey] != managedByLabelValue {
		t.Fatalf("missing managed-by: %+v", labels)
	}
	if labels[synapseRunIDLabel] != "7" || labels[synapsePipelineIDLabel] != "42" {
		t.Fatalf("labels = %+v", labels)
	}

	spec, _ := wf.Object["spec"].(map[string]any)
	tref, _ := spec["workflowTemplateRef"].(map[string]any)
	if tref["name"] != "build-app" {
		t.Fatalf("workflowTemplateRef.name = %v", tref["name"])
	}
	if spec["serviceAccountName"] != "runner" {
		t.Fatalf("serviceAccountName = %v", spec["serviceAccountName"])
	}
	args, _ := spec["arguments"].(map[string]any)
	params, _ := args["parameters"].([]any)
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
}

func TestTrigger_NoVariables_OmitsArguments(t *testing.T) {
	resolver := newResolverArgoInstalled(t)
	a := newAdapter(t, resolver, `{"workflow_template_name":"wt","namespace":"ci"}`)
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	dyn, _ := resolver.Dynamic(1)
	list, _ := dyn.Resource(gvrWorkflow).Namespace("ci").List(context.Background(), metav1.ListOptions{})
	spec := list.Items[0].Object["spec"].(map[string]any)
	if _, present := spec["arguments"]; present {
		t.Fatalf("arguments should be omitted")
	}
}

func TestTrigger_NoServiceAccount(t *testing.T) {
	resolver := newResolverArgoInstalled(t)
	a := newAdapter(t, resolver, `{"workflow_template_name":"wt","namespace":"ci"}`)
	if _, err := a.Trigger(context.Background(), &engine.TriggerRequest{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	dyn, _ := resolver.Dynamic(1)
	list, _ := dyn.Resource(gvrWorkflow).Namespace("ci").List(context.Background(), metav1.ListOptions{})
	spec := list.Items[0].Object["spec"].(map[string]any)
	if _, present := spec["serviceAccountName"]; present {
		t.Fatalf("serviceAccountName should be omitted when not set")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestBuildArguments_Empty(t *testing.T) {
	if got := buildArguments(nil); got != nil {
		t.Fatalf("nil: %v", got)
	}
	if got := buildArguments(map[string]string{}); got != nil {
		t.Fatalf("empty: %v", got)
	}
}

func TestBuildArguments_Shape(t *testing.T) {
	got := buildArguments(map[string]string{"X": "1"})
	params, ok := got["parameters"].([]any)
	if !ok || len(params) != 1 {
		t.Fatalf("bad shape: %+v", got)
	}
	p := params[0].(map[string]any)
	if p["name"] != "X" || p["value"] != "1" {
		t.Fatalf("param: %+v", p)
	}
}

// TestMapArgoPhase locks in the status mapping.
func TestMapArgoPhase_AllValues(t *testing.T) {
	cases := map[string]engine.RunPhase{
		"Pending":   engine.RunPhasePending,
		"Running":   engine.RunPhaseRunning,
		"Succeeded": engine.RunPhaseSuccess,
		"Failed":    engine.RunPhaseFailed,
		"Error":     engine.RunPhaseFailed,
		"":          engine.RunPhasePending,
		"Strange":   engine.RunPhaseUnknown,
	}
	for in, want := range cases {
		if got := mapArgoPhase(in); got != want {
			t.Fatalf("mapArgoPhase(%q) = %q, want %q", in, got, want)
		}
	}
}
