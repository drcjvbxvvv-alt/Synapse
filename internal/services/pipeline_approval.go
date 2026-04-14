package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Approval Step — 人工審核閘道
//
// 設計原則（CICD_ARCHITECTURE §7.8, P1-8）：
//   - Approval Step 不建立 K8s Job，而是將 StepRun 狀態設為 waiting_approval
//   - Scheduler 輪詢 DB 等待狀態變更（由 ApproveStep / RejectStep API 更新）
//   - 審核通過 → success，拒絕 → failed（可配置為 rejected 語義）
//   - 預設無 timeout；可擴展加入 approval_timeout 欄位
// ---------------------------------------------------------------------------

// ApproveStep 批准 Approval Step，將其狀態設為 success。
func ApproveStep(ctx context.Context, db *gorm.DB, stepRunID uint, approvedBy string) error {
	var sr models.StepRun
	if err := db.WithContext(ctx).First(&sr, stepRunID).Error; err != nil {
		return fmt.Errorf("find step run %d: %w", stepRunID, err)
	}

	if sr.StepType != "approval" {
		return fmt.Errorf("step run %d is not an approval step (type: %s)", stepRunID, sr.StepType)
	}
	if sr.Status != models.StepRunStatusWaitingApproval {
		return fmt.Errorf("step run %d is not waiting for approval (status: %s)", stepRunID, sr.Status)
	}

	now := time.Now()
	sr.Status = models.StepRunStatusSuccess
	sr.ApprovedBy = &approvedBy
	sr.ApprovedAt = &now
	sr.FinishedAt = &now

	if err := db.WithContext(ctx).Save(&sr).Error; err != nil {
		return fmt.Errorf("approve step run %d: %w", stepRunID, err)
	}

	logger.Info("approval step approved",
		"step_run_id", stepRunID,
		"step_name", sr.StepName,
		"approved_by", approvedBy,
	)
	return nil
}

// RejectStep 拒絕 Approval Step，將其狀態設為 failed。
func RejectStep(ctx context.Context, db *gorm.DB, stepRunID uint, rejectedBy string, reason string) error {
	var sr models.StepRun
	if err := db.WithContext(ctx).First(&sr, stepRunID).Error; err != nil {
		return fmt.Errorf("find step run %d: %w", stepRunID, err)
	}

	if sr.StepType != "approval" {
		return fmt.Errorf("step run %d is not an approval step (type: %s)", stepRunID, sr.StepType)
	}
	if sr.Status != models.StepRunStatusWaitingApproval {
		return fmt.Errorf("step run %d is not waiting for approval (status: %s)", stepRunID, sr.Status)
	}

	now := time.Now()
	sr.Status = models.StepRunStatusFailed
	sr.ApprovedBy = &rejectedBy
	sr.ApprovedAt = &now
	sr.FinishedAt = &now
	errMsg := fmt.Sprintf("rejected by %s", rejectedBy)
	if reason != "" {
		errMsg = fmt.Sprintf("rejected by %s: %s", rejectedBy, reason)
	}
	sr.Error = errMsg

	if err := db.WithContext(ctx).Save(&sr).Error; err != nil {
		return fmt.Errorf("reject step run %d: %w", stepRunID, err)
	}

	logger.Info("approval step rejected",
		"step_run_id", stepRunID,
		"step_name", sr.StepName,
		"rejected_by", rejectedBy,
		"reason", reason,
	)
	return nil
}

// executeApprovalStep 由 Scheduler 呼叫，設定 waiting_approval 狀態並輪詢等待。
// 回傳 true 表示該 step 最終失敗（被拒絕或逾時）。
func (s *PipelineScheduler) executeApprovalStep(ctx context.Context, run *models.PipelineRun, sr *models.StepRun) bool {
	now := time.Now()
	sr.Status = models.StepRunStatusWaitingApproval
	sr.StartedAt = &now
	if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
		logger.Error("failed to set approval step to waiting", "step_run_id", sr.ID, "error", err)
		return true
	}

	logger.Info("approval step waiting for review",
		"step_run_id", sr.ID,
		"step_name", sr.StepName,
		"run_id", run.ID,
	)

	// 輪詢 DB 等待狀態變更（由 ApproveStep / RejectStep API 更新）
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return true
		case <-s.stopCh:
			return true
		case <-ticker.C:
			if err := s.db.WithContext(ctx).First(sr, sr.ID).Error; err != nil {
				logger.Error("failed to reload approval step run", "step_run_id", sr.ID, "error", err)
				return true
			}

			switch sr.Status {
			case models.StepRunStatusSuccess:
				return false // 審核通過
			case models.StepRunStatusFailed:
				return true // 被拒絕
			case models.StepRunStatusCancelled:
				return true
			case models.StepRunStatusWaitingApproval:
				// 繼續等待
				continue
			default:
				// 意外狀態
				logger.Warn("approval step in unexpected status",
					"step_run_id", sr.ID,
					"status", sr.Status,
				)
				return true
			}
		}
	}
}
