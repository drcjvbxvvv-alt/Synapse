package tekton

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

// StreamLogs streams the log output for a single container inside a Tekton
// TaskRun pod.
//
// # stepID format
//
//	"<taskrun-name>/<container-name>"
//
// Examples:
//
//	"build-app-run-abc12/step-unit-test"
//	"lint-run-xyz99/step-lint"
//
// When stepID is empty the adapter picks the first TaskRun (alphabetical by
// name) and its first step container (step-<step-name>).
//
// # Pod resolution
//
// The actual pod name is stored in TaskRun.status.podName — not equal to the
// TaskRun name. We GET the TaskRun to resolve that field.
func (a *Adapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("tekton.StreamLogs: empty run id: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return nil, err
	}
	ns, err := a.extra.requireNamespace()
	if err != nil {
		return nil, err
	}

	// ── 1. Dynamic client to list/get TaskRuns ────────────────────────────
	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("tekton.StreamLogs: dynamic client: %w", engine.ErrUnavailable)
	}

	// ── 2. Resolve taskRunName + containerName from stepID ────────────────
	taskRunName, containerName, err := resolveStepID(ctx, dyn, ns, runID, stepID)
	if err != nil {
		return nil, err
	}

	// ── 3. Get podName from TaskRun.status.podName ────────────────────────
	trObj, err := dyn.Resource(gvrTaskRun).Namespace(ns).Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("tekton.StreamLogs: get TaskRun %s: %w", taskRunName, mapK8sError(err))
	}
	podName := readPodName(trObj.Object)
	if podName == "" {
		return nil, fmt.Errorf("tekton.StreamLogs: TaskRun %s has no podName yet: %w",
			taskRunName, engine.ErrNotFound)
	}

	// ── 4. Typed clientset to stream pod logs ─────────────────────────────
	cs, err := a.resolver.Kubernetes(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("tekton.StreamLogs: kubernetes client: %w", engine.ErrUnavailable)
	}

	logger.Info("tekton.StreamLogs: streaming",
		"run_id", runID,
		"taskrun", taskRunName,
		"container", containerName,
		"pod", podName,
		"namespace", ns,
	)

	rc, err := cs.CoreV1().Pods(ns).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		Follow:    false,
	}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("tekton.StreamLogs: pod %s container %s: %w",
			podName, containerName, mapK8sError(err))
	}
	return rc, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveStepID interprets stepID and returns (taskRunName, containerName).
//
// If stepID is non-empty it must be "<taskrun-name>/<container-name>".
// If empty, the first TaskRun (smallest name alphabetically) is chosen and
// containerName defaults to the first step container in spec.steps[].
func resolveStepID(
	ctx context.Context,
	dyn dynamic.Interface,
	ns, runID, stepID string,
) (taskRunName, containerName string, err error) {
	if stepID != "" {
		parts := strings.SplitN(stepID, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf(
				"tekton.StreamLogs: stepID %q must be \"<taskrun-name>/<container-name>\": %w",
				stepID, engine.ErrInvalidInput,
			)
		}
		return parts[0], parts[1], nil
	}

	// stepID empty → pick first TaskRun for this PipelineRun.
	list, listErr := dyn.Resource(gvrTaskRun).Namespace(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineRun=" + runID,
	})
	if listErr != nil {
		return "", "", fmt.Errorf("tekton.StreamLogs: list TaskRuns: %w", mapK8sError(listErr))
	}
	if list == nil || len(list.Items) == 0 {
		return "", "", fmt.Errorf("tekton.StreamLogs: no TaskRuns found for PipelineRun %s: %w",
			runID, engine.ErrNotFound)
	}

	// Stable selection: smallest TaskRun name (alphabetical order).
	first := &list.Items[0]
	for i := 1; i < len(list.Items); i++ {
		if list.Items[i].GetName() < first.GetName() {
			first = &list.Items[i]
		}
	}
	taskRunName = first.GetName()
	containerName = firstStepContainer(first.Object)
	if containerName == "" {
		return "", "", fmt.Errorf(
			"tekton.StreamLogs: TaskRun %s has no step containers: %w",
			taskRunName, engine.ErrNotFound,
		)
	}
	return taskRunName, containerName, nil
}

// readPodName extracts status.podName from a TaskRun Unstructured object.
func readPodName(obj map[string]any) string {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return ""
	}
	s, _ := status["podName"].(string)
	return s
}

// firstStepContainer returns "step-<name>" for the first step in spec.steps.
// Returns "" when spec.steps is absent or all entries lack a name.
func firstStepContainer(obj map[string]any) string {
	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		return ""
	}
	steps, _ := spec["steps"].([]any)
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		name, _ := step["name"].(string)
		if name != "" {
			return "step-" + name
		}
	}
	return ""
}
