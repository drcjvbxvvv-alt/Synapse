package models

import (
	"github.com/shaia/Synapse/pkg/crypto"
	"gorm.io/gorm"
)

// HelmRepository Helm Chart 倉庫配置（全域共用）
type HelmRepository struct {
	gorm.Model
	Name     string `json:"name" gorm:"uniqueIndex;size:128"`
	URL      string `json:"url" gorm:"size:512"`
	Username string `json:"username,omitempty" gorm:"size:256"`
	Password string `json:"-" gorm:"type:text"` // 加密儲存，不對外暴露
}

// ---------------------------------------------------------------------------
// GORM hooks — AES-256-GCM encryption for Helm repository password (P2-3).
// ---------------------------------------------------------------------------

func (h *HelmRepository) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&h.Password)
}

func (h *HelmRepository) AfterCreate(_ *gorm.DB) error { return decryptFields(&h.Password) }
func (h *HelmRepository) AfterUpdate(_ *gorm.DB) error { return decryptFields(&h.Password) }
func (h *HelmRepository) AfterFind(_ *gorm.DB) error {
	if !crypto.IsEnabled() {
		return nil
	}
	return decryptFields(&h.Password)
}
