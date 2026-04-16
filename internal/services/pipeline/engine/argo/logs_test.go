package argo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newArgoLogServer creates an httptest.Server that responds to the pod-log
// URL for the given pod with logContent; other paths return 404.
func newArgoLogServer(t *testing.T, ns, podName, logContent string) *httptest.Server {
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

// k8sClientForArgoLogs builds a real kubernetes.Clientset backed by the
// supplied httptest.Server — mirrors the Tekton pattern.
func k8sClientForArgoLogs(t *testing.T, srv *httptest.Server) kubernetes.Interface {
	t.Helper()
	cfg := &rest.Config{Host: srv.URL}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes.NewForConfig: %v", err)
	}
	return cs
}

// ---------------------------------------------------------------------------
// resolveArgoNodeID unit tests
// ---------------------------------------------------------------------------

func TestResolveArgoNodeID_ExplicitID_Match(t *testing.T) {
	nodes := map[string]any{
		"wf-abc-step1": map[string]any{"type": "Pod", "displayName": "build"},
		"wf-abc-step2": map[string]any{"type": "Pod", "displayName": "test"},
	}
	obj := map[string]any{"status": map[string]any{"nodes": nodes}}

	id, err := resolveArgoNodeID(nil, "ci", "wf-abc", "wf-abc-step1", obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "wf-abc-step1" {
		t.Fatalf("expected wf-abc-step1, got %q", id)
	}
}

func TestResolveArgoNodeID_ByDisplayName(t *testing.T) {
	nodes := map[string]any{
		"wf-abc-xyz": map[string]any{"type": "Pod", "displayName": "my-step"},
	}
	obj := map[string]any{"status": map[string]any{"nodes": nodes}}

	id, err := resolveArgoNodeID(nil, "ci", "wf-abc", "my-step", obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "wf-abc-xyz" {
		t.Fatalf("expected wf-abc-xyz, got %q", id)
	}
}

func TestResolveArgoNodeID_StepIDNotFound(t *testing.T) {
	nodes := map[string]any{
		"wf-abc-step1": map[string]any{"type": "Pod", "displayName": "build"},
	}
	obj := map[string]any{"status": map[string]any{"nodes": nodes}}

	_, err := resolveArgoNodeID(nil, "ci", "wf-abc", "nonexistent", obj)
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveArgoNodeID_Empty_PicksFirstPodNode(t *testing.T) {
	// "wf-abc-aaa" < "wf-abc-zzz" alphabetically; should pick aaa.
	nodes := map[string]any{
		"wf-abc-zzz": map[string]any{"type": "Pod", "displayName": "test"},
		"wf-abc-aaa": map[string]any{"type": "Pod", "displayName": "build"},
	}
	obj := map[string]any{"status": map[string]any{"nodes": nodes}}

	id, err := resolveArgoNodeID(nil, "ci", "wf-abc", "", obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "wf-abc-aaa" {
		t.Fatalf("expected wf-abc-aaa (alphabetically first Pod node), got %q", id)
	}
}

func TestResolveArgoNodeID_Empty_SkipsNonPodNodes(t *testing.T) {
	// Only DAG-type node — no Pod nodes.
	nodes := map[string]any{
		"wf-abc-dag": map[string]any{"type": "DAG", "displayName": "main"},
	}
	obj := map[string]any{"status": map[string]any{"nodes": nodes}}

	_, err := resolveArgoNodeID(nil, "ci", "wf-abc", "", obj)
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for no Pod nodes, got %v", err)
	}
}

func TestResolveArgoNodeID_Empty_NoNodes(t *testing.T) {
	obj := map[string]any{"status": map[string]any{}}

	_, err := resolveArgoNodeID(nil, "ci", "wf-abc", "", obj)
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for empty nodes, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// argoNodes unit tests
// ---------------------------------------------------------------------------

func TestArgoNodes_Present(t *testing.T) {
	nodes := map[string]any{
		"n1": map[string]any{"type": "Pod"},
	}
	obj := map[string]any{"status": map[string]any{"nodes": nodes}}
	got := argoNodes(obj)
	if len(got) != 1 {
		t.Fatalf("expected 1 node, got %d", len(got))
	}
}

func TestArgoNodes_MissingStatus(t *testing.T) {
	if got := argoNodes(map[string]any{}); got != nil {
		t.Fatalf("expected nil for missing status")
	}
}

func TestArgoNodes_EmptyNodes(t *testing.T) {
	obj := map[string]any{"status": map[string]any{"nodes": map[string]any{}}}
	if got := argoNodes(obj); len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// StreamLogs — validation error paths
// ---------------------------------------------------------------------------

func TestStreamLogs_Argo_EmptyRunID(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStreamLogs_Argo_NilResolver(t *testing.T) {
	a := newAdapter(t, nil, `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "wf-1", "")
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestStreamLogs_Argo_MissingNamespace(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	_, err := a.StreamLogs(context.Background(), "wf-1", "")
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for missing namespace, got %v", err)
	}
}

func TestStreamLogs_Argo_WorkflowNotFound(t *testing.T) {
	// Dynamic client seeded with no Workflows — Get will 404.
	a := newAdapter(t, newResolverArgoInstalled(t), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "missing-wf", "")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStreamLogs_Argo_NoNodes(t *testing.T) {
	// Workflow exists but status.nodes is absent (workflow not yet started).
	wf := newWorkflow("wf-1", "ci", map[string]any{"phase": "Pending"})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "wf-1", "")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when no nodes, got %v", err)
	}
}

func TestStreamLogs_Argo_StepIDNotFound(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{
		"nodes": map[string]any{
			"wf-1-abc": map[string]any{"type": "Pod", "displayName": "build"},
		},
	})
	a := newAdapter(t, newResolverArgoInstalled(t, wf), `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "wf-1", "nonexistent-step")
	if !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStreamLogs_Argo_KubernetesClientError(t *testing.T) {
	wf := newWorkflow("wf-1", "ci", map[string]any{
		"nodes": map[string]any{
			"wf-1-pod": map[string]any{"type": "Pod", "displayName": "build"},
		},
	})
	r := newResolverArgoInstalled(t, wf)
	r.k8sErr = errors.New("cluster unreachable")
	a := newAdapter(t, r, `{"namespace":"ci"}`)
	_, err := a.StreamLogs(context.Background(), "wf-1", "")
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable when k8s client fails, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// StreamLogs — happy path (httptest server as fake K8s API)
// ---------------------------------------------------------------------------

func TestStreamLogs_Argo_Success_ExplicitNodeID(t *testing.T) {
	const (
		ns         = "ci"
		nodeID     = "wf-abc-pod-xyz"
		logContent = "argo step build: done\n"
	)

	wf := newWorkflow("wf-abc", ns, map[string]any{
		"nodes": map[string]any{
			nodeID: map[string]any{"type": "Pod", "displayName": "build"},
		},
	})

	srv := newArgoLogServer(t, ns, nodeID, logContent)
	cs := k8sClientForArgoLogs(t, srv)

	r := newResolverArgoInstalled(t, wf)
	r.k8s = cs
	a := newAdapter(t, r, `{"namespace":"`+ns+`"}`)

	rc, err := a.StreamLogs(context.Background(), "wf-abc", nodeID)
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !strings.Contains(string(got), "done") {
		t.Fatalf("expected log content, got %q", string(got))
	}
}

func TestStreamLogs_Argo_Success_ByDisplayName(t *testing.T) {
	const (
		ns         = "ci"
		nodeID     = "wf-abc-xyz99"
		logContent = "display-name step log\n"
	)

	wf := newWorkflow("wf-abc", ns, map[string]any{
		"nodes": map[string]any{
			nodeID: map[string]any{"type": "Pod", "displayName": "my-special-step"},
		},
	})

	srv := newArgoLogServer(t, ns, nodeID, logContent)
	cs := k8sClientForArgoLogs(t, srv)

	r := newResolverArgoInstalled(t, wf)
	r.k8s = cs
	a := newAdapter(t, r, `{"namespace":"`+ns+`"}`)

	rc, err := a.StreamLogs(context.Background(), "wf-abc", "my-special-step")
	if err != nil {
		t.Fatalf("StreamLogs by displayName: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if !strings.Contains(string(got), "display-name") {
		t.Fatalf("expected log content, got %q", string(got))
	}
}

func TestStreamLogs_Argo_Success_EmptyStepID_AutoSelect(t *testing.T) {
	const (
		ns         = "ci"
		podName    = "wf-z-aaa-pod" // alphabetically first among Pod nodes
		logContent = "auto-selected argo step\n"
	)

	wf := newWorkflow("wf-z", ns, map[string]any{
		"nodes": map[string]any{
			"wf-z-zzz-pod": map[string]any{"type": "Pod", "displayName": "test"},
			podName:         map[string]any{"type": "Pod", "displayName": "build"},
		},
	})

	srv := newArgoLogServer(t, ns, podName, logContent)
	cs := k8sClientForArgoLogs(t, srv)

	r := newResolverArgoInstalled(t, wf)
	r.k8s = cs
	a := newAdapter(t, r, `{"namespace":"`+ns+`"}`)

	rc, err := a.StreamLogs(context.Background(), "wf-z", "") // empty stepID
	if err != nil {
		t.Fatalf("StreamLogs with empty stepID: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if !strings.Contains(string(got), "auto-selected") {
		t.Fatalf("expected auto-selected log, got %q", string(got))
	}
}
