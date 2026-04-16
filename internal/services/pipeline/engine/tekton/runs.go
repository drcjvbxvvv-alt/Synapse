package tekton

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetRun reads a PipelineRun's status plus its child TaskRuns and composes
// an engine.RunStatus. The runID is the PipelineRun's metadata.name
// (produced by Trigger).
//
// Flow:
//  1. Get PipelineRun/:runID in the configured namespace.
//  2. Extract the Succeeded condition → RunPhase + Raw + Message.
//  3. List TaskRuns filtered by label tekton.dev/pipelineRun=<runID>; each
//     becomes a StepStatus. If the list fails we fall back to an empty
//     Steps slice — the UI still gets the pipeline-level status
//     (Observer Pattern per CLAUDE §8).
func (a *Adapter) GetRun(ctx context.Context, runID string) (*engine.RunStatus, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("tekton.GetRun: empty run id: %w", engine.ErrInvalidInput)
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
		return nil, fmt.Errorf("tekton.GetRun: dynamic client: %w", engine.ErrUnavailable)
	}

	pr, err := dyn.Resource(gvrPipelineRun).Namespace(ns).
		Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("tekton.GetRun %s: %w", runID, mapK8sError(err))
	}

	rs := buildRunStatusFromPipelineRun(runID, pr)

	// Best-effort list of child TaskRuns; failures go into Raw as a hint
	// instead of aborting the whole call.
	trList, listErr := dyn.Resource(gvrTaskRun).Namespace(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineRun=" + runID,
	})
	if listErr != nil {
		rs.Raw += " (taskruns unavailable: " + listErr.Error() + ")"
		return rs, nil
	}
	rs.Steps = stepsFromTaskRuns(trList)
	return rs, nil
}

// buildRunStatusFromPipelineRun interprets a PipelineRun's status block.
// Exported lowercase (package-internal) helper so Trigger() can reuse it
// in the future without refactoring.
func buildRunStatusFromPipelineRun(runID string, pr *unstructured.Unstructured) *engine.RunStatus {
	cond := readSucceededCondition(pr.Object, "status")
	phase := mapTektonStatus(cond)
	started, finished := readTimes(pr.Object)

	raw := cond.Status
	if cond.Reason != "" {
		if raw == "" {
			raw = cond.Reason
		} else {
			raw = cond.Status + "/" + cond.Reason
		}
	}
	message := readConditionMessage(pr.Object)

	return &engine.RunStatus{
		RunID:      runID,
		ExternalID: runID,
		Phase:      phase,
		Raw:        raw,
		Message:    message,
		StartedAt:  started,
		FinishedAt: finished,
	}
}

// stepsFromTaskRuns turns a list of TaskRun Unstructured objects into
// engine.StepStatus entries.
func stepsFromTaskRuns(list *unstructured.UnstructuredList) []engine.StepStatus {
	if list == nil || len(list.Items) == 0 {
		return []engine.StepStatus{}
	}
	out := make([]engine.StepStatus, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		cond := readSucceededCondition(item.Object, "status")
		start, finish := readTimes(item.Object)
		raw := cond.Status
		if cond.Reason != "" {
			if raw == "" {
				raw = cond.Reason
			} else {
				raw = cond.Status + "/" + cond.Reason
			}
		}
		out = append(out, engine.StepStatus{
			Name:       readStepName(item),
			Phase:      mapTektonStatus(cond),
			Raw:        raw,
			StartedAt:  start,
			FinishedAt: finish,
		})
	}
	return out
}

// readStepName prefers the `tekton.dev/pipelineTask` label when available
// (the logical task name from the Pipeline spec), falling back to the
// TaskRun metadata.name otherwise.
func readStepName(tr *unstructured.Unstructured) string {
	if labels := tr.GetLabels(); labels != nil {
		if v := labels["tekton.dev/pipelineTask"]; v != "" {
			return v
		}
	}
	return tr.GetName()
}

// ---------------------------------------------------------------------------
// Small field extractors over unstructured.Unstructured.
// ---------------------------------------------------------------------------
//
// Intentionally do not use unstructured.NestedXxx helpers for readability;
// the status shape is fixed enough that direct assertions are fine and
// preserve nil-safety cleanly.

// readSucceededCondition walks obj["status"]["conditions"][] and returns
// the one with type=Succeeded.
func readSucceededCondition(obj map[string]any, statusKey string) succeededCondition {
	status, _ := obj[statusKey].(map[string]any)
	if status == nil {
		return succeededCondition{}
	}
	conds, _ := status["conditions"].([]any)
	for _, c := range conds {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := cm["type"].(string); t == conditionTypeSucceeded {
			s, _ := cm["status"].(string)
			r, _ := cm["reason"].(string)
			return succeededCondition{Status: s, Reason: r}
		}
	}
	return succeededCondition{}
}

// readConditionMessage returns the Succeeded condition's Message field (the
// reason string is already in Raw; Message is a free-form human note).
func readConditionMessage(obj map[string]any) string {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return ""
	}
	conds, _ := status["conditions"].([]any)
	for _, c := range conds {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := cm["type"].(string); t == conditionTypeSucceeded {
			m, _ := cm["message"].(string)
			return m
		}
	}
	return ""
}

// readTimes pulls status.startTime / status.completionTime from a
// PipelineRun or TaskRun object.
func readTimes(obj map[string]any) (started, finished *time.Time) {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return nil, nil
	}
	if s, ok := status["startTime"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			utc := t.UTC()
			started = &utc
		}
	}
	if s, ok := status["completionTime"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			utc := t.UTC()
			finished = &utc
		}
	}
	return started, finished
}
