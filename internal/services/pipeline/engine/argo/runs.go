package argo

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetRun reads a Workflow's status and composes an engine.RunStatus.
//
// Argo encodes per-step results in status.nodes (a map keyed by the
// internal node id). Unlike Tekton where child TaskRuns are separate
// CRDs, Argo's node tree lives entirely within the single Workflow
// object — so there's no List call, just one Get.
func (a *Adapter) GetRun(ctx context.Context, runID string) (*engine.RunStatus, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("argo.GetRun: empty run id: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return nil, err
	}
	ns, err := a.extra.requireNamespace()
	if err != nil {
		return nil, err
	}
	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("argo.GetRun: dynamic client: %w", engine.ErrUnavailable)
	}

	wf, err := dyn.Resource(gvrWorkflow).Namespace(ns).Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("argo.GetRun %s: %w", runID, mapK8sError(err))
	}
	return buildRunStatusFromWorkflow(runID, wf), nil
}

// buildRunStatusFromWorkflow interprets the status subresource.
func buildRunStatusFromWorkflow(runID string, wf *unstructured.Unstructured) *engine.RunStatus {
	phase := readArgoPhase(wf.Object)
	started, finished := readArgoTimes(wf.Object)
	message := readArgoMessage(wf.Object)

	rs := &engine.RunStatus{
		RunID:      runID,
		ExternalID: runID,
		Phase:      mapArgoPhase(phase),
		Raw:        phase,
		Message:    message,
		StartedAt:  started,
		FinishedAt: finished,
	}
	rs.Steps = stepsFromNodes(wf.Object)
	return rs
}

// stepsFromNodes walks status.nodes (a map) and emits one StepStatus per
// node. Argo creates nodes of several types (Pod / Steps / DAG / …);
// callers typically only care about the leaf Pod nodes. M18e surfaces all
// of them — the UI can filter by `type` if needed.
//
// Nodes are sorted by startedAt for stable UI rendering; Argo does not
// guarantee map iteration order.
func stepsFromNodes(obj map[string]any) []engine.StepStatus {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return []engine.StepStatus{}
	}
	nodes, _ := status["nodes"].(map[string]any)
	if len(nodes) == 0 {
		return []engine.StepStatus{}
	}
	steps := make([]engine.StepStatus, 0, len(nodes))
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		phase, _ := node["phase"].(string)
		started := parseTimeField(node, "startedAt")
		finished := parseTimeField(node, "finishedAt")
		steps = append(steps, engine.StepStatus{
			Name:       stepNameFromNode(node),
			Phase:      mapArgoPhase(phase),
			Raw:        phase,
			StartedAt:  started,
			FinishedAt: finished,
		})
	}
	return steps
}

// stepNameFromNode prefers the user-friendly displayName (matches the
// Pipeline template step name), falling back to the internal id.
func stepNameFromNode(node map[string]any) string {
	if v, _ := node["displayName"].(string); v != "" {
		return v
	}
	if v, _ := node["name"].(string); v != "" {
		return v
	}
	if v, _ := node["id"].(string); v != "" {
		return v
	}
	return ""
}

// ---------------------------------------------------------------------------
// Field extractors (unstructured access helpers)
// ---------------------------------------------------------------------------

func readArgoPhase(obj map[string]any) string {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return ""
	}
	s, _ := status["phase"].(string)
	return s
}

func readArgoMessage(obj map[string]any) string {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return ""
	}
	s, _ := status["message"].(string)
	return s
}

func readArgoTimes(obj map[string]any) (started, finished *time.Time) {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return nil, nil
	}
	started = parseTimeField(status, "startedAt")
	finished = parseTimeField(status, "finishedAt")
	return
}

// parseTimeField reads an RFC3339 string from the given map[key]. Returns
// nil when the key is absent, not a string, or doesn't parse.
func parseTimeField(m map[string]any, key string) *time.Time {
	s, ok := m[key].(string)
	if !ok || s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
