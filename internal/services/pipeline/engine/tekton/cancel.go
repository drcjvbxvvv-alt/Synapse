package tekton

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Cancel stops a running PipelineRun by patching spec.status.
//
// Tekton's cancellation model: set `spec.status = "Cancelled"` and the
// controller gracefully stops child TaskRuns. Already-terminal runs ignore
// the patch; the adapter translates "attempting to cancel a terminal run"
// into engine.ErrAlreadyTerminal so callers get the same semantics as
// GitLab/Jenkins adapters.
//
// A JSON-merge PATCH is used (as opposed to strategic merge) because the
// dynamic client lacks schema knowledge for Tekton CRDs.
func (a *Adapter) Cancel(ctx context.Context, runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("tekton.Cancel: empty run id: %w", engine.ErrInvalidInput)
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
		return fmt.Errorf("tekton.Cancel: dynamic client: %w", engine.ErrUnavailable)
	}

	// Fast-path terminality check. Skipping straight to PATCH would also
	// work but Tekton does not return a distinct error for "already
	// terminal" — we'd have to re-GET anyway to know the final state.
	cur, err := dyn.Resource(gvrPipelineRun).Namespace(ns).
		Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("tekton.Cancel %s: %w", runID, mapK8sError(err))
	}
	phase := mapTektonStatus(readSucceededCondition(cur.Object, "status"))
	if phase.IsTerminal() {
		return fmt.Errorf("tekton.Cancel %s: phase=%s: %w", runID, phase, engine.ErrAlreadyTerminal)
	}

	patch := []byte(`{"spec":{"status":"` + cancelSpecStatus + `"}}`)
	_, err = dyn.Resource(gvrPipelineRun).Namespace(ns).
		Patch(ctx, runID, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("tekton.Cancel %s: %w", runID, mapK8sError(err))
	}
	return nil
}
