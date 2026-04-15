package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// TagRetentionPolicy — Registry Tag 保留策略（CICD_ARCHITECTURE §11, M15）
//
// 設計：
//   - 每個 Registry 可設定多條保留規則
//   - 規則由 cron 排程評估或手動觸發
//   - 保留模式：keep_last_n / keep_by_age / keep_by_pattern
// ---------------------------------------------------------------------------

// 保留策略類型
const (
	RetentionKeepLastN    = "keep_last_n"    // 保留最近 N 個 tag
	RetentionKeepByAge    = "keep_by_age"    // 保留 N 天內的 tag
	RetentionKeepByRegex  = "keep_by_regex"  // 保留匹配 regex 的 tag
)

// TagRetentionPolicy 定義 Registry 的 Tag 保留策略。
type TagRetentionPolicy struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	RegistryID     uint           `json:"registry_id" gorm:"not null;index"`
	RepositoryMatch string        `json:"repository_match" gorm:"not null;size:512"` // glob pattern, e.g. "myapp/*" or "*"
	TagMatch       string         `json:"tag_match" gorm:"size:255;default:'*'"`     // glob pattern for tags
	RetentionType  string         `json:"retention_type" gorm:"not null;size:30"`    // keep_last_n | keep_by_age | keep_by_regex
	KeepCount      int            `json:"keep_count,omitempty"`                      // for keep_last_n
	KeepDays       int            `json:"keep_days,omitempty"`                       // for keep_by_age
	KeepPattern    string         `json:"keep_pattern,omitempty" gorm:"size:512"`    // regex for keep_by_regex
	Enabled        bool           `json:"enabled" gorm:"default:true"`
	CronExpr       string         `json:"cron_expr,omitempty" gorm:"size:100"`       // e.g. "0 3 * * 0" (weekly at 3am)
	LastRunAt      *time.Time     `json:"last_run_at,omitempty"`
	LastRunResult  string         `json:"last_run_result,omitempty" gorm:"type:text"` // JSON summary
	CreatedBy      uint           `json:"created_by" gorm:"not null"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (TagRetentionPolicy) TableName() string { return "tag_retention_policies" }
