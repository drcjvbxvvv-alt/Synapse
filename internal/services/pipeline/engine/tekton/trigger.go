package tekton

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Trigger creates a Tekton PipelineRun in the target cluster.
//
// The PipelineRun spec uses:
//   - spec.pipelineRef.name  = extra.PipelineName
//   - spec.params[]          = req.Variables (string params)
//   - spec.taskRunTemplate.serviceAccountName = extra.ServiceAccountName (if set)
//
// The generated PipelineRun carries labels that let Synapse filter its own
// runs from those created by other tools in the same cluster:
//   - app.kubernetes.io/managed-by = "synapse-ci-adapter"
//   - synapse.io/run-id            = <SnapshotID> (when non-zero)
//   - synapse.io/pipeline-id       = <PipelineID> (when non-zero)
//
// The returned RunID equals the generated PipelineRun metadata.name (Tekton
// does not issue an integer id). ExternalID is the same string.
func (a *Adapter) Trigger(ctx context.Context, req *engine.TriggerRequest) (*engine.TriggerResult, error) {
	if req == nil {
		return nil, fmt.Errorf("tekton.Trigger: nil request: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return nil, err
	}
	pipelineName, namespace, err := a.extra.requireTargets()
	if err != nil {
		return nil, err
	}

	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("tekton.Trigger: dynamic client: %w", engine.ErrUnavailable)
	}

	pr := buildPipelineRunUnstructured(pipelineName, namespace, a.extra.ServiceAccountName, req)

	created, err := dyn.Resource(gvrPipelineRun).Namespace(namespace).
		Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("tekton.Trigger: %w", mapK8sError(err))
	}

	name := created.GetName()
	queuedAt := time.Now().UTC()
	if t := created.GetCreationTimestamp().Time; !t.IsZero() {
		queuedAt = t.UTC()
	}
	return &engine.TriggerResult{
		RunID:      name,
		ExternalID: name,
		QueuedAt:   queuedAt,
	}, nil
}

// buildPipelineRunUnstructured constructs the `*unstructured.Unstructured`
// payload Tekton expects. Factored out for testability.
//
// Adapter-side name generation is deliberate: K8s' server-side `generateName`
// works against real clusters but is not honoured by the dynamic-client fake
// (v0.29). Generating the suffix ourselves is byte-compatible with both and
// gives callers a stable RunID immediately.
func buildPipelineRunUnstructured(pipelineName, namespace, serviceAccount string, req *engine.TriggerRequest) *unstructured.Unstructured {
	// Labels: every Synapse-managed PipelineRun carries these so operators
	// can filter them. Values must be valid label values — we stringify
	// integers via strconv and skip any that are zero.
	labels := map[string]string{
		managedByLabelKey: managedByLabelValue,
	}
	if req.SnapshotID != 0 {
		labels[synapseRunIDLabel] = strconv.FormatUint(uint64(req.SnapshotID), 10)
	}
	if req.PipelineID != 0 {
		labels[synapsePipelineIDLabel] = strconv.FormatUint(uint64(req.PipelineID), 10)
	}

	pr := &unstructured.Unstructured{Object: map[string]any{}}
	pr.SetAPIVersion(probeGroupVersion)
	pr.SetKind("PipelineRun")
	pr.SetNamespace(namespace)
	pr.SetName(generateRunName())
	pr.SetLabels(labels)

	spec := map[string]any{
		"pipelineRef": map[string]any{"name": pipelineName},
	}
	if params := convertVariablesToParams(req.Variables); len(params) > 0 {
		spec["params"] = params
	}
	if serviceAccount != "" {
		spec["taskRunTemplate"] = map[string]any{
			"serviceAccountName": serviceAccount,
		}
	}
	pr.Object["spec"] = spec
	return pr
}

// generateRunName returns a DNS-1123-valid PipelineRun name of the form
// `synapse-run-<10-hex-chars>`. Uses crypto/rand so concurrent triggers
// don't collide. 40-bit randomness is sufficient for practical collision
// avoidance at the scale of a single Tekton cluster.
func generateRunName() string {
	var buf [5]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Falls back to a time-based suffix on the astronomically unlikely
		// rand failure; still keeps the name unique within a single
		// millisecond burst.
		return "synapse-run-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "synapse-run-" + hex.EncodeToString(buf[:])
}

// convertVariablesToParams turns a map[string]string into Tekton's
// `params: [{name, value}]` form. Returns nil (not an empty slice) when
// the input map is empty so the surrounding spec doesn't serialise
// `params: []`.
func convertVariablesToParams(vars map[string]string) []any {
	if len(vars) == 0 {
		return nil
	}
	out := make([]any, 0, len(vars))
	for k, v := range vars {
		out = append(out, map[string]any{
			"name":  k,
			"value": v,
		})
	}
	return out
}
