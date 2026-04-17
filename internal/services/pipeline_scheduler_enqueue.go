package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// TriggerRunInput — EnqueueRunInEnvironment 觸發參數
// ---------------------------------------------------------------------------

// TriggerRunInput 攜帶 EnqueueRunInEnvironment 所需的觸發參數。
type TriggerRunInput struct {
	PipelineID  uint
	EnvID       uint   // 執行目標 Environment ID（叢集 + Namespace 從此解析）
	VersionID   *uint  // 指定版本（nil → 使用 Pipeline.CurrentVersionID）
	UserID      uint
	TriggerType string // TriggerTypeManual / TriggerTypeWebhook / TriggerTypeCron
	Payload     string // webhook payload hash 等（可留空）
	RerunFromID *uint  // 若為 rerun，指向原始 Run ID
}

// ---------------------------------------------------------------------------
// EnqueueRun — 建立新 Run 並處理 Concurrency Group
// ---------------------------------------------------------------------------

// EnqueueRun 建立 PipelineRun 並根據並發策略處理 Concurrency Group。
func (s *PipelineScheduler) EnqueueRun(ctx context.Context, run *models.PipelineRun) error {
	// 設定初始狀態
	run.Status = models.PipelineRunStatusQueued
	run.QueuedAt = time.Now()

	// 佇列滿檢查（飢餓預防）
	var queuedCount int64
	if err := s.db.WithContext(ctx).Model(&models.PipelineRun{}).
		Where("status = ?", models.PipelineRunStatusQueued).
		Count(&queuedCount).Error; err != nil {
		return fmt.Errorf("count queued runs: %w", err)
	}
	if int(queuedCount) >= s.cfg.SystemMaxRuns*s.cfg.QueueOverflowRatio {
		run.Status = models.PipelineRunStatusRejected
		run.Error = "queue overflow: too many queued runs, retry later"
		if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
			return fmt.Errorf("create rejected run: %w", err)
		}
		logger.Warn("pipeline run rejected due to queue overflow",
			"pipeline_id", run.PipelineID,
			"queued_count", queuedCount,
		)
		return nil
	}

	// Concurrency Group 策略
	if run.ConcurrencyGroup != "" {
		if err := s.applyConcurrencyPolicy(ctx, run); err != nil {
			return err
		}
		// reject 策略可能已把 status 改為 rejected
		if run.Status == models.PipelineRunStatusRejected {
			if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
				return fmt.Errorf("create rejected run: %w", err)
			}
			return nil
		}
	}

	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create pipeline run: %w", err)
	}

	logger.Info("pipeline run enqueued",
		"run_id", run.ID,
		"pipeline_id", run.PipelineID,
		"concurrency_group", run.ConcurrencyGroup,
		"status", run.Status,
	)
	return nil
}

// ---------------------------------------------------------------------------
// EnqueueRunInEnvironment — 以 Environment 為執行目標建立 Run
// ---------------------------------------------------------------------------

// EnqueueRunInEnvironment 根據 Environment 解析目標叢集與 Namespace，建立並排入 PipelineRun。
// 這是 Environment-based trigger 的統一入口，由 HTTP handler 呼叫。
func (s *PipelineScheduler) EnqueueRunInEnvironment(ctx context.Context, envSvc *EnvironmentService, input TriggerRunInput) (*models.PipelineRun, error) {
	// 解析 Environment → ClusterID + Namespace
	env, err := envSvc.GetEnvironment(ctx, input.EnvID)
	if err != nil {
		return nil, fmt.Errorf("resolve environment %d: %w", input.EnvID, err)
	}

	// 取得 Pipeline 設定
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).Select("id, current_version_id, concurrency_group, max_concurrent_runs").
		First(&pipeline, input.PipelineID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pipeline %d not found", input.PipelineID)
		}
		return nil, fmt.Errorf("get pipeline %d: %w", input.PipelineID, err)
	}

	// 決定版本
	snapshotID := pipeline.CurrentVersionID
	if input.VersionID != nil {
		snapshotID = input.VersionID
	}
	if snapshotID == nil {
		return nil, fmt.Errorf("pipeline %d has no active version; create a version first", input.PipelineID)
	}

	run := &models.PipelineRun{
		PipelineID:       pipeline.ID,
		SnapshotID:       *snapshotID,
		ClusterID:        env.ClusterID,
		Namespace:        env.Namespace,
		TriggerType:      input.TriggerType,
		TriggerPayload:   input.Payload,
		TriggeredByUser:  input.UserID,
		ConcurrencyGroup: pipeline.ConcurrencyGroup,
		RerunFromID:      input.RerunFromID,
	}

	if err := s.EnqueueRun(ctx, run); err != nil {
		return nil, err
	}

	logger.Info("pipeline run enqueued via environment",
		"run_id", run.ID,
		"pipeline_id", input.PipelineID,
		"env_id", input.EnvID,
		"cluster_id", env.ClusterID,
		"namespace", env.Namespace,
	)
	return run, nil
}

// ---------------------------------------------------------------------------
// CancelRun — 取消執行中或排隊中的 Run
// ---------------------------------------------------------------------------

// CancelRun 將 Run 標記為 cancelling，Watcher 負責清理 K8s Job。
func (s *PipelineScheduler) CancelRun(ctx context.Context, runID uint) error {
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("pipeline run %d not found", runID)
		}
		return fmt.Errorf("get pipeline run %d: %w", runID, err)
	}

	switch run.Status {
	case models.PipelineRunStatusQueued:
		now := time.Now()
		run.Status = models.PipelineRunStatusCancelled
		run.FinishedAt = &now
		if err := s.db.WithContext(ctx).Save(&run).Error; err != nil {
			return fmt.Errorf("cancel queued run %d: %w", runID, err)
		}
	case models.PipelineRunStatusRunning:
		run.Status = models.PipelineRunStatusCancelling
		if err := s.db.WithContext(ctx).Save(&run).Error; err != nil {
			return fmt.Errorf("cancel running run %d: %w", runID, err)
		}
	default:
		return fmt.Errorf("cannot cancel run %d in status %s", runID, run.Status)
	}

	logger.Info("pipeline run cancel requested",
		"run_id", runID,
		"previous_status", run.Status,
	)
	return nil
}
