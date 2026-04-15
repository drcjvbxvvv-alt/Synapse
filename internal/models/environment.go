package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Environment — Pipeline 執行目標（Cluster + Namespace 綁定）
// ---------------------------------------------------------------------------

// Environment 是 Pipeline 的部署目標，唯一地映射到一個 Cluster + Namespace。
// 每個 Pipeline 可以有多個 Environment，按 OrderIndex 排列促進順序。
type Environment struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	Name              string         `json:"name" gorm:"not null;size:255;uniqueIndex:uq_pipeline_env"`
	PipelineID        uint           `json:"pipeline_id" gorm:"not null;uniqueIndex:uq_pipeline_env;index"`
	ClusterID         uint           `json:"cluster_id" gorm:"not null;index"`
	Namespace         string         `json:"namespace" gorm:"not null;size:253"`
	OrderIndex        int            `json:"order_index" gorm:"not null"`
	AutoPromote       bool           `json:"auto_promote" gorm:"default:false"`
	ApprovalRequired  bool           `json:"approval_required" gorm:"default:false"`
	ApproverIDs       string         `json:"approver_ids,omitempty" gorm:"type:text"` // JSON 陣列，user ID 列表
	SmokeTestStepName string         `json:"smoke_test_step_name,omitempty" gorm:"size:255"`
	NotifyChannelIDs  string         `json:"notify_channel_ids,omitempty" gorm:"type:text"` // JSON 陣列
	VariablesJSON     string         `json:"variables_json,omitempty" gorm:"type:text"`     // env-specific 變數覆寫
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定資料表名稱。
func (Environment) TableName() string { return "environments" }

// ---------------------------------------------------------------------------
// PromotionHistory — 環境促進記錄
// ---------------------------------------------------------------------------

// PromotionHistory 記錄 Pipeline Run 在環境間的晉升事件。
type PromotionHistory struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	PipelineID     uint      `json:"pipeline_id" gorm:"not null;index"`
	PipelineRunID  uint      `json:"pipeline_run_id" gorm:"not null;index"`
	FromEnvironment string   `json:"from_environment" gorm:"not null;size:255"`
	ToEnvironment  string    `json:"to_environment" gorm:"not null;size:255"`
	Status         string    `json:"status" gorm:"not null;size:30"` // pending / approved / rejected / auto_promoted
	PromotedBy     *uint     `json:"promoted_by,omitempty"`
	ApprovalID     *uint     `json:"approval_id,omitempty"`
	Reason         string    `json:"reason,omitempty" gorm:"type:text"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName 指定資料表名稱。
func (PromotionHistory) TableName() string { return "promotion_history" }

// ---------------------------------------------------------------------------
// Promotion status constants
// ---------------------------------------------------------------------------

const (
	PromotionStatusPending       = "pending"
	PromotionStatusApproved      = "approved"
	PromotionStatusRejected      = "rejected"
	PromotionStatusAutoPromoted  = "auto_promoted"
)
