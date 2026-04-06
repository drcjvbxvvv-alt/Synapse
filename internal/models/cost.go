package models

import "time"

// CostConfig 叢整合本定價設定
type CostConfig struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ClusterID       uint      `json:"cluster_id" gorm:"uniqueIndex;not null"`
	CpuPricePerCore float64   `json:"cpu_price_per_core" gorm:"default:0.048"`  // USD / core / hour
	MemPricePerGiB  float64   `json:"mem_price_per_gib" gorm:"default:0.006"`   // USD / GiB / hour
	Currency        string    `json:"currency" gorm:"size:10;default:USD"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (CostConfig) TableName() string { return "cost_configs" }

// ResourceSnapshot 每日資源快照（按命名空間 + 工作負載）
type ResourceSnapshot struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ClusterID  uint      `json:"cluster_id" gorm:"index;not null"`
	Namespace  string    `json:"namespace" gorm:"size:128;not null"`
	Workload   string    `json:"workload" gorm:"size:256;not null"` // "Deployment/app-name"
	Date       time.Time `json:"date" gorm:"index;not null"`        // 精確到日，UTC 00:00
	CpuRequest float64   `json:"cpu_request"`                       // millicores
	CpuUsage   float64   `json:"cpu_usage"`                         // millicores
	MemRequest float64   `json:"mem_request"`                       // MiB
	MemUsage   float64   `json:"mem_usage"`                         // MiB
	PodCount   int       `json:"pod_count"`
}

func (ResourceSnapshot) TableName() string { return "resource_snapshots" }
