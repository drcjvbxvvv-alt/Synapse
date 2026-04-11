package models

import "time"

// FeatureFlag represents a single feature flag persisted in the database.
// The Key is the canonical flag name (e.g. "use_repo_layer") and serves
// as the primary key — no auto-increment ID needed.
type FeatureFlag struct {
	Key         string    `json:"key"         gorm:"primaryKey;size:100"`
	Enabled     bool      `json:"enabled"     gorm:"default:false;not null"`
	Description string    `json:"description" gorm:"size:500"`
	UpdatedBy   string    `json:"updatedBy"   gorm:"column:updated_by;size:100"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (FeatureFlag) TableName() string { return "feature_flags" }
