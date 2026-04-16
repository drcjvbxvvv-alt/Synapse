package github

import "github.com/shaia/Synapse/internal/services/pipeline/engine"

// mapGitHubStatus converts GitHub's (status, conclusion) pair into the
// unified RunPhase.
//
// GitHub fields:
//   - status ∈ {queued, in_progress, completed, requested, waiting, pending}
//   - conclusion (populated when status==completed) ∈
//     {success, failure, cancelled, skipped, timed_out, action_required,
//      neutral, stale, startup_failure}
//
// Mapping rules:
//   - status != completed   → Pending/Running based on status
//   - status == completed:
//       success              → Success
//       failure / timed_out  → Failed
//       cancelled / skipped  → Cancelled
//       action_required      → Pending (awaiting manual approval)
//       neutral / stale / empty / others → Unknown
func mapGitHubStatus(status, conclusion string) engine.RunPhase {
	switch status {
	case "queued", "requested", "waiting", "pending":
		return engine.RunPhasePending
	case "in_progress":
		return engine.RunPhaseRunning
	case "completed":
		switch conclusion {
		case "success":
			return engine.RunPhaseSuccess
		case "failure", "timed_out", "startup_failure":
			return engine.RunPhaseFailed
		case "cancelled", "skipped":
			return engine.RunPhaseCancelled
		case "action_required":
			return engine.RunPhasePending
		case "neutral", "stale", "":
			return engine.RunPhaseUnknown
		default:
			return engine.RunPhaseUnknown
		}
	case "":
		return engine.RunPhasePending
	default:
		return engine.RunPhaseUnknown
	}
}
