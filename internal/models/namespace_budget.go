package models

import "time"

// NamespaceBudget 命名空間預算設定
type NamespaceBudget struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClusterID        uint      `json:"cluster_id" gorm:"uniqueIndex:idx_budget_cluster_ns;not null;index"`
	Namespace        string    `json:"namespace" gorm:"uniqueIndex:idx_budget_cluster_ns;size:128;not null"`
	CPUCoresLimit    float64   `json:"cpu_cores_limit" gorm:"default:0"`       // max CPU cores (0=unlimited)
	MemoryGiBLimit   float64   `json:"memory_gib_limit" gorm:"default:0"`      // max memory GiB (0=unlimited)
	MonthlyCostLimit float64   `json:"monthly_cost_limit" gorm:"default:0"`    // max monthly cost USD (0=unlimited)
	AlertThreshold   float64   `json:"alert_threshold" gorm:"default:0.8"`     // alert when usage > threshold (0.0-1.0)
	Enabled          bool      `json:"enabled" gorm:"default:true;not null"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (NamespaceBudget) TableName() string { return "namespace_budgets" }
