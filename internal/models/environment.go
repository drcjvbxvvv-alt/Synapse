package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Environment — 環境管理（CICD_ARCHITECTURE §13）
//
// 設計：
//   - 每個 Environment 屬於一個 Pipeline
//   - order_index 決定晉升順序（dev=1 → staging=2 → production=3）
//   - auto_promote 控制是否自動晉升到下一環境
//   - approval_required 控制是否需要人工審核
//   - approver_ids 為 JSON 陣列格式的 user ID 清單
// ---------------------------------------------------------------------------

// Environment 儲存部署環境設定。
type Environment struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	Name             string         `json:"name" gorm:"not null;size:255"`
	PipelineID       uint           `json:"pipeline_id" gorm:"not null;uniqueIndex:uq_pipeline_env"`
	ClusterID        uint           `json:"cluster_id" gorm:"not null;index"`
	Namespace        string         `json:"namespace" gorm:"not null;size:253"`
	OrderIndex       int            `json:"order_index" gorm:"not null;index:idx_env_order"`
	AutoPromote      bool           `json:"auto_promote" gorm:"default:false"`
	ApprovalRequired bool           `json:"approval_required" gorm:"default:false"`
	ApproverIDs      string         `json:"approver_ids,omitempty" gorm:"type:text"` // JSON array of user IDs
	SmokeTestStepName string        `json:"smoke_test_step_name,omitempty" gorm:"size:255"`
	NotifyChannelIDs  string        `json:"notify_channel_ids,omitempty" gorm:"type:text"` // JSON array: Production Gate 通知 channel
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Environment) TableName() string { return "environments" }

// ---------------------------------------------------------------------------
// PromotionHistory — 環境晉升歷史記錄
// ---------------------------------------------------------------------------

// PromotionHistory 記錄環境間的晉升操作。
type PromotionHistory struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	PipelineID      uint      `json:"pipeline_id" gorm:"not null;index"`
	PipelineRunID   uint      `json:"pipeline_run_id" gorm:"not null;index"`
	FromEnvironment string    `json:"from_environment" gorm:"not null;size:255"`
	ToEnvironment   string    `json:"to_environment" gorm:"not null;size:255"`
	Status          string    `json:"status" gorm:"not null;size:30"` // pending / approved / rejected / auto_promoted
	PromotedBy      uint      `json:"promoted_by,omitempty"`
	ApprovalID      *uint     `json:"approval_id,omitempty" gorm:"index"` // FK to approval_requests
	Reason          string    `json:"reason,omitempty" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PromotionHistory) TableName() string { return "promotion_history" }

// PromotionHistory 狀態常數
const (
	PromotionStatusPending      = "pending"
	PromotionStatusApproved     = "approved"
	PromotionStatusRejected     = "rejected"
	PromotionStatusAutoPromoted = "auto_promoted"
)
