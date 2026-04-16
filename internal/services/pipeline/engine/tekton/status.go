package tekton

import (
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Status interpretation
// ---------------------------------------------------------------------------
//
// Tekton uses knative conditions: a PipelineRun carries
// `.status.conditions[]` with the canonical `Succeeded` entry. Each entry
// has `status` (True | False | Unknown) and `reason`.
//
// Mapping table:
//
//   Succeeded=True            → RunPhaseSuccess
//   Succeeded=False,Reason=Cancelled | PipelineRunCancelled
//                             → RunPhaseCancelled
//   Succeeded=False,Reason=<other>  → RunPhaseFailed
//   Succeeded=Unknown,Reason=Running | Pending | Started | Resolving
//                             → RunPhaseRunning / RunPhasePending
//   (no condition yet)        → RunPhasePending
//
// The distinction between Pending and Running in the Unknown branch is
// taken from the reason string — "Pending" / "PipelineRunPending" maps to
// pending, everything else (including empty) to running.

// succeededCondition represents the fields mapPipelineRunStatus cares
// about; keeping it as a plain struct makes the mapping unit-testable
// without unstructured plumbing.
type succeededCondition struct {
	Status string // "True" | "False" | "Unknown" | "" (not yet present)
	Reason string
}

// mapTektonStatus converts a Tekton PipelineRun's Succeeded condition into
// the unified RunPhase. Callers that couldn't locate the condition should
// pass a zero-value succeededCondition; the function handles that as
// RunPhasePending.
func mapTektonStatus(cond succeededCondition) engine.RunPhase {
	switch cond.Status {
	case "True":
		return engine.RunPhaseSuccess
	case "False":
		if cond.Reason == reasonCancelled || cond.Reason == reasonPipelineCancelled {
			return engine.RunPhaseCancelled
		}
		return engine.RunPhaseFailed
	case "Unknown":
		// Tekton uses a handful of reason strings to signal "still going".
		switch cond.Reason {
		case "Pending", "PipelineRunPending":
			return engine.RunPhasePending
		default:
			return engine.RunPhaseRunning
		}
	case "":
		return engine.RunPhasePending
	}
	return engine.RunPhaseUnknown
}
