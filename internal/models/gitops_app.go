package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// GitOpsApp — GitOps 應用（CICD_ARCHITECTURE §12）
//
// 設計：
//   - source = "native"：Synapse 原生 GitOps（M16）
//   - source = "argocd"：ArgoCD 代理（現有 argocd_service.go）
//   - 同一 App 不可同時被兩種 source 管理（互斥規則 §12.1）
//   - render_type 支援 raw / kustomize / helm
//   - sync_policy 控制 auto sync 或 manual
// ---------------------------------------------------------------------------

// GitOps 來源類型
const (
	GitOpsSourceNative = "native"
	GitOpsSourceArgoCD = "argocd"
)

// GitOps render 類型
const (
	GitOpsRenderRaw       = "raw"       // 純 YAML manifest
	GitOpsRenderKustomize = "kustomize" // Kustomize overlay
	GitOpsRenderHelm      = "helm"      // Helm Chart
)

// GitOps sync 策略
const (
	GitOpsSyncPolicyAuto   = "auto"
	GitOpsSyncPolicyManual = "manual"
)

// GitOps 應用狀態
const (
	GitOpsStatusSynced  = "synced"
	GitOpsStatusDrifted = "drifted"
	GitOpsStatusError   = "error"
	GitOpsStatusUnknown = "unknown"
	GitOpsStatusSyncing = "syncing"
)

// GitOpsApp 儲存 GitOps 應用設定。
type GitOpsApp struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Name          string         `json:"name" gorm:"not null;size:255;uniqueIndex:idx_gitops_name_cluster"`
	Source        string         `json:"source" gorm:"not null;size:20;default:'native';index"` // native / argocd
	GitProviderID *uint          `json:"git_provider_id,omitempty" gorm:"index"`
	RepoURL       string         `json:"repo_url,omitempty" gorm:"size:512"`
	Branch        string         `json:"branch,omitempty" gorm:"size:255"`
	Path          string         `json:"path,omitempty" gorm:"size:512"` // repo 中的路徑
	RenderType    string         `json:"render_type" gorm:"not null;size:50;default:'raw'"`
	HelmValues    string         `json:"helm_values,omitempty" gorm:"type:text"` // JSON
	ClusterID     uint           `json:"cluster_id" gorm:"not null;index;uniqueIndex:idx_gitops_name_cluster"`
	Namespace     string         `json:"namespace" gorm:"not null;size:253"`
	SyncPolicy    string         `json:"sync_policy" gorm:"not null;size:50;default:'manual'"`
	SyncInterval  int            `json:"sync_interval" gorm:"default:300"` // 秒
	LastSyncedAt  *time.Time     `json:"last_synced_at,omitempty"`
	LastDiffAt    *time.Time     `json:"last_diff_at,omitempty"`
	LastDiffResult string        `json:"last_diff_result,omitempty" gorm:"type:text"` // JSON diff summary
	Status        string         `json:"status" gorm:"not null;size:50;default:'unknown';index"`
	StatusMessage    string         `json:"status_message,omitempty" gorm:"type:text"`
	NotifyChannelIDs string        `json:"notify_channel_ids,omitempty" gorm:"type:text"` // JSON 陣列: [1,2,3]
	CreatedBy        uint          `json:"created_by" gorm:"not null"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (GitOpsApp) TableName() string { return "gitops_apps" }
