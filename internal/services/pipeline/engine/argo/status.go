package argo

import "github.com/shaia/Synapse/internal/services/pipeline/engine"

// mapArgoPhase converts Argo's `status.phase` string into the unified
// RunPhase. Argo's phases are defined in
// https://argoproj.github.io/argo-workflows/fields/#workflowstatus.
//
// Note that Argo distinguishes "Failed" (pipeline logic decided to fail)
// from "Error" (infrastructure-level failure). Both surface as
// RunPhaseFailed since callers treat them equivalently for gating.
func mapArgoPhase(phase string) engine.RunPhase {
	switch phase {
	case argoPhasePending:
		return engine.RunPhasePending
	case argoPhaseRunning:
		return engine.RunPhaseRunning
	case argoPhaseSucceeded:
		return engine.RunPhaseSuccess
	case argoPhaseFailed, argoPhaseError:
		return engine.RunPhaseFailed
	case "":
		// No phase yet — freshly created Workflow.
		return engine.RunPhasePending
	default:
		return engine.RunPhaseUnknown
	}
}
