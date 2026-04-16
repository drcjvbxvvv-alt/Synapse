package argo

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

// Trigger creates an Argo Workflow that references the configured
// WorkflowTemplate.
//
// Spec shape (minimal):
//
//	apiVersion: argoproj.io/v1alpha1
//	kind: Workflow
//	metadata:
//	  name: synapse-run-<hex>
//	  namespace: <extra.Namespace>
//	  labels:
//	    app.kubernetes.io/managed-by: synapse-ci-adapter
//	    synapse.io/run-id: "<SnapshotID>"
//	    synapse.io/pipeline-id: "<PipelineID>"
//	spec:
//	  workflowTemplateRef:
//	    name: <extra.WorkflowTemplateName>
//	  arguments:
//	    parameters:
//	      - {name: "...", value: "..."}
//	  serviceAccountName: <extra.ServiceAccountName>  # if set
//
// Adapter-side name generation mirrors Tekton's trigger.go: avoids both
// the fake-client limitation and an extra round-trip to pick up the
// server-assigned name.
func (a *Adapter) Trigger(ctx context.Context, req *engine.TriggerRequest) (*engine.TriggerResult, error) {
	if req == nil {
		return nil, fmt.Errorf("argo.Trigger: nil request: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return nil, err
	}
	templateName, namespace, err := a.extra.requireTargets()
	if err != nil {
		return nil, err
	}
	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("argo.Trigger: dynamic client: %w", engine.ErrUnavailable)
	}

	wf := buildWorkflowUnstructured(templateName, namespace, a.extra.ServiceAccountName, req)
	created, err := dyn.Resource(gvrWorkflow).Namespace(namespace).
		Create(ctx, wf, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("argo.Trigger: %w", mapK8sError(err))
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

// buildWorkflowUnstructured constructs the Workflow payload. Factored out
// for testability; side-effect free.
func buildWorkflowUnstructured(templateName, namespace, serviceAccount string, req *engine.TriggerRequest) *unstructured.Unstructured {
	labels := map[string]string{
		managedByLabelKey: managedByLabelValue,
	}
	if req.SnapshotID != 0 {
		labels[synapseRunIDLabel] = strconv.FormatUint(uint64(req.SnapshotID), 10)
	}
	if req.PipelineID != 0 {
		labels[synapsePipelineIDLabel] = strconv.FormatUint(uint64(req.PipelineID), 10)
	}

	wf := &unstructured.Unstructured{Object: map[string]any{}}
	wf.SetAPIVersion(probeGroupVersion)
	wf.SetKind("Workflow")
	wf.SetNamespace(namespace)
	wf.SetName(generateRunName())
	wf.SetLabels(labels)

	spec := map[string]any{
		"workflowTemplateRef": map[string]any{"name": templateName},
	}
	if args := buildArguments(req.Variables); args != nil {
		spec["arguments"] = args
	}
	if serviceAccount != "" {
		spec["serviceAccountName"] = serviceAccount
	}
	wf.Object["spec"] = spec
	return wf
}

// buildArguments converts the flat variables map into Argo's parameters
// form:
//
//	arguments:
//	  parameters:
//	    - {name, value}
//
// Returns nil (not an empty struct) when there are no variables so the
// surrounding spec doesn't serialise `arguments: {}`.
func buildArguments(vars map[string]string) map[string]any {
	if len(vars) == 0 {
		return nil
	}
	params := make([]any, 0, len(vars))
	for k, v := range vars {
		params = append(params, map[string]any{
			"name":  k,
			"value": v,
		})
	}
	return map[string]any{"parameters": params}
}

// generateRunName returns a DNS-1123-compliant Workflow name with a
// cryptographically random suffix so concurrent triggers don't collide.
func generateRunName() string {
	var buf [5]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "synapse-run-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "synapse-run-" + hex.EncodeToString(buf[:])
}
