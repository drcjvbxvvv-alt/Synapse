package models

import "time"

// ---------------------------------------------------------------------------
// PipelineArtifact — Step 產出物記錄
// ---------------------------------------------------------------------------

// PipelineArtifact 記錄 Pipeline Step 的產出物（映像檔、掃描報告、Helm Chart 等）。
type PipelineArtifact struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	PipelineRunID uint       `json:"pipeline_run_id" gorm:"not null;index"`
	StepRunID     uint       `json:"step_run_id" gorm:"not null;index"`
	Kind          string     `json:"kind" gorm:"size:50"`           // image / jar / scan_report / yaml / helm_chart
	Name          string     `json:"name" gorm:"size:255"`
	Reference     string     `json:"reference" gorm:"type:text"`    // image digest / OSS URL / ImageScanResult ID
	SizeBytes     *int64     `json:"size_bytes"`
	MetadataJSON  string     `json:"metadata_json,omitempty" gorm:"type:text"` // 額外元資料（如 image labels）
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
}
