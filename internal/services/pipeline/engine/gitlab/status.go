package gitlab

import "github.com/shaia/Synapse/internal/services/pipeline/engine"

// mapGitLabStatus converts GitLab's pipeline / job state string to the
// unified RunPhase contract. Unknown values map to RunPhaseUnknown so the UI
// can surface a diagnostic indicator rather than silently treating them as
// "pending".
//
// GitLab states reference:
// https://docs.gitlab.com/ee/api/pipelines.html
//
// States:
//   - created / waiting_for_resource / preparing / pending / scheduled / manual
//     → RunPhasePending
//   - running                                                                   → RunPhaseRunning
//   - success                                                                   → RunPhaseSuccess
//   - failed                                                                    → RunPhaseFailed
//   - canceled / skipped                                                        → RunPhaseCancelled
//   - (anything else)                                                           → RunPhaseUnknown
func mapGitLabStatus(raw string) engine.RunPhase {
	switch raw {
	case "created", "waiting_for_resource", "preparing", "pending", "scheduled", "manual":
		return engine.RunPhasePending
	case "running":
		return engine.RunPhaseRunning
	case "success":
		return engine.RunPhaseSuccess
	case "failed":
		return engine.RunPhaseFailed
	case "canceled", "skipped":
		return engine.RunPhaseCancelled
	default:
		return engine.RunPhaseUnknown
	}
}
