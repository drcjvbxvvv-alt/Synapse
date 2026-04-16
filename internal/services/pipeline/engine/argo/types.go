// Package argo implements the CI engine adapter for Argo Workflows (M18e).
//
// Like Tekton, Argo Workflows is a Kubernetes-native CI/CD system. The
// adapter reaches a Synapse-managed cluster via a dynamic.Interface and
// creates `Workflow` custom resources. The two adapters share the same
// architectural pattern (ClusterResolver injection, Observer-pattern
// IsAvailable, status-condition interpretation) but target different
// CRDs.
//
// Target resource:
//   - argoproj.io/v1alpha1 Workflow   (one per run)
package argo

import "k8s.io/apimachinery/pkg/runtime/schema"

// ---------------------------------------------------------------------------
// Group / Version / Resource constants
// ---------------------------------------------------------------------------

const argoGroup = "argoproj.io"

// gvrWorkflow is the primary resource the adapter creates. Argo also has
// WorkflowTemplate, CronWorkflow, ClusterWorkflowTemplate — out of scope
// for M18e; callers reference a WorkflowTemplate via spec.workflowTemplateRef.
var gvrWorkflow = schema.GroupVersionResource{
	Group:    argoGroup,
	Version:  "v1alpha1",
	Resource: "workflows",
}

// probeGroupVersion is what the Discovery API is queried against to detect
// whether Argo Workflows is installed in the target cluster.
const probeGroupVersion = argoGroup + "/v1alpha1"

// ---------------------------------------------------------------------------
// Argo Workflow status phases
// ---------------------------------------------------------------------------
//
// Reference: https://argoproj.github.io/argo-workflows/fields/#workflow-status
//
// Argo uses a plain string phase field (no knative conditions[] indirection
// like Tekton). Values we handle explicitly; anything else maps to
// RunPhaseUnknown for diagnostic visibility.
const (
	argoPhasePending   = "Pending"
	argoPhaseRunning   = "Running"
	argoPhaseSucceeded = "Succeeded"
	argoPhaseFailed    = "Failed"
	argoPhaseError     = "Error"
)

// ---------------------------------------------------------------------------
// Spec shutdown values (Cancel)
// ---------------------------------------------------------------------------
//
// Argo offers two shutdown modes:
//   - "Terminate": immediately fail running pods; no exit handler runs
//   - "Stop":      run exit handlers gracefully
//
// M18e uses "Terminate" for symmetry with other adapters' Cancel semantics
// (equivalent to Tekton's Cancelled / GitLab's canceled / Jenkins' aborted).
const shutdownTerminate = "Terminate"

// ---------------------------------------------------------------------------
// Labels tagging Synapse-managed workflows
// ---------------------------------------------------------------------------

const (
	managedByLabelKey      = "app.kubernetes.io/managed-by"
	managedByLabelValue    = "synapse-ci-adapter"
	synapseRunIDLabel      = "synapse.io/run-id"
	synapsePipelineIDLabel = "synapse.io/pipeline-id"
)
