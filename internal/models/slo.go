package models

import (
	"time"

	"gorm.io/gorm"
)

// SLO 服務水準目標（Service Level Objective）
//
// SLI 量測：
//   - PromQuery（必填）：PromQL 運算式；若 TotalQuery 為空，此式須直接回傳 0.0-1.0 的比率
//     可用 $window 佔位符，服務自動代換為目前計算視窗（如 30d、1h）
//   - TotalQuery（選填）：當填入時，SLI = sum(rate(PromQuery[$w])) / sum(rate(TotalQuery[$w]))
//
// 燃燒率 (Burn Rate)：
//   BurnRate = (1 - SLI) / (1 - Target)
//   = 1.0 → 剛好在預算速率消耗
//   > 1.0 → 消耗超速（告警）
type SLO struct {
	ID          uint           `json:"id"          gorm:"primaryKey;autoIncrement"`
	ClusterID   uint           `json:"cluster_id"  gorm:"not null;index"`
	Name        string         `json:"name"        gorm:"not null;size:255"`
	Description string         `json:"description" gorm:"size:1024"`
	Namespace   string         `json:"namespace"   gorm:"size:128;index"` // 空值表示叢集層級

	// SLI 定義
	SLIType    string `json:"sli_type"    gorm:"size:32;not null"` // "availability" | "latency" | "error_rate" | "custom"
	PromQuery  string `json:"prom_query"  gorm:"type:text;not null"` // good events expr (or ratio, see above)
	TotalQuery string `json:"total_query" gorm:"type:text"`          // total events expr (optional)

	// SLO 目標
	Target float64 `json:"target" gorm:"not null"` // 0.0-1.0，例如 0.999 = 99.9%
	Window string  `json:"window" gorm:"size:16;not null"` // "7d" | "28d" | "30d"

	// 告警閾值（燃燒率倍數）
	BurnRateWarning  float64 `json:"burn_rate_warning"  gorm:"default:2"`  // 預設 2x
	BurnRateCritical float64 `json:"burn_rate_critical" gorm:"default:10"` // 預設 10x

	Enabled bool `json:"enabled" gorm:"default:true;not null"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (SLO) TableName() string { return "slos" }
