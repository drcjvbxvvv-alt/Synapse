package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// EnvironmentService — Environment CRUD + Promotion History（CICD_ARCHITECTURE §13, P2-8）
// ---------------------------------------------------------------------------

// EnvironmentService 管理 Environment 的 CRUD 操作。
type EnvironmentService struct {
	db *gorm.DB
}

// NewEnvironmentService 建立 Environment 服務。
func NewEnvironmentService(db *gorm.DB) *EnvironmentService {
	return &EnvironmentService{db: db}
}

// CreateEnvironment 建立新的 Environment。
func (s *EnvironmentService) CreateEnvironment(ctx context.Context, env *models.Environment) error {
	if err := validateEnvironment(env); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Create(env).Error; err != nil {
		return fmt.Errorf("create environment: %w", err)
	}

	logger.Info("environment created",
		"environment_id", env.ID,
		"name", env.Name,
		"pipeline_id", env.PipelineID,
		"cluster_id", env.ClusterID,
	)
	return nil
}

// GetEnvironment 取得單一 Environment。
func (s *EnvironmentService) GetEnvironment(ctx context.Context, id uint) (*models.Environment, error) {
	var env models.Environment
	if err := s.db.WithContext(ctx).First(&env, id).Error; err != nil {
		return nil, fmt.Errorf("get environment %d: %w", id, err)
	}
	return &env, nil
}

// ListEnvironments 列出指定 Pipeline 的所有 Environment（按 order_index 排序）。
func (s *EnvironmentService) ListEnvironments(ctx context.Context, pipelineID uint) ([]models.Environment, error) {
	var envs []models.Environment
	if err := s.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("order_index ASC").
		Find(&envs).Error; err != nil {
		return nil, fmt.Errorf("list environments for pipeline %d: %w", pipelineID, err)
	}
	return envs, nil
}

// UpdateEnvironment 更新 Environment。
func (s *EnvironmentService) UpdateEnvironment(ctx context.Context, id uint, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).Model(&models.Environment{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update environment %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("environment %d not found", id)
	}

	logger.Info("environment updated", "environment_id", id)
	return nil
}

// DeleteEnvironment 刪除 Environment（soft delete）。
func (s *EnvironmentService) DeleteEnvironment(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.Environment{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete environment %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("environment %d not found", id)
	}

	logger.Info("environment deleted", "environment_id", id)
	return nil
}

// GetNextEnvironment 取得指定 Environment 的下一個晉升目標。
func (s *EnvironmentService) GetNextEnvironment(ctx context.Context, pipelineID uint, currentOrderIndex int) (*models.Environment, error) {
	var env models.Environment
	if err := s.db.WithContext(ctx).
		Where("pipeline_id = ? AND order_index > ?", pipelineID, currentOrderIndex).
		Order("order_index ASC").
		First(&env).Error; err != nil {
		return nil, fmt.Errorf("get next environment after index %d: %w", currentOrderIndex, err)
	}
	return &env, nil
}

// ---------------------------------------------------------------------------
// Promotion History
// ---------------------------------------------------------------------------

// RecordPromotion 記錄一筆晉升歷史。
func (s *EnvironmentService) RecordPromotion(ctx context.Context, history *models.PromotionHistory) error {
	if err := s.db.WithContext(ctx).Create(history).Error; err != nil {
		return fmt.Errorf("record promotion: %w", err)
	}

	logger.Info("promotion recorded",
		"pipeline_id", history.PipelineID,
		"from", history.FromEnvironment,
		"to", history.ToEnvironment,
		"status", history.Status,
	)
	return nil
}

// ListPromotionHistory 列出指定 Pipeline 的晉升歷史。
func (s *EnvironmentService) ListPromotionHistory(ctx context.Context, pipelineID uint) ([]models.PromotionHistory, error) {
	var history []models.PromotionHistory
	if err := s.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("list promotion history for pipeline %d: %w", pipelineID, err)
	}
	return history, nil
}

// UpdatePromotionStatus 更新晉升記錄的狀態。
func (s *EnvironmentService) UpdatePromotionStatus(ctx context.Context, id uint, status string, promotedBy uint, reason string) error {
	updates := map[string]interface{}{
		"status":      status,
		"promoted_by": promotedBy,
		"reason":      reason,
	}

	result := s.db.WithContext(ctx).Model(&models.PromotionHistory{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update promotion status %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("promotion record %d not found", id)
	}

	logger.Info("promotion status updated",
		"promotion_id", id,
		"status", status,
	)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateEnvironment(env *models.Environment) error {
	if env.Name == "" {
		return fmt.Errorf("environment name is required")
	}
	if env.PipelineID == 0 {
		return fmt.Errorf("environment pipeline_id is required")
	}
	if env.ClusterID == 0 {
		return fmt.Errorf("environment cluster_id is required")
	}
	if env.Namespace == "" {
		return fmt.Errorf("environment namespace is required")
	}
	if env.OrderIndex < 0 {
		return fmt.Errorf("environment order_index must be non-negative")
	}

	// Validate approver_ids is valid JSON array if provided
	if env.ApproverIDs != "" {
		var ids []uint
		if err := json.Unmarshal([]byte(env.ApproverIDs), &ids); err != nil {
			return fmt.Errorf("approver_ids must be a valid JSON array of user IDs: %w", err)
		}
	}

	// Validate variables_json is valid JSON object if provided
	if env.VariablesJSON != "" {
		var vars map[string]string
		if err := json.Unmarshal([]byte(env.VariablesJSON), &vars); err != nil {
			return fmt.Errorf("variables_json must be a valid JSON object of string key-value pairs: %w", err)
		}
	}

	return nil
}
