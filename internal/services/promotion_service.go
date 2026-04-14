package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PromotionService — 環境晉升狀態機 + Policy 引擎（CICD_ARCHITECTURE §13, P1-7）
//
// 設計原則：
//   - 嚴格按照 order_index 順序晉升，不可跳關
//   - auto_promote 環境自動建立 ApprovalRequest 並批准
//   - approval_required 環境必須人工審核
//   - Production Gate 永遠需要人工審核（最高 order_index）
//   - 復用現有 ApprovalRequest 模型
// ---------------------------------------------------------------------------

// PromotionService 管理環境晉升流程。
type PromotionService struct {
	db     *gorm.DB
	envSvc *EnvironmentService
}

// NewPromotionService 建立 PromotionService。
func NewPromotionService(db *gorm.DB, envSvc *EnvironmentService) *PromotionService {
	return &PromotionService{db: db, envSvc: envSvc}
}

// ---------------------------------------------------------------------------
// Promotion Policy 評估
// ---------------------------------------------------------------------------

// PromotionDecision 代表晉升策略評估結果。
type PromotionDecision struct {
	Allowed          bool   `json:"allowed"`
	Action           string `json:"action"`            // auto_promote / require_approval / blocked
	Reason           string `json:"reason"`
	FromEnvironment  string `json:"from_environment"`
	ToEnvironment    string `json:"to_environment"`
	ApprovalRequired bool   `json:"approval_required"`
	ApproverIDs      []uint `json:"approver_ids,omitempty"`
}

// EvaluatePromotion 評估是否可以從指定環境晉升到下一個環境。
func (s *PromotionService) EvaluatePromotion(ctx context.Context, pipelineID uint, fromEnvName string) (*PromotionDecision, error) {
	// 取得所有環境（按 order_index 排序）
	envs, err := s.envSvc.ListEnvironments(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	if len(envs) == 0 {
		return &PromotionDecision{
			Allowed: false,
			Action:  "blocked",
			Reason:  "no environments configured for this pipeline",
		}, nil
	}

	// 找到來源環境
	var fromEnv *models.Environment
	var fromIdx int
	for i := range envs {
		if envs[i].Name == fromEnvName {
			fromEnv = &envs[i]
			fromIdx = i
			break
		}
	}
	if fromEnv == nil {
		return &PromotionDecision{
			Allowed: false,
			Action:  "blocked",
			Reason:  fmt.Sprintf("environment %q not found", fromEnvName),
		}, nil
	}

	// 檢查是否有下一個環境
	if fromIdx >= len(envs)-1 {
		return &PromotionDecision{
			Allowed:         false,
			Action:          "blocked",
			Reason:          fmt.Sprintf("%q is the last environment, nowhere to promote", fromEnvName),
			FromEnvironment: fromEnvName,
		}, nil
	}

	toEnv := &envs[fromIdx+1]

	// 解析 approver IDs
	var approverIDs []uint
	if toEnv.ApproverIDs != "" {
		if err := json.Unmarshal([]byte(toEnv.ApproverIDs), &approverIDs); err != nil {
			logger.Warn("failed to parse approver_ids", "environment", toEnv.Name, "error", err)
		}
	}

	decision := &PromotionDecision{
		FromEnvironment: fromEnvName,
		ToEnvironment:   toEnv.Name,
		ApproverIDs:     approverIDs,
	}

	// Policy: 目標環境需要審核
	if toEnv.ApprovalRequired {
		decision.Allowed = true
		decision.Action = "require_approval"
		decision.Reason = fmt.Sprintf("promotion to %q requires approval", toEnv.Name)
		decision.ApprovalRequired = true
		return decision, nil
	}

	// Policy: 來源環境設定自動晉升
	if fromEnv.AutoPromote {
		decision.Allowed = true
		decision.Action = "auto_promote"
		decision.Reason = fmt.Sprintf("auto-promote from %q to %q", fromEnvName, toEnv.Name)
		decision.ApprovalRequired = false
		return decision, nil
	}

	// 預設：需要審核
	decision.Allowed = true
	decision.Action = "require_approval"
	decision.Reason = fmt.Sprintf("promotion from %q to %q requires approval (default policy)", fromEnvName, toEnv.Name)
	decision.ApprovalRequired = true
	return decision, nil
}

// ---------------------------------------------------------------------------
// Promotion 執行
// ---------------------------------------------------------------------------

// ExecutePromotion 執行環境晉升（根據 policy 決定自動或建立審核請求）。
func (s *PromotionService) ExecutePromotion(ctx context.Context, req *PromotionRequest) (*PromotionResult, error) {
	// 評估策略
	decision, err := s.EvaluatePromotion(ctx, req.PipelineID, req.FromEnvironment)
	if err != nil {
		return nil, fmt.Errorf("evaluate promotion: %w", err)
	}

	if !decision.Allowed {
		return &PromotionResult{
			Status:  "blocked",
			Message: decision.Reason,
		}, nil
	}

	// 記錄晉升歷史
	history := &models.PromotionHistory{
		PipelineID:      req.PipelineID,
		PipelineRunID:   req.PipelineRunID,
		FromEnvironment: decision.FromEnvironment,
		ToEnvironment:   decision.ToEnvironment,
		PromotedBy:      req.TriggeredBy,
	}

	switch decision.Action {
	case "auto_promote":
		history.Status = models.PromotionStatusAutoPromoted
		history.Reason = decision.Reason

		if err := s.envSvc.RecordPromotion(ctx, history); err != nil {
			return nil, fmt.Errorf("record auto promotion: %w", err)
		}

		logger.Info("auto-promotion executed",
			"pipeline_id", req.PipelineID,
			"from", decision.FromEnvironment,
			"to", decision.ToEnvironment,
		)

		return &PromotionResult{
			Status:      "auto_promoted",
			Message:     decision.Reason,
			PromotionID: history.ID,
		}, nil

	case "require_approval":
		history.Status = models.PromotionStatusPending

		if err := s.envSvc.RecordPromotion(ctx, history); err != nil {
			return nil, fmt.Errorf("record pending promotion: %w", err)
		}

		// 建立 ApprovalRequest
		approval := &models.ApprovalRequest{
			ClusterID:       req.ClusterID,
			Namespace:       req.Namespace,
			ResourceKind:    "Pipeline",
			ResourceName:    fmt.Sprintf("pipeline-%d", req.PipelineID),
			Action:          "promote_environment",
			RequesterID:     req.TriggeredBy,
			RequesterName:   req.TriggeredByName,
			Status:          "pending",
			Payload:         marshalPromotionPayload(req, decision),
			PipelineRunID:   &req.PipelineRunID,
			FromEnvironment: decision.FromEnvironment,
			ToEnvironment:   decision.ToEnvironment,
			ExpiresAt:       time.Now().Add(24 * time.Hour), // 24h expiry
		}

		if err := s.db.WithContext(ctx).Create(approval).Error; err != nil {
			return nil, fmt.Errorf("create approval request: %w", err)
		}

		logger.Info("promotion approval requested",
			"pipeline_id", req.PipelineID,
			"from", decision.FromEnvironment,
			"to", decision.ToEnvironment,
			"approval_id", approval.ID,
		)

		return &PromotionResult{
			Status:      "pending_approval",
			Message:     decision.Reason,
			PromotionID: history.ID,
			ApprovalID:  approval.ID,
		}, nil

	default:
		return &PromotionResult{
			Status:  "blocked",
			Message: fmt.Sprintf("unknown action: %s", decision.Action),
		}, nil
	}
}

// ApprovePromotion 批准晉升請求。
func (s *PromotionService) ApprovePromotion(ctx context.Context, promotionID uint, approverID uint, reason string) error {
	return s.envSvc.UpdatePromotionStatus(ctx, promotionID, models.PromotionStatusApproved, approverID, reason)
}

// RejectPromotion 拒絕晉升請求。
func (s *PromotionService) RejectPromotion(ctx context.Context, promotionID uint, approverID uint, reason string) error {
	return s.envSvc.UpdatePromotionStatus(ctx, promotionID, models.PromotionStatusRejected, approverID, reason)
}

// ---------------------------------------------------------------------------
// Policy validation helpers
// ---------------------------------------------------------------------------

// ValidatePromotionOrder 驗證晉升是否按照正確的順序（不可跳關）。
func ValidatePromotionOrder(envs []models.Environment, fromName, toName string) error {
	var fromIdx, toIdx int
	fromFound, toFound := false, false

	for i, env := range envs {
		if env.Name == fromName {
			fromIdx = i
			fromFound = true
		}
		if env.Name == toName {
			toIdx = i
			toFound = true
		}
	}

	if !fromFound {
		return fmt.Errorf("source environment %q not found", fromName)
	}
	if !toFound {
		return fmt.Errorf("target environment %q not found", toName)
	}

	if toIdx != fromIdx+1 {
		return fmt.Errorf("cannot promote from %q (index %d) to %q (index %d): must promote to the next environment in order",
			fromName, fromIdx, toName, toIdx)
	}

	return nil
}

// ValidateNotReversePromotion 驗證不允許反向晉升（降級）。
func ValidateNotReversePromotion(envs []models.Environment, fromName, toName string) error {
	var fromIdx, toIdx int

	for i, env := range envs {
		if env.Name == fromName {
			fromIdx = i
		}
		if env.Name == toName {
			toIdx = i
		}
	}

	if toIdx <= fromIdx {
		return fmt.Errorf("reverse promotion not allowed: %q (index %d) → %q (index %d)",
			fromName, fromIdx, toName, toIdx)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PromotionRequest 代表晉升請求。
type PromotionRequest struct {
	PipelineID      uint   `json:"pipeline_id"`
	PipelineRunID   uint   `json:"pipeline_run_id"`
	FromEnvironment string `json:"from_environment"`
	ClusterID       uint   `json:"cluster_id"`
	Namespace       string `json:"namespace"`
	TriggeredBy     uint   `json:"triggered_by"`
	TriggeredByName string `json:"triggered_by_name"`
}

// PromotionResult 代表晉升執行結果。
type PromotionResult struct {
	Status      string `json:"status"`       // auto_promoted / pending_approval / blocked
	Message     string `json:"message"`
	PromotionID uint   `json:"promotion_id,omitempty"`
	ApprovalID  uint   `json:"approval_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func marshalPromotionPayload(req *PromotionRequest, decision *PromotionDecision) string {
	payload := map[string]interface{}{
		"pipeline_id":      req.PipelineID,
		"pipeline_run_id":  req.PipelineRunID,
		"from_environment": decision.FromEnvironment,
		"to_environment":   decision.ToEnvironment,
		"triggered_by":     req.TriggeredBy,
	}
	b, _ := json.Marshal(payload)
	return string(b)
}
