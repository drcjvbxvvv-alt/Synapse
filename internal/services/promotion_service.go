package services

import (
	"context"
	"encoding/json"
	"fmt"

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
// 注意：Environment 抽象層已移除，此服務的晉升邏輯已停用。
// PipelineRun 現在直接攜帶 cluster_id + namespace。
type PromotionService struct {
	db       *gorm.DB
	notifier *PromotionNotifier // Production Gate 通知（可為 nil）
}

// NewPromotionService 建立 PromotionService。
func NewPromotionService(db *gorm.DB) *PromotionService {
	return &PromotionService{db: db}
}

// SetNotifier 設定 Production Gate 通知器。
func (s *PromotionService) SetNotifier(notifier *PromotionNotifier) {
	s.notifier = notifier
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

// EvaluatePromotion 評估是否可以晉升。
// 注意：Environment 抽象層已移除，此方法目前停用，固定回傳 blocked。
func (s *PromotionService) EvaluatePromotion(ctx context.Context, pipelineID uint, fromEnvName string) (*PromotionDecision, error) {
	return &PromotionDecision{
		Allowed:         false,
		Action:          "blocked",
		Reason:          "environment-based promotion has been removed; use direct cluster_id + namespace on each run",
		FromEnvironment: fromEnvName,
	}, nil
}

// ---------------------------------------------------------------------------
// Promotion 執行
// ---------------------------------------------------------------------------

// ExecutePromotion 執行環境晉升。
// 注意：Environment 抽象層已移除，此方法固定回傳 blocked。
func (s *PromotionService) ExecutePromotion(ctx context.Context, req *PromotionRequest) (*PromotionResult, error) {
	return &PromotionResult{
		Status:  "blocked",
		Message: "environment-based promotion has been removed; use direct cluster_id + namespace on each run",
	}, nil
}

// ApprovePromotion 批准晉升請求。
// 注意：Environment 抽象層已移除，此方法暫無作用。
func (s *PromotionService) ApprovePromotion(ctx context.Context, promotionID uint, approverID uint, reason string) error {
	return fmt.Errorf("environment-based promotion has been removed")
}

// RejectPromotion 拒絕晉升請求。
// 注意：Environment 抽象層已移除，此方法暫無作用。
func (s *PromotionService) RejectPromotion(ctx context.Context, promotionID uint, approverID uint, reason string) error {
	return fmt.Errorf("environment-based promotion has been removed")
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

// notifyProductionGate は Environment 抽象層削除により無効化されました。
func (s *PromotionService) notifyProductionGate(ctx context.Context, req *PromotionRequest, decision *PromotionDecision, approvalID uint) {
	// Environment 抽象層削除により、通知機能は無効です。
}

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
