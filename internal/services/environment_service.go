package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// EnvironmentService — Pipeline 環境 CRUD + 促進輔助
// ---------------------------------------------------------------------------

// EnvironmentService 管理 Pipeline 的執行目標環境。
type EnvironmentService struct {
	db *gorm.DB
}

// NewEnvironmentService 建立 EnvironmentService。
func NewEnvironmentService(db *gorm.DB) *EnvironmentService {
	return &EnvironmentService{db: db}
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// CreateEnvironmentRequest 建立 Environment 請求。
type CreateEnvironmentRequest struct {
	Name              string `json:"name" binding:"required,max=255"`
	ClusterID         uint   `json:"cluster_id" binding:"required"`
	Namespace         string `json:"namespace" binding:"required,max=253"`
	OrderIndex        int    `json:"order_index"`
	AutoPromote       bool   `json:"auto_promote"`
	ApprovalRequired  bool   `json:"approval_required"`
	ApproverIDs       string `json:"approver_ids"`
	SmokeTestStepName string `json:"smoke_test_step_name"`
	NotifyChannelIDs  string `json:"notify_channel_ids"`
	VariablesJSON     string `json:"variables_json"`
}

// UpdateEnvironmentRequest 更新 Environment 請求（所有欄位可選）。
type UpdateEnvironmentRequest struct {
	ClusterID         *uint   `json:"cluster_id"`
	Namespace         *string `json:"namespace"`
	OrderIndex        *int    `json:"order_index"`
	AutoPromote       *bool   `json:"auto_promote"`
	ApprovalRequired  *bool   `json:"approval_required"`
	ApproverIDs       *string `json:"approver_ids"`
	SmokeTestStepName *string `json:"smoke_test_step_name"`
	NotifyChannelIDs  *string `json:"notify_channel_ids"`
	VariablesJSON     *string `json:"variables_json"`
}

// EnvironmentWithCluster 包含 Environment 及其關聯 Cluster 的基本資訊。
type EnvironmentWithCluster struct {
	models.Environment
	ClusterName   string `json:"cluster_name"`
	ClusterStatus string `json:"cluster_status"`
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// ListEnvironments 列出 Pipeline 的所有 Environment，按 OrderIndex 升序排列。
func (s *EnvironmentService) ListEnvironments(ctx context.Context, pipelineID uint) ([]models.Environment, error) {
	var envs []models.Environment
	if err := s.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("order_index ASC, id ASC").
		Find(&envs).Error; err != nil {
		return nil, fmt.Errorf("list environments for pipeline %d: %w", pipelineID, err)
	}
	return envs, nil
}

// GetEnvironment 取得單一 Environment。
func (s *EnvironmentService) GetEnvironment(ctx context.Context, envID uint) (*models.Environment, error) {
	var env models.Environment
	if err := s.db.WithContext(ctx).First(&env, envID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("environment %d not found: %w", envID, gorm.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("get environment %d: %w", envID, err)
	}
	return &env, nil
}

// GetEnvironmentByPipelineAndID 取得屬於特定 Pipeline 的 Environment（防止跨 Pipeline 存取）。
func (s *EnvironmentService) GetEnvironmentByPipelineAndID(ctx context.Context, pipelineID, envID uint) (*models.Environment, error) {
	var env models.Environment
	err := s.db.WithContext(ctx).
		Where("id = ? AND pipeline_id = ?", envID, pipelineID).
		First(&env).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("environment %d not found in pipeline %d: %w", envID, pipelineID, gorm.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("get environment %d: %w", envID, err)
	}
	return &env, nil
}

// CreateEnvironment 為指定 Pipeline 建立新 Environment。
// 若 OrderIndex 未指定（<=0），自動使用最大值 + 1。
func (s *EnvironmentService) CreateEnvironment(ctx context.Context, pipelineID uint, req *CreateEnvironmentRequest) (*models.Environment, error) {
	orderIndex := req.OrderIndex
	if orderIndex <= 0 {
		var maxOrder int
		s.db.WithContext(ctx).
			Model(&models.Environment{}).
			Where("pipeline_id = ?", pipelineID).
			Select("COALESCE(MAX(order_index), 0)").
			Scan(&maxOrder)
		orderIndex = maxOrder + 1
	}

	env := &models.Environment{
		Name:              req.Name,
		PipelineID:        pipelineID,
		ClusterID:         req.ClusterID,
		Namespace:         req.Namespace,
		OrderIndex:        orderIndex,
		AutoPromote:       req.AutoPromote,
		ApprovalRequired:  req.ApprovalRequired,
		ApproverIDs:       req.ApproverIDs,
		SmokeTestStepName: req.SmokeTestStepName,
		NotifyChannelIDs:  req.NotifyChannelIDs,
		VariablesJSON:     req.VariablesJSON,
	}

	if env.VariablesJSON == "" {
		env.VariablesJSON = "{}"
	}

	if err := s.db.WithContext(ctx).Create(env).Error; err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("environment %q already exists in pipeline %d", req.Name, pipelineID)
		}
		return nil, fmt.Errorf("create environment: %w", err)
	}

	logger.Info("environment created",
		"env_id", env.ID,
		"pipeline_id", pipelineID,
		"name", env.Name,
		"cluster_id", env.ClusterID,
	)
	return env, nil
}

// UpdateEnvironment 更新 Environment 欄位（部分更新）。
func (s *EnvironmentService) UpdateEnvironment(ctx context.Context, pipelineID, envID uint, req *UpdateEnvironmentRequest) (*models.Environment, error) {
	env, err := s.GetEnvironmentByPipelineAndID(ctx, pipelineID, envID)
	if err != nil {
		return nil, err
	}

	if req.ClusterID != nil {
		env.ClusterID = *req.ClusterID
	}
	if req.Namespace != nil {
		env.Namespace = *req.Namespace
	}
	if req.OrderIndex != nil {
		env.OrderIndex = *req.OrderIndex
	}
	if req.AutoPromote != nil {
		env.AutoPromote = *req.AutoPromote
	}
	if req.ApprovalRequired != nil {
		env.ApprovalRequired = *req.ApprovalRequired
	}
	if req.ApproverIDs != nil {
		env.ApproverIDs = *req.ApproverIDs
	}
	if req.SmokeTestStepName != nil {
		env.SmokeTestStepName = *req.SmokeTestStepName
	}
	if req.NotifyChannelIDs != nil {
		env.NotifyChannelIDs = *req.NotifyChannelIDs
	}
	if req.VariablesJSON != nil {
		env.VariablesJSON = *req.VariablesJSON
	}

	if err := s.db.WithContext(ctx).Save(env).Error; err != nil {
		return nil, fmt.Errorf("update environment %d: %w", envID, err)
	}

	logger.Info("environment updated", "env_id", envID, "pipeline_id", pipelineID)
	return env, nil
}

// DeleteEnvironment 軟刪除指定 Environment。
func (s *EnvironmentService) DeleteEnvironment(ctx context.Context, pipelineID, envID uint) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND pipeline_id = ?", envID, pipelineID).
		Delete(&models.Environment{})
	if result.Error != nil {
		return fmt.Errorf("delete environment %d: %w", envID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("environment %d not found in pipeline %d: %w", envID, pipelineID, gorm.ErrRecordNotFound)
	}

	logger.Info("environment deleted", "env_id", envID, "pipeline_id", pipelineID)
	return nil
}

// ---------------------------------------------------------------------------
// Promotion helpers
// ---------------------------------------------------------------------------

// GetDefaultEnvironment 取得 Pipeline 的預設環境（OrderIndex 最小者）。
// 用於 Webhook 觸發時無指定 Environment 的情況。
func (s *EnvironmentService) GetDefaultEnvironment(ctx context.Context, pipelineID uint) (*models.Environment, error) {
	var env models.Environment
	err := s.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("order_index ASC, id ASC").
		First(&env).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pipeline %d has no environments: %w", pipelineID, gorm.ErrRecordNotFound)
		}
		return nil, fmt.Errorf("get default environment for pipeline %d: %w", pipelineID, err)
	}
	return &env, nil
}

// GetNextEnvironment 取得下一個促進目標 Environment。
// 依 OrderIndex 升序排列，取第一個 OrderIndex 大於當前環境的環境。
// 若當前環境已是最後一個，回傳 nil, nil。
func (s *EnvironmentService) GetNextEnvironment(ctx context.Context, pipelineID uint, currentOrderIndex int) (*models.Environment, error) {
	var env models.Environment
	err := s.db.WithContext(ctx).
		Where("pipeline_id = ? AND order_index > ?", pipelineID, currentOrderIndex).
		Order("order_index ASC").
		First(&env).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 已是最後一個環境
		}
		return nil, fmt.Errorf("get next environment for pipeline %d: %w", pipelineID, err)
	}
	return &env, nil
}

// GetEnvironmentsForPipeline 取得 Pipeline 的所有 Environment 列表（含 Cluster 基本資訊）。
// 使用 JOIN 一次取得 cluster_name 和 cluster_status，避免 N+1 查詢。
func (s *EnvironmentService) GetEnvironmentsForPipeline(ctx context.Context, pipelineID uint) ([]EnvironmentWithCluster, error) {
	type row struct {
		models.Environment
		ClusterName   string `gorm:"column:cluster_name"`
		ClusterStatus string `gorm:"column:cluster_status"`
	}

	var rows []row
	err := s.db.WithContext(ctx).
		Table("environments e").
		Select("e.*, COALESCE(c.name, '') AS cluster_name, COALESCE(c.status, '') AS cluster_status").
		Joins("LEFT JOIN clusters c ON c.id = e.cluster_id AND c.deleted_at IS NULL").
		Where("e.pipeline_id = ? AND e.deleted_at IS NULL", pipelineID).
		Order("e.order_index ASC, e.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get environments for pipeline %d: %w", pipelineID, err)
	}

	result := make([]EnvironmentWithCluster, 0, len(rows))
	for _, r := range rows {
		result = append(result, EnvironmentWithCluster{
			Environment:   r.Environment,
			ClusterName:   r.ClusterName,
			ClusterStatus: r.ClusterStatus,
		})
	}
	return result, nil
}

// RecordPromotion 記錄環境晉升事件。
func (s *EnvironmentService) RecordPromotion(ctx context.Context, h *models.PromotionHistory) error {
	if err := s.db.WithContext(ctx).Create(h).Error; err != nil {
		return fmt.Errorf("record promotion: %w", err)
	}
	return nil
}
