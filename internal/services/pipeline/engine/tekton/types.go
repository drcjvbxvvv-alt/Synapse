// Package tekton implements the CI engine adapter for Tekton Pipelines (M18d).
//
// Unlike the GitLab and Jenkins adapters which speak HTTP REST APIs, Tekton
// is a Kubernetes-native system that exposes PipelineRun / TaskRun as Custom
// Resources. The adapter therefore reaches into a target Synapse-managed
// cluster via a dynamic.Interface and manipulates CRDs directly.
//
// Target cluster resolution:
//   - CIEngineConfig.ClusterID points to a Synapse-managed cluster (the
//     cluster where Tekton is installed).
//   - The adapter receives a ClusterResolver at construction time (see
//     cluster.go) that maps ClusterID → dynamic.Interface + discovery.
//
// Resource naming:
//   - CIEngineConfig.ExtraJSON carries the Pipeline name and the namespace
//     in which to create PipelineRun. See config.go.
package tekton

import "k8s.io/apimachinery/pkg/runtime/schema"

// ---------------------------------------------------------------------------
// Group / Version / Resource constants
// ---------------------------------------------------------------------------
//
// M18d targets the stable `tekton.dev/v1` API. The adapter detects its
// availability via the cluster's discovery API; installations still on
// `tekton.dev/v1beta1` report IsAvailable()=false.

const tektonGroup = "tekton.dev"

// PipelineRun GVR — the primary resource the adapter creates.
var gvrPipelineRun = schema.GroupVersionResource{
	Group:    tektonGroup,
	Version:  "v1",
	Resource: "pipelineruns",
}

// TaskRun GVR — children of a PipelineRun; one per task execution, used to
// build the per-step breakdown in engine.RunStatus.Steps.
var gvrTaskRun = schema.GroupVersionResource{
	Group:    tektonGroup,
	Version:  "v1",
	Resource: "taskruns",
}

// probeResource is the canonical GroupVersion the adapter checks to decide
// whether Tekton is installed. ServerResourcesForGroupVersion on this value
// returns a 200 when the CRDs are present.
const probeGroupVersion = tektonGroup + "/v1"

// ---------------------------------------------------------------------------
// Cancel spec patches
// ---------------------------------------------------------------------------
//
// Tekton cancels a PipelineRun by setting `spec.status` to one of the values
// below via PATCH. "Cancelled" is graceful (current TaskRuns run to
// completion). "CancelledRunFinally" runs `finally{}` tasks. M18d uses the
// plain "Cancelled" form.
const cancelSpecStatus = "Cancelled"

// ---------------------------------------------------------------------------
// Managed-by label — tags every PipelineRun this adapter creates so that
// diagnostics can filter "Synapse-originated" runs.
// ---------------------------------------------------------------------------

const (
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "synapse-ci-adapter"

	// synapseRunIDLabel carries the Synapse pipeline run id (Adapter
	// receives it via TriggerRequest.SnapshotID; callers passing zero get
	// no label).
	synapseRunIDLabel = "synapse.io/run-id"

	// synapsePipelineIDLabel carries the Synapse Pipeline.ID for filtering
	// / dashboards.
	synapsePipelineIDLabel = "synapse.io/pipeline-id"
)

// ---------------------------------------------------------------------------
// Tekton status condition type
// ---------------------------------------------------------------------------
//
// Tekton uses knative's `Conditions[]` pattern; the `Succeeded` condition
// is the canonical rollup for PipelineRun / TaskRun status.
const conditionTypeSucceeded = "Succeeded"

// Tekton uses these status condition reasons for cancelled runs.
const (
	reasonCancelled         = "Cancelled"
	reasonPipelineCancelled = "PipelineRunCancelled"
)
