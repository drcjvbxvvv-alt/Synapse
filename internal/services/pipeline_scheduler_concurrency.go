package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Concurrency counting
// ---------------------------------------------------------------------------

type concurrencyCounts struct {
	system   int
	cluster  map[uint]int
	pipeline map[uint]int
}

func (s *PipelineScheduler) loadConcurrencyCounts(ctx context.Context) (*concurrencyCounts, error) {
	counts := &concurrencyCounts{
		cluster:  make(map[uint]int),
		pipeline: make(map[uint]int),
	}

	var runningRuns []struct {
		PipelineID uint
		ClusterID  uint
	}
	if err := s.db.WithContext(ctx).
		Model(&models.PipelineRun{}).
		Select("pipeline_id, cluster_id").
		Where("status = ?", models.PipelineRunStatusRunning).
		Find(&runningRuns).Error; err != nil {
		return nil, fmt.Errorf("count running runs: %w", err)
	}

	counts.system = len(runningRuns)
	for _, r := range runningRuns {
		counts.cluster[r.ClusterID]++
		counts.pipeline[r.PipelineID]++
	}
	return counts, nil
}

func (s *PipelineScheduler) canSchedule(ctx context.Context, run *models.PipelineRun, counts *concurrencyCounts) bool {
	if counts.system >= s.cfg.SystemMaxRuns {
		return false
	}
	if counts.cluster[run.ClusterID] >= s.cfg.ClusterMaxRuns {
		return false
	}
	pipelineMax := s.getPipelineMaxConcurrent(ctx, run.PipelineID)
	if counts.pipeline[run.PipelineID] >= pipelineMax {
		return false
	}
	return true
}

func (s *PipelineScheduler) getPipelineMaxConcurrent(ctx context.Context, pipelineID uint) int {
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).Select("max_concurrent_runs").First(&pipeline, pipelineID).Error; err != nil {
		return 1 // fallback
	}
	if pipeline.MaxConcurrentRuns <= 0 {
		return 1
	}
	return pipeline.MaxConcurrentRuns
}

// ---------------------------------------------------------------------------
// Concurrency Group 策略
// ---------------------------------------------------------------------------

func (s *PipelineScheduler) applyConcurrencyPolicy(ctx context.Context, newRun *models.PipelineRun) error {
	var activeRuns []models.PipelineRun
	if err := s.db.WithContext(ctx).
		Where("concurrency_group = ? AND status IN ?",
			newRun.ConcurrencyGroup,
			[]string{models.PipelineRunStatusRunning, models.PipelineRunStatusQueued},
		).
		Order("queued_at ASC").
		Find(&activeRuns).Error; err != nil {
		return fmt.Errorf("find active runs in group %s: %w", newRun.ConcurrencyGroup, err)
	}

	if len(activeRuns) == 0 {
		return nil
	}

	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).
		Select("concurrency_policy").
		First(&pipeline, newRun.PipelineID).Error; err != nil {
		return fmt.Errorf("get pipeline concurrency policy: %w", err)
	}

	switch pipeline.ConcurrencyPolicy {
	case models.ConcurrencyPolicyCancelPrevious:
		for i := range activeRuns {
			old := &activeRuns[i]
			if old.Status == models.PipelineRunStatusRunning {
				old.Status = models.PipelineRunStatusCancelling
			} else {
				now := time.Now()
				old.Status = models.PipelineRunStatusCancelled
				old.FinishedAt = &now
			}
			if err := s.db.WithContext(ctx).Save(old).Error; err != nil {
				logger.Error("failed to cancel previous run",
					"run_id", old.ID, "error", err)
			}
			logger.Info("previous run cancelled by concurrency policy",
				"cancelled_run_id", old.ID,
				"new_run_pipeline_id", newRun.PipelineID,
				"group", newRun.ConcurrencyGroup,
			)
		}

	case models.ConcurrencyPolicyQueue:
		// 不做任何處理，讓新 Run 排隊等待

	case models.ConcurrencyPolicyReject:
		newRun.Status = models.PipelineRunStatusRejected
		newRun.Error = fmt.Sprintf("rejected: concurrency group %q already has active runs", newRun.ConcurrencyGroup)
		logger.Info("pipeline run rejected by concurrency policy",
			"pipeline_id", newRun.PipelineID,
			"group", newRun.ConcurrencyGroup,
		)

	default:
		logger.Warn("unknown concurrency policy, falling back to cancel_previous",
			"policy", pipeline.ConcurrencyPolicy)
	}

	return nil
}
