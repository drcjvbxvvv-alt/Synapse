package models

import "time"

// ---------------------------------------------------------------------------
// PipelineLog — Step 執行日誌（分塊儲存）
// ---------------------------------------------------------------------------

// PipelineLog 以分塊方式儲存 Step 的執行日誌，每塊最大 1MB。
// 透過 step_run_id + chunk_seq 排序即可還原完整日誌流。
type PipelineLog struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	PipelineRunID uint      `json:"pipeline_run_id" gorm:"not null;index"`
	StepRunID     uint      `json:"step_run_id" gorm:"not null;uniqueIndex:idx_step_chunk"`
	ChunkSeq      int       `json:"chunk_seq" gorm:"not null;uniqueIndex:idx_step_chunk"` // 分塊序號
	Content       string    `json:"content" gorm:"type:text"`                              // 單塊最大 1MB
	StoredAt      time.Time `json:"stored_at"`
}
