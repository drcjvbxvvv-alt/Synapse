package services

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineRecover — 重啟後恢復執行中的 Pipeline Runs
//
// 設計（CICD_ARCHITECTURE §17）：
//   - 啟動時掃描 status=running / cancelling 的 Runs
//   - 透過 label selector 重新 attach 到 K8s Job
//   - 無法 reattach 的 Run 標記為 failed（避免永久 running 狀態）
//   - 掃描 queued Runs 中已過期的（超過 maxQueueAge）標記為 failed
// ---------------------------------------------------------------------------

// RecoverConfig 恢復機制可調參數。
type RecoverConfig struct {
	MaxQueueAge  time.Duration // queued 超過此時間視為過期
	ReattachTimeout time.Duration // reattach 單一 Run 的超時
}

// DefaultRecoverConfig 傳回預設配置。
func DefaultRecoverConfig() RecoverConfig {
	return RecoverConfig{
		MaxQueueAge:     30 * time.Minute,
		ReattachTimeout: 30 * time.Second,
	}
}

// PipelineRecover 處理 Synapse 重啟後的 Pipeline 恢復。
type PipelineRecover struct {
	db      *gorm.DB
	watcher *JobWatcher
	cfg     RecoverConfig
}

// NewPipelineRecover 建立恢復器。
func NewPipelineRecover(db *gorm.DB, watcher *JobWatcher, cfg RecoverConfig) *PipelineRecover {
	return &PipelineRecover{
		db:      db,
		watcher: watcher,
		cfg:     cfg,
	}
}

// Recover 在啟動時執行，恢復所有中斷的 Pipeline Runs。
// 應在 Scheduler.Start() 之前呼叫。
func (r *PipelineRecover) Recover(ctx context.Context) error {
	logger.Info("pipeline recover started")

	var errs []error

	if err := r.recoverRunningRuns(ctx); err != nil {
		errs = append(errs, fmt.Errorf("recover running runs: %w", err))
	}

	if err := r.recoverCancellingRuns(ctx); err != nil {
		errs = append(errs, fmt.Errorf("recover cancelling runs: %w", err))
	}

	if err := r.expireStaleQueuedRuns(ctx); err != nil {
		errs = append(errs, fmt.Errorf("expire stale queued runs: %w", err))
	}

	if len(errs) > 0 {
		for _, e := range errs {
			logger.Error("pipeline recover partial failure", "error", e)
		}
		return errs[0]
	}

	logger.Info("pipeline recover completed")
	return nil
}

// ---------------------------------------------------------------------------
// Running Runs — re-attach Watcher
// ---------------------------------------------------------------------------

func (r *PipelineRecover) recoverRunningRuns(ctx context.Context) error {
	var runs []models.PipelineRun
	if err := r.db.WithContext(ctx).
		Where("status = ?", models.PipelineRunStatusRunning).
		Find(&runs).Error; err != nil {
		return fmt.Errorf("query running runs: %w", err)
	}

	if len(runs) == 0 {
		logger.Info("no running runs to recover")
		return nil
	}

	logger.Info("recovering running runs", "count", len(runs))

	for i := range runs {
		run := &runs[i]

		// 檢查是否有對應的 StepRun 仍在執行
		var activeSteps int64
		if err := r.db.WithContext(ctx).Model(&models.StepRun{}).
			Where("pipeline_run_id = ? AND status IN ?",
				run.ID,
				[]string{models.StepRunStatusRunning, models.StepRunStatusPending},
			).Count(&activeSteps).Error; err != nil {
			logger.Error("failed to count active steps", "run_id", run.ID, "error", err)
			continue
		}

		if activeSteps == 0 {
			// 所有 Step 已結束但 Run 未更新 — 直接完結
			r.finalizeOrphanedRun(ctx, run)
			continue
		}

		// Re-attach Watcher（K8s client 可能尚未初始化，watcher 內部會在 poll 時重試）
		r.watcher.WatchRun(run)
		logger.Info("run reattached to watcher",
			"run_id", run.ID,
			"cluster_id", run.ClusterID,
		)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Cancelling Runs — 完成取消流程
// ---------------------------------------------------------------------------

func (r *PipelineRecover) recoverCancellingRuns(ctx context.Context) error {
	var runs []models.PipelineRun
	if err := r.db.WithContext(ctx).
		Where("status = ?", models.PipelineRunStatusCancelling).
		Find(&runs).Error; err != nil {
		return fmt.Errorf("query cancelling runs: %w", err)
	}

	if len(runs) == 0 {
		return nil
	}

	logger.Info("recovering cancelling runs", "count", len(runs))

	for i := range runs {
		run := &runs[i]
		// Re-attach watcher，它會偵測 cancelling 狀態並執行清理
		r.watcher.WatchRun(run)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Stale Queued Runs — 過期處理
// ---------------------------------------------------------------------------

func (r *PipelineRecover) expireStaleQueuedRuns(ctx context.Context) error {
	cutoff := time.Now().Add(-r.cfg.MaxQueueAge)

	var staleRuns []models.PipelineRun
	if err := r.db.WithContext(ctx).
		Where("status = ? AND queued_at < ?", models.PipelineRunStatusQueued, cutoff).
		Find(&staleRuns).Error; err != nil {
		return fmt.Errorf("query stale queued runs: %w", err)
	}

	if len(staleRuns) == 0 {
		return nil
	}

	logger.Warn("expiring stale queued runs", "count", len(staleRuns))

	now := time.Now()
	for i := range staleRuns {
		run := &staleRuns[i]
		run.Status = models.PipelineRunStatusFailed
		run.Error = fmt.Sprintf("expired: queued for over %v without being scheduled", r.cfg.MaxQueueAge)
		run.FinishedAt = &now
		if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
			logger.Error("failed to expire stale run", "run_id", run.ID, "error", err)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (r *PipelineRecover) finalizeOrphanedRun(ctx context.Context, run *models.PipelineRun) {
	// 檢查是否有任何 Step 失敗
	var failedCount int64
	if err := r.db.WithContext(ctx).Model(&models.StepRun{}).
		Where("pipeline_run_id = ? AND status = ?", run.ID, models.StepRunStatusFailed).
		Count(&failedCount).Error; err != nil {
		logger.Error("failed to count failed steps for orphaned run",
			"run_id", run.ID, "error", err)
		// 保守策略：查詢失敗時標記為 failed
		failedCount = 1
	}

	now := time.Now()
	run.FinishedAt = &now
	if failedCount > 0 {
		run.Status = models.PipelineRunStatusFailed
	} else {
		run.Status = models.PipelineRunStatusSuccess
	}

	if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
		logger.Error("failed to finalize orphaned run", "run_id", run.ID, "error", err)
		return
	}

	logger.Info("orphaned run finalized",
		"run_id", run.ID,
		"status", run.Status,
	)
}

func (r *PipelineRecover) markRunFailed(ctx context.Context, run *models.PipelineRun, errMsg string) {
	now := time.Now()
	run.Status = models.PipelineRunStatusFailed
	run.Error = errMsg
	run.FinishedAt = &now

	// 同時標記所有 active steps 為 cancelled
	if err := r.db.WithContext(ctx).Model(&models.StepRun{}).
		Where("pipeline_run_id = ? AND status IN ?",
			run.ID,
			[]string{models.StepRunStatusRunning, models.StepRunStatusPending},
		).Updates(map[string]interface{}{
		"status":      models.StepRunStatusCancelled,
		"finished_at": now,
	}).Error; err != nil {
		logger.Error("failed to cancel active steps for run", "run_id", run.ID, "error", err)
	}

	if err := r.db.WithContext(ctx).Save(run).Error; err != nil {
		logger.Error("failed to mark run as failed", "run_id", run.ID, "error", err)
	}
}
