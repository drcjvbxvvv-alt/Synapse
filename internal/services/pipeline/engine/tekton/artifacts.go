package tekton

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetArtifacts returns the PipelineRun's `pipelineResults` as engine.Artifacts.
//
// Tekton does not have a first-class "artifact" concept like GitLab or
// Jenkins — the closest equivalents are:
//
//   - `pipelineResults[]` on the PipelineRun (typed name/value pairs
//     surfaced by the Pipeline's `results` declarations)
//   - Files written to shared Workspaces (PVC / ConfigMap / Secret) —
//     opaque to the Tekton API itself
//
// M18d surfaces pipelineResults as engine.Artifact entries with `Kind="result"`;
// workspace-based files are out of scope. Each entry carries the result
// name, its string value, and the PipelineRun's completionTime.
//
// Returns an empty slice (not nil, not error) when the run has no results
// — matching the contract other adapters use for the UI's empty state.
func (a *Adapter) GetArtifacts(ctx context.Context, runID string) ([]*engine.Artifact, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("tekton.GetArtifacts: empty run id: %w", engine.ErrInvalidInput)
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
		return nil, fmt.Errorf("tekton.GetArtifacts: dynamic client: %w", engine.ErrUnavailable)
	}

	pr, err := dyn.Resource(gvrPipelineRun).Namespace(ns).Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("tekton.GetArtifacts %s: %w", runID, mapK8sError(err))
	}

	results := extractPipelineResults(pr.Object)
	if len(results) == 0 {
		return []*engine.Artifact{}, nil
	}

	// Use completionTime when available; otherwise fall back to startTime
	// or "now" so the UI always has a timestamp.
	var createdAt time.Time
	_, finished := readTimes(pr.Object)
	if finished != nil {
		createdAt = *finished
	} else if started, _ := readTimes(pr.Object); started != nil {
		createdAt = *started
	} else {
		createdAt = time.Now().UTC()
	}

	out := make([]*engine.Artifact, 0, len(results))
	for _, r := range results {
		out = append(out, &engine.Artifact{
			Name:      r.name,
			Kind:      "result",
			Digest:    r.value, // repurpose Digest to carry the raw value string
			CreatedAt: createdAt,
		})
	}
	return out, nil
}

// pipelineResult is a minimal projection of a single entry in
// status.pipelineResults[].
type pipelineResult struct {
	name  string
	value string
}

// extractPipelineResults walks status.pipelineResults[] and returns the
// flattened name/value pairs. The value field in Tekton can be a string, a
// string slice, or an object; M18d surfaces only the string form
// (stringifying nested shapes is a follow-up if users ask for it).
func extractPipelineResults(obj map[string]any) []pipelineResult {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return nil
	}
	list, _ := status["pipelineResults"].([]any)
	if len(list) == 0 {
		// Tekton v1 renamed the field to just `results` in some revisions;
		// accept both for forward-compatibility.
		list, _ = status["results"].([]any)
	}
	if len(list) == 0 {
		return nil
	}
	out := make([]pipelineResult, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		val, _ := m["value"].(string)
		out = append(out, pipelineResult{name: name, value: val})
	}
	return out
}
