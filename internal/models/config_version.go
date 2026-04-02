package models

import "time"

// ConfigVersion ConfigMap / Secret 版本快照
type ConfigVersion struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClusterID    uint      `json:"clusterId" gorm:"not null;index:idx_config_ver"`
	ResourceType string    `json:"resourceType" gorm:"size:20;not null;index:idx_config_ver"` // configmap | secret
	Namespace    string    `json:"namespace" gorm:"size:255;not null;index:idx_config_ver"`
	Name         string    `json:"name" gorm:"size:255;not null;index:idx_config_ver"`
	Version      int       `json:"version" gorm:"not null"`
	ContentJSON  string    `json:"contentJSON" gorm:"type:text"` // Secret 內容以 AES-256-GCM 加密
	ChangedBy    string    `json:"changedBy" gorm:"size:100"`
	ChangedAt    time.Time `json:"changedAt"`
}

func (ConfigVersion) TableName() string { return "config_versions" }
