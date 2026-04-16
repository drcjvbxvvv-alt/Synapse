package argo

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/pkg/logger"
)

// StreamLogs streams the log output for a single node inside an Argo Workflow.
//
// # stepID format
//
// Argo Workflow steps are represented as "nodes" inside status.nodes (a map
// keyed by the internal node id). stepID may be:
//
//   - "<node-id>"         — the internal Argo node id (also equals pod name)
//   - "<displayName>"     — the human-readable step name from the template
//   - ""                  — auto-select the first Pod-type node (alphabetical)
//
// # Pod resolution
//
// In Argo the node id equals the pod name created for that step. The container
// to stream is always "main" — Argo injects wait/init sidecars but the user
// workload runs in "main".
func (a *Adapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("argo.StreamLogs: empty run id: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return nil, err
	}
	ns, err := a.extra.requireNamespace()
	if err != nil {
		return nil, err
	}

	// ── 1. Dynamic client to get the Workflow ────────────────────────────
	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("argo.StreamLogs: dynamic client: %w", engine.ErrUnavailable)
	}

	// ── 2. Fetch the Workflow object ──────────────────────────────────────
	wfObj, err := dyn.Resource(gvrWorkflow).Namespace(ns).Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("argo.StreamLogs: get Workflow %s: %w", runID, mapK8sError(err))
	}

	// ── 3. Resolve node id (= pod name) from stepID ──────────────────────
	nodeID, err := resolveArgoNodeID(dyn, ns, runID, stepID, wfObj.Object)
	if err != nil {
		return nil, err
	}

	// ── 4. Typed clientset to stream pod logs ─────────────────────────────
	cs, err := a.resolver.Kubernetes(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("argo.StreamLogs: kubernetes client: %w", engine.ErrUnavailable)
	}

	const mainContainer = "main"
	logger.Info("argo.StreamLogs: streaming",
		"run_id", runID,
		"node_id", nodeID,
		"container", mainContainer,
		"namespace", ns,
	)

	rc, err := cs.CoreV1().Pods(ns).GetLogs(nodeID, &corev1.PodLogOptions{
		Container: mainContainer,
		Follow:    false,
	}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("argo.StreamLogs: pod %s container %s: %w",
			nodeID, mainContainer, mapK8sError(err))
	}
	return rc, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveArgoNodeID returns the Argo node id for the requested step.
//
//   - If stepID is non-empty it is matched against node displayName first,
//     then node id (direct key lookup). Both are unambiguous identifiers.
//   - If empty, the first Pod-type node is chosen (smallest id
//     alphabetically for stable selection).
//
// The node id equals the Kubernetes pod name created for that step.
func resolveArgoNodeID(
	_ dynamic.Interface,
	_, runID, stepID string,
	wfObject map[string]any,
) (string, error) {
	nodes := argoNodes(wfObject)
	if len(nodes) == 0 {
		return "", fmt.Errorf("argo.StreamLogs: Workflow %s has no nodes yet: %w",
			runID, engine.ErrNotFound)
	}

	if stepID != "" {
		// Match by displayName or id.
		for id, raw := range nodes {
			node, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if v, _ := node["displayName"].(string); v == stepID {
				return id, nil
			}
			if id == stepID {
				return id, nil
			}
		}
		return "", fmt.Errorf("argo.StreamLogs: no node matching stepID %q in Workflow %s: %w",
			stepID, runID, engine.ErrNotFound)
	}

	// stepID empty → pick first Pod-type node alphabetically.
	firstID := ""
	for id, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		nodeType, _ := node["type"].(string)
		if nodeType != "Pod" {
			continue
		}
		if firstID == "" || id < firstID {
			firstID = id
		}
	}
	if firstID == "" {
		return "", fmt.Errorf("argo.StreamLogs: Workflow %s has no Pod-type nodes: %w",
			runID, engine.ErrNotFound)
	}
	return firstID, nil
}

// argoNodes extracts the status.nodes map from a Workflow Unstructured object.
func argoNodes(obj map[string]any) map[string]any {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return nil
	}
	nodes, _ := status["nodes"].(map[string]any)
	return nodes
}
