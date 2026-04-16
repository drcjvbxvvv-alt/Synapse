package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetRun returns the current status of a Jenkins run.
//
// runID formats:
//   - "<build number>" — direct build; we fetch /job/:path/:num/api/json
//   - "queue:<id>"     — the build hasn't been scheduled yet. We poll the
//     queue endpoint once; if a build number has since materialised, we
//     fetch it; otherwise we report RunPhasePending with the queue id.
//
// Jenkins jobs are keyed by path inside Jenkins; the path lives in
// CIEngineConfig.ExtraJSON (field `job_path`) and is required.
func (a *Adapter) GetRun(ctx context.Context, runID string) (*engine.RunStatus, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("jenkins.GetRun: empty run id: %w", engine.ErrInvalidInput)
	}
	jobPath, err := a.extra.requireJobPath()
	if err != nil {
		return nil, err
	}

	// Queue item path — the build hasn't started yet when this runID was
	// issued. Probe the queue once to see whether Jenkins assigned a build
	// number in the meantime.
	if strings.HasPrefix(runID, queueRunIDPrefix) {
		return a.getRunFromQueue(ctx, jobPath, runID)
	}

	// Otherwise treat runID as a build number.
	buildNum, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("jenkins.GetRun: runID %q is not a build number: %w", runID, engine.ErrInvalidInput)
	}
	return a.getRunByBuildNumber(ctx, jobPath, buildNum)
}

// getRunFromQueue handles runIDs still referencing the queue. One poll
// attempt; if the build has started we delegate to getRunByBuildNumber.
func (a *Adapter) getRunFromQueue(ctx context.Context, jobPath, runID string) (*engine.RunStatus, error) {
	idStr := strings.TrimPrefix(runID, queueRunIDPrefix)
	queueID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("jenkins.GetRun: invalid queue id in %q: %w", runID, engine.ErrInvalidInput)
	}

	req, err := a.c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/queue/item/%d/api/json", queueID), nil)
	if err != nil {
		return nil, err
	}
	var item queueItem
	if err := a.c.doJSON(req, &item); err != nil {
		return nil, fmt.Errorf("jenkins.GetRun: poll queue %d: %w", queueID, err)
	}
	if item.Cancelled {
		return &engine.RunStatus{
			RunID:      runID,
			ExternalID: runID,
			Phase:      engine.RunPhaseCancelled,
			Raw:        "queue-cancelled",
			Message:    item.Why,
		}, nil
	}
	if item.Executable != nil && item.Executable.Number > 0 {
		return a.getRunByBuildNumber(ctx, jobPath, item.Executable.Number)
	}
	// Still queued.
	return &engine.RunStatus{
		RunID:      runID,
		ExternalID: runID,
		Phase:      engine.RunPhasePending,
		Raw:        "queued",
		Message:    item.Why,
	}, nil
}

// getRunByBuildNumber fetches /job/:path/:num/api/json.
func (a *Adapter) getRunByBuildNumber(ctx context.Context, jobPath string, buildNum int64) (*engine.RunStatus, error) {
	path := buildJobURLPath(jobPath) + "/" + strconv.FormatInt(buildNum, 10) + "/api/json"
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var b jenkinsBuild
	if err := a.c.doJSON(req, &b); err != nil {
		return nil, fmt.Errorf("jenkins.GetRun %d: %w", buildNum, err)
	}
	return buildStatusFromJenkinsBuild(&b, buildNum), nil
}

// buildStatusFromJenkinsBuild maps a jenkinsBuild into engine.RunStatus.
// Jenkins lacks a per-step breakdown in the basic /api/json payload (stages
// require additional calls); M18c does not surface Steps — the UI reads
// Phase + Raw only. A Stages-tree endpoint can be added in M18c follow-up.
func buildStatusFromJenkinsBuild(b *jenkinsBuild, buildNum int64) *engine.RunStatus {
	runIDStr := strconv.FormatInt(buildNum, 10)
	phase := mapJenkinsStatus(b.Result, b.Building)

	// Timestamps: Jenkins uses epoch ms. `timestamp` is when the build
	// entered the executor; no separate `startedAt` field. `duration` is 0
	// while building.
	var startedAt, finishedAt *time.Time
	if b.Timestamp > 0 {
		tt := time.UnixMilli(b.Timestamp).UTC()
		startedAt = &tt
		if !b.Building && b.Duration > 0 {
			ft := tt.Add(time.Duration(b.Duration) * time.Millisecond)
			finishedAt = &ft
		}
	}

	raw := b.Result
	if raw == "" {
		if b.Building {
			raw = "BUILDING"
		} else {
			raw = "PENDING"
		}
	}

	return &engine.RunStatus{
		RunID:      runIDStr,
		ExternalID: runIDStr,
		Phase:      phase,
		Raw:        raw,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}
}
