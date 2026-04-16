package tekton

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newLogTaskRun builds a minimal TaskRun unstructured object for log streaming tests.
//
//   - name: TaskRun metadata.name
//   - prName: tekton.dev/pipelineRun label value (links it to a PipelineRun)
//   - podName: status.podName (pod that actually runs the steps)
//   - stepNames: steps to seed in spec.steps[].name (generates "step-<name>" containers)
func newLogTaskRun(name, ns, prName, podName string, stepNames ...string) *unstructured.Unstructured {
	steps := make([]any, 0, len(stepNames))
	for _, s := range stepNames {
		steps = append(steps, map[string]any{"name": s})
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "TaskRun",
		"metadata": map[string]any{
			"name":      name,
			"namespace": ns,
			"labels": map[string]any{
				"tekton.dev/pipelineRun": prName,
			},
		},
		"spec": map[string]any{
			"steps": steps,
		},
		"status": map[string]any{
			"podName": podName,
		},
	}}
	obj.SetGroupVersionKind(gvrTaskRun.GroupVersion().WithKind("TaskRun"))
	return obj
}

// k8sClientForPodLogs builds a real kubernetes.Clientset whose REST calls
// are forwarded to the supplied httptest.Server.  The server is responsible
// for replying to GET /api/v1/namespaces/<ns>/pods/<pod>/log with the desired
// content.
func k8sClientForPodLogs(t *testing.T, srv *httptest.Server) kubernetes.Interface {
	t.Helper()
	cfg := &rest.Config{Host: srv.URL}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes.NewForConfig: %v", err)
	}
	return cs
}

// newLogServer creates an httptest.Server that responds to the pod-log URL
// path with logContent and other requests with 404.
func newLogServer(t *testing.T, ns, podName, logContent string) *httptest.Server {
	t.Helper()
	expectedPath := "/api/v1/namespaces/" + ns + "/pods/" + podName + "/log"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == expectedPath {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(logContent))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// resolverWithK8s extends a dynamic resolver with a kubernetes clientset for
// log streaming tests.
func resolverWithK8s(base *fakeResolver, cs kubernetes.Interface) *fakeResolver {
	base.k8s = cs
	return base
}

// ---------------------------------------------------------------------------
// resolveStepID unit tests
// ---------------------------------------------------------------------------

func TestResolveStepID_ExplicitID(t *testing.T) {
	dyn := newDynamicResolver(t)
	trName, container, err := resolveStepID(context.Background(), dyn.dyn, "ci", "pr-1", "my-tr/step-build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trName != "my-tr" || container != "step-build" {
		t.Fatalf("got (%q, %q), want (my-tr, step-build)", trName, container)
	}
}

func TestResolveStepID_InvalidFormat_MissingSlash(t *testing.T) {
	dyn := newDynamicResolver(t)
	_, _, err := resolveStepID(context.Background(), dyn.dyn, "ci", "pr-1", "notaslash")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestResolveStepID_InvalidFormat_EmptyParts(t *testing.T) {
	dyn := newDynamicResolver(t)
	_, _, err := resolveStepID(context.Background(), dyn.dyn, "ci", "pr-1", "/step-build")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestResolveStepID_Empty_NoTaskRuns(t *testing.T) {
	dyn := newDynamicResolver(t) // no objects seeded
	_, _, err := resolveStepID(context.Background(), dyn.dyn, "ci", "pr-1", "")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveStepID_Empty_PicksFirstAlphabetically(t *testing.T) {
	tr1 := newLogTaskRun("tr-b", "ci", "pr-1", "pod-b", "build")
	tr2 := newLogTaskRun("tr-a", "ci", "pr-1", "pod-a", "test", "lint")

	dyn := newDynamicResolver(t, tr1, tr2)
	trName, container, err := resolveStepID(context.Background(), dyn.dyn, "ci", "pr-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trName != "tr-a" {
		t.Fatalf("expected tr-a (alphabetically first), got %q", trName)
	}
	if container != "step-test" {
		t.Fatalf("expected step-test (first step of tr-a), got %q", container)
	}
}

func TestResolveStepID_Empty_TaskRunHasNoSteps(t *testing.T) {
	tr := newLogTaskRun("tr-1", "ci", "pr-1", "pod-1") // no step names → no containers
	dyn := newDynamicResolver(t, tr)
	_, _, err := resolveStepID(context.Background(), dyn.dyn, "ci", "pr-1", "")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound (no step containers), got %v", err)
	}
}

// ---------------------------------------------------------------------------
// readPodName unit tests
// ---------------------------------------------------------------------------

func TestReadPodName_Present(t *testing.T) {
	tr := newLogTaskRun("tr-1", "ci", "pr-1", "my-pod-xyz")
	if got := readPodName(tr.Object); got != "my-pod-xyz" {
		t.Fatalf("expected my-pod-xyz, got %q", got)
	}
}

func TestReadPodName_Empty(t *testing.T) {
	tr := newLogTaskRun("tr-1", "ci", "pr-1", "") // empty podName
	if got := readPodName(tr.Object); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestReadPodName_MissingStatus(t *testing.T) {
	obj := map[string]any{"metadata": map[string]any{"name": "tr-1"}}
	if got := readPodName(obj); got != "" {
		t.Fatalf("expected empty string for missing status, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// firstStepContainer unit tests
// ---------------------------------------------------------------------------

func TestFirstStepContainer_Normal(t *testing.T) {
	tr := newLogTaskRun("tr-1", "ci", "pr-1", "pod-1", "build", "test")
	if got := firstStepContainer(tr.Object); got != "step-build" {
		t.Fatalf("expected step-build, got %q", got)
	}
}

func TestFirstStepContainer_NoSteps(t *testing.T) {
	tr := newLogTaskRun("tr-1", "ci", "pr-1", "pod-1")
	if got := firstStepContainer(tr.Object); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFirstStepContainer_NoSpec(t *testing.T) {
	obj := map[string]any{"metadata": map[string]any{"name": "tr-1"}}
	if got := firstStepContainer(obj); got != "" {
		t.Fatalf("expected empty for missing spec, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// StreamLogs — validation error paths
// ---------------------------------------------------------------------------

func TestStreamLogs_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "pr-1", "")
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestStreamLogs_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), "")
	_, err := a.StreamLogs(context.Background(), "pr-1", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for missing namespace, got %v", err)
	}
}

func TestStreamLogs_InvalidStepIDFormat(t *testing.T) {
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "pr-1", "no-slash-here")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad stepID format, got %v", err)
	}
}

func TestStreamLogs_TaskRunNotFound_WhenExplicitStepID(t *testing.T) {
	// Dynamic client has no TaskRuns; explicit stepID causes a Get that 404s.
	a := newAdapter(t, newDynamicResolver(t), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "pr-1", "missing-tr/step-build")
	// mapK8sError wraps the fake 404 as ErrNotFound.
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStreamLogs_NoPodNameYet(t *testing.T) {
	// TaskRun exists but podName is still empty (not yet scheduled).
	tr := newLogTaskRun("my-tr", "ci", "pr-1", "" /* empty podName */, "build")
	a := newAdapter(t, newDynamicResolver(t, tr), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "pr-1", "my-tr/step-build")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when podName is empty, got %v", err)
	}
}

func TestStreamLogs_KubernetesClientError(t *testing.T) {
	tr := newLogTaskRun("my-tr", "ci", "pr-1", "my-pod", "build")
	base := newDynamicResolver(t, tr)
	base.k8sErr = errors.New("cluster unreachable")
	a := newAdapter(t, base, `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "pr-1", "my-tr/step-build")
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable when k8s client fails, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// StreamLogs — happy path (httptest server as fake K8s API)
// ---------------------------------------------------------------------------

func TestStreamLogs_Success_ExplicitStepID(t *testing.T) {
	const (
		ns         = "ci"
		podName    = "my-pod-abc12"
		logContent = "step-build: compiling...\nstep-build: done\n"
	)
	tr := newLogTaskRun("my-tr", ns, "pr-1", podName, "build")

	// httptest server serves the pod log.
	srv := newLogServer(t, ns, podName, logContent)
	cs := k8sClientForPodLogs(t, srv)

	base := newDynamicResolver(t, tr)
	base.k8s = cs
	a := newAdapter(t, base, `{"namespace":"`+ns+`"}`)

	rc, err := a.StreamLogs(context.Background(), "pr-1", "my-tr/step-build")
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !strings.Contains(string(got), "compiling") {
		t.Fatalf("expected log content, got %q", string(got))
	}
}

func TestStreamLogs_Success_EmptyStepID_AutoSelect(t *testing.T) {
	const (
		ns         = "ci"
		podName    = "auto-pod-xyz99"
		logContent = "auto-selected step log\n"
	)
	// Two TaskRuns; tr-a is alphabetically first and has step "unit-test".
	tr1 := newLogTaskRun("tr-b", ns, "pr-2", "pod-b", "lint")
	tr2 := newLogTaskRun("tr-a", ns, "pr-2", podName, "unit-test")

	srv := newLogServer(t, ns, podName, logContent)
	cs := k8sClientForPodLogs(t, srv)

	base := newDynamicResolver(t, tr1, tr2)
	base.k8s = cs
	a := newAdapter(t, base, `{"namespace":"`+ns+`"}`)

	rc, err := a.StreamLogs(context.Background(), "pr-2", "") // empty stepID
	if err != nil {
		t.Fatalf("StreamLogs with empty stepID: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !strings.Contains(string(got), "auto-selected") {
		t.Fatalf("expected auto-selected log, got %q", string(got))
	}
}

// ---------------------------------------------------------------------------
// Compile-time object conversion helper — needed by newDynamicResolver.
// ---------------------------------------------------------------------------

// unstructuredToRuntimeObject satisfies runtime.Object for seeding the fake
// dynamic client.
func mustToUnstructured(obj *unstructured.Unstructured) runtime.Object {
	return obj
}
