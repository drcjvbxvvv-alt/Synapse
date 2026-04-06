package models

import "time"

// SyncPolicy 多叢集配置同步策略
type SyncPolicy struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"size:128;not null"`
	Description     string    `json:"description" gorm:"size:512"`
	SourceClusterID uint      `json:"source_cluster_id" gorm:"index;not null"`
	SourceNamespace string    `json:"source_namespace" gorm:"size:128;not null"`
	ResourceType    string    `json:"resource_type" gorm:"size:32;not null"` // "ConfigMap" / "Secret"
	ResourceNames   string    `json:"resource_names" gorm:"type:text"`        // JSON 陣列
	TargetClusters  string    `json:"target_clusters" gorm:"type:text"`       // JSON 陣列（叢集 ID）
	ConflictPolicy  string    `json:"conflict_policy" gorm:"size:16;default:skip"` // overwrite / skip
	Schedule        string    `json:"schedule" gorm:"size:64"`               // Cron 表示式，空表示手動
	Enabled         bool      `json:"enabled" gorm:"default:true"`
	LastSyncAt      *time.Time `json:"last_sync_at"`
	LastSyncStatus  string    `json:"last_sync_status" gorm:"size:16"` // success / partial / failed
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (SyncPolicy) TableName() string { return "sync_policies" }

// SyncHistory 同步歷史紀錄
type SyncHistory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	PolicyID     uint      `json:"policy_id" gorm:"index;not null"`
	TriggeredBy  string    `json:"triggered_by" gorm:"size:64"` // "manual" / "schedule"
	Status       string    `json:"status" gorm:"size:16"`       // success / partial / failed
	Message      string    `json:"message" gorm:"type:text"`
	Details      string    `json:"details" gorm:"type:text"` // JSON：各目標叢集結果
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

func (SyncHistory) TableName() string { return "sync_histories" }
