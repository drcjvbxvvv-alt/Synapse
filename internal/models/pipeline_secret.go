package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PipelineSecret — CI/CD 專用密鑰，AES-256-GCM 加密儲存
// ---------------------------------------------------------------------------

// PipelineSecret 儲存 Pipeline 使用的敏感憑證（如 Harbor 密碼、Git Token）。
// 支援三種 Scope：global（全域）、environment（環境級）、pipeline（Pipeline 級）。
type PipelineSecret struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Scope       string         `json:"scope" gorm:"not null;size:20;uniqueIndex:uq_scope_name"` // global / environment / pipeline
	ScopeRef    *uint          `json:"scope_ref" gorm:"uniqueIndex:uq_scope_name"`              // environment_id 或 pipeline_id
	Name        string         `json:"name" gorm:"not null;size:100;uniqueIndex:uq_scope_name"` // 例：HARBOR_PASSWORD
	ValueEnc    string         `json:"-" gorm:"type:text;not null"`                              // AES-256-GCM 加密
	Description string         `json:"description" gorm:"size:255"`
	CreatedBy   uint           `json:"created_by" gorm:"not null"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// ---------------------------------------------------------------------------
// GORM hooks — transparent AES-256-GCM encryption for ValueEnc
// ---------------------------------------------------------------------------

// BeforeSave encrypts ValueEnc before INSERT or UPDATE.
func (s *PipelineSecret) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&s.ValueEnc)
}

func (s *PipelineSecret) afterSave() {
	_ = decryptFields(&s.ValueEnc)
}

// AfterCreate decrypts back to plaintext after CREATE.
func (s *PipelineSecret) AfterCreate(_ *gorm.DB) error {
	s.afterSave()
	return nil
}

// AfterUpdate decrypts back to plaintext after UPDATE.
func (s *PipelineSecret) AfterUpdate(_ *gorm.DB) error {
	s.afterSave()
	return nil
}

// AfterFind decrypts after loading from the database.
func (s *PipelineSecret) AfterFind(_ *gorm.DB) error {
	return decryptFields(&s.ValueEnc)
}
