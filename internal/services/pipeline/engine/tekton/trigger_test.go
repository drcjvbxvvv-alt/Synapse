package tekton

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Test helpers — fake dynamic client with Tekton GVR → ListKind mapping.
// ---------------------------------------------------------------------------

// newDynamicResolver builds a fakeResolver whose dynamic client knows about
// Tekton's GVR→ListKind mapping (required by NewSimpleDynamicClient when
// the scheme doesn't register typed objects). The initial `objects` are
// seeded into the client for Get/List tests.
func newDynamicResolver(t *testing.T, objects ...runtime.Object) *fakeResolver {
	t.Helper()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		gvrPipelineRun: "PipelineRunList",
		gvrTaskRun:     "TaskRunList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)

	// Discovery advertises tekton.dev/v1 so IsAvailable / Version behave
	// consistently across Stage-3 tests.
	fc := &clienttesting.Fake{Resources: []*metav1.APIResourceList{{
		GroupVersion: "tekton.dev/v1",
		APIResources: []metav1.APIResource{
			{Name: "pipelineruns", Namespaced: true, Kind: "PipelineRun"},
			{Name: "taskruns", Namespaced: true, Kind: "TaskRun"},
		},
	}}}
	return &fakeResolver{
		dyn:  dyn,
		disc: &discoveryfake.FakeDiscovery{Fake: fc},
	}
}

// newPipelineRun builds an unstructured PipelineRun for seeding the fake
// dynamic client. status is an optional map (nil → PipelineRun with just
// spec).
func newPipelineRun(name, namespace string, status map[string]any) *unstructured.Unstructured {
	pr := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "PipelineRun",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"pipelineRef": map[string]any{"name": "my-pipeline"},
		},
	}}
	if status != nil {
		pr.Object["status"] = status
	}
	return pr
}

// newTaskRun builds an unstructured TaskRun labelled for the given
// PipelineRun.
func newTaskRun(name, namespace, pipelineRun, pipelineTask string, status map[string]any) *unstructured.Unstructured {
	tr := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "TaskRun",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]any{
				"tekton.dev/pipelineRun":  pipelineRun,
				"tekton.dev/pipelineTask": pipelineTask,
			},
		},
	}}
	if status != nil {
		tr.Object["status"] = status
	}
	return tr
}

// discardDiscovery returns a resolver with a minimal discovery fake so
// callers that only exercise the dynamic path don't pay the setup cost.
var _ discovery.DiscoveryInterface = (*discoveryfake.FakeDiscovery)(nil)

// ---------------------------------------------------------------------------
// Trigger — validation
// ---------------------------------------------------------------------------

func TestTrigger_NilRequest(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"pipeline_name":"p","namespace":"ns"}`)
	_, err := a.Trigger(context.Background(), nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTrigger_MissingPipelineName(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ns"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTrigger_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"pipeline_name":"p"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTrigger_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"pipeline_name":"p","namespace":"ns"}`)
	// Construction allowed nil resolver; execution must fail with ErrUnavailable.
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Trigger — happy path
// ---------------------------------------------------------------------------

func TestTrigger_Success(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"pipeline_name":"build-app","namespace":"ci","service_account_name":"runner"}`)

	res, err := a.Trigger(context.Background(), &engine.TriggerRequest{
		PipelineID: 42,
		SnapshotID: 7,
		Variables:  map[string]string{"ENV": "staging"},
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if res.RunID == "" {
		t.Fatal("RunID should not be empty")
	}
	if res.RunID != res.ExternalID {
		t.Fatalf("RunID/ExternalID should match: %+v", res)
	}

	// Verify the PipelineRun actually landed in the fake dynamic store.
	dyn, _ := a.resolver.Dynamic(a.clusterID)
	list, err := dyn.Resource(gvrPipelineRun).Namespace("ci").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 PipelineRun in ci namespace, got %d", len(list.Items))
	}
	pr := list.Items[0]

	// Labels: managed-by + synapse.io/* keys.
	labels := pr.GetLabels()
	if labels[managedByLabelKey] != managedByLabelValue {
		t.Fatalf("missing managed-by label: %+v", labels)
	}
	if labels[synapsePipelineIDLabel] != "42" {
		t.Fatalf("synapse.io/pipeline-id = %q", labels[synapsePipelineIDLabel])
	}
	if labels[synapseRunIDLabel] != "7" {
		t.Fatalf("synapse.io/run-id = %q", labels[synapseRunIDLabel])
	}

	// Spec: pipelineRef.name + serviceAccountName propagated.
	spec, _ := pr.Object["spec"].(map[string]any)
	pref, _ := spec["pipelineRef"].(map[string]any)
	if pref["name"] != "build-app" {
		t.Fatalf("pipelineRef.name = %v", pref["name"])
	}
	trt, _ := spec["taskRunTemplate"].(map[string]any)
	if trt == nil || trt["serviceAccountName"] != "runner" {
		t.Fatalf("serviceAccountName not propagated: %+v", trt)
	}
	params, _ := spec["params"].([]any)
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
}

func TestTrigger_NoVariables_OmitsParams(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"pipeline_name":"p","namespace":"ns"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	dyn, _ := a.resolver.Dynamic(a.clusterID)
	list, _ := dyn.Resource(gvrPipelineRun).Namespace("ns").List(context.Background(), metav1.ListOptions{})
	pr := list.Items[0]
	spec := pr.Object["spec"].(map[string]any)
	if _, ok := spec["params"]; ok {
		t.Fatalf("params should be omitted when no variables, got %+v", spec["params"])
	}
}

func TestTrigger_NoServiceAccount_OmitsTemplate(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"pipeline_name":"p","namespace":"ns"}`)
	_, err := a.Trigger(context.Background(), &engine.TriggerRequest{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	dyn, _ := a.resolver.Dynamic(a.clusterID)
	list, _ := dyn.Resource(gvrPipelineRun).Namespace("ns").List(context.Background(), metav1.ListOptions{})
	pr := list.Items[0]
	spec := pr.Object["spec"].(map[string]any)
	if _, ok := spec["taskRunTemplate"]; ok {
		t.Fatalf("taskRunTemplate should be omitted when no SA, got %+v", spec["taskRunTemplate"])
	}
}

// ---------------------------------------------------------------------------
// convertVariablesToParams
// ---------------------------------------------------------------------------

func TestConvertVariablesToParams_Empty(t *testing.T) {
	if got := convertVariablesToParams(nil); got != nil {
		t.Fatalf("nil map → %v, want nil", got)
	}
	if got := convertVariablesToParams(map[string]string{}); got != nil {
		t.Fatalf("empty map → %v, want nil", got)
	}
}

func TestConvertVariablesToParams_Shape(t *testing.T) {
	got := convertVariablesToParams(map[string]string{"ENV": "staging"})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	m, ok := got[0].(map[string]any)
	if !ok {
		t.Fatalf("param[0] not a map: %+v", got[0])
	}
	if m["name"] != "ENV" || m["value"] != "staging" {
		t.Fatalf("param shape wrong: %+v", m)
	}
}
