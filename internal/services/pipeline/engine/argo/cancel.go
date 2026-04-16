package argo

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Cancel terminates a Workflow by patching spec.shutdown = "Terminate".
//
// Argo interprets spec.shutdown on in-flight workflows and signals the
// controller to stop running steps. Already-terminal workflows silently
// ignore the patch; the adapter pre-checks status.phase so it can surface
// ErrAlreadyTerminal with the same semantics as other adapters.
func (a *Adapter) Cancel(ctx context.Context, runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("argo.Cancel: empty run id: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return err
	}
	ns, err := a.extra.requireNamespace()
	if err != nil {
		return err
	}
	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return fmt.Errorf("argo.Cancel: dynamic client: %w", engine.ErrUnavailable)
	}

	// Pre-check: if the Workflow is already terminal, there's nothing to
	// cancel. Save the PATCH round-trip and return ErrAlreadyTerminal.
	cur, err := dyn.Resource(gvrWorkflow).Namespace(ns).Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("argo.Cancel %s: %w", runID, mapK8sError(err))
	}
	phase := mapArgoPhase(readArgoPhase(cur.Object))
	if phase.IsTerminal() {
		return fmt.Errorf("argo.Cancel %s: phase=%s: %w", runID, phase, engine.ErrAlreadyTerminal)
	}

	patch := []byte(`{"spec":{"shutdown":"` + shutdownTerminate + `"}}`)
	if _, err := dyn.Resource(gvrWorkflow).Namespace(ns).
		Patch(ctx, runID, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("argo.Cancel %s: %w", runID, mapK8sError(err))
	}
	return nil
}
