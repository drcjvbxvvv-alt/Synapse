package models

import (
	"time"

	"gorm.io/gorm"
)

// NamespaceProtection 命名空間保護設定
type NamespaceProtection struct {
	gorm.Model
	ClusterID       uint   `gorm:"not null;uniqueIndex:idx_ns_protection"`
	Namespace       string `gorm:"type:varchar(253);not null;uniqueIndex:idx_ns_protection"`
	RequireApproval bool   `gorm:"default:false"`
	Description     string // 保護原因說明
}

// ApprovalRequest 部署審批請求
type ApprovalRequest struct {
	gorm.Model
	ClusterID     uint       `gorm:"not null;index"`
	ClusterName   string     // 快取叢集名稱（便於跨叢集展示）
	Namespace     string     `gorm:"type:varchar(253);not null;index"`
	ResourceKind  string     `gorm:"not null"` // Deployment / StatefulSet / DaemonSet
	ResourceName  string     `gorm:"not null"`
	Action        string     `gorm:"not null"` // scale / delete / update / apply
	RequesterID   uint       `gorm:"not null"`
	RequesterName string
	ApproverID    *uint
	ApproverName  string
	Status        string     `gorm:"default:'pending';index"` // pending / approved / rejected / expired
	Payload       string     `gorm:"type:text"` // JSON 原始請求內容，審批透過後可重播
	Reason        string     // 審批人填寫的理由
	ExpiresAt     time.Time
	ApprovedAt    *time.Time
}
