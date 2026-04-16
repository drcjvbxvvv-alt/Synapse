package jenkins

import "github.com/shaia/Synapse/internal/services/pipeline/engine"

// mapJenkinsStatus converts a Jenkins build's (result, building) pair into
// the unified RunPhase contract.
//
// Jenkins fields:
//   - Building: true  → run is in progress (result is null)
//   - Building: false + Result "SUCCESS"  → success
//   - Building: false + Result "FAILURE" / "UNSTABLE" → failed
//   - Building: false + Result "ABORTED"  → cancelled
//   - Building: false + Result ""         → the build has not started yet
//     (still in queue); treated as pending
//
// UNSTABLE is mapped to RunPhaseFailed for safety: the build completed but
// tests failed, which for most gating workflows should NOT be treated as
// success. Adapters may later provide a knob to reclassify.
func mapJenkinsStatus(result string, building bool) engine.RunPhase {
	if building {
		return engine.RunPhaseRunning
	}
	switch result {
	case "SUCCESS":
		return engine.RunPhaseSuccess
	case "FAILURE", "UNSTABLE":
		return engine.RunPhaseFailed
	case "ABORTED":
		return engine.RunPhaseCancelled
	case "":
		// No result yet — still queued / preparing.
		return engine.RunPhasePending
	default:
		return engine.RunPhaseUnknown
	}
}
