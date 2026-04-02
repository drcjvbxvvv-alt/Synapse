package models

import (
	"time"

	"gorm.io/gorm"
)

// ImageIndex 映像索引（跨叢集工作負載映像快照）
type ImageIndex struct {
	gorm.Model
	ClusterID     uint      `gorm:"not null;index"`
	ClusterName   string
	Namespace     string    `gorm:"not null;index"`
	WorkloadKind  string    `gorm:"not null"` // Deployment / StatefulSet / DaemonSet
	WorkloadName  string    `gorm:"not null"`
	ContainerName string    `gorm:"not null"`
	Image         string    `gorm:"not null;index"` // 完整映像（含 tag），如 nginx:1.21
	ImageName     string    `gorm:"not null;index"` // 不含 tag，如 nginx
	ImageTag      string    // tag 部分，如 1.21
	LastSyncAt    time.Time
}
