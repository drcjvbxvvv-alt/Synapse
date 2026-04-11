package models

import (
	"github.com/shaia/Synapse/pkg/crypto"
	"gorm.io/gorm"
)

// SIEMWebhookConfig 稽核日誌 SIEM 匯出設定
type SIEMWebhookConfig struct {
	gorm.Model
	Enabled      bool   `gorm:"default:false"`
	WebhookURL   string `gorm:"not null"`
	SecretHeader string  // Header 名稱，例如 X-Auth-Token
	SecretValue  string  `json:"-"` // Header 值（加密儲存）
	BatchSize    int    `gorm:"default:100"` // 批次推送大小（保留，當前為即時推送）
}

// ---------------------------------------------------------------------------
// GORM hooks — AES-256-GCM encryption for SIEM webhook secret (P2-3).
// ---------------------------------------------------------------------------

func (s *SIEMWebhookConfig) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&s.SecretValue)
}

func (s *SIEMWebhookConfig) AfterCreate(_ *gorm.DB) error { return decryptFields(&s.SecretValue) }
func (s *SIEMWebhookConfig) AfterUpdate(_ *gorm.DB) error { return decryptFields(&s.SecretValue) }
func (s *SIEMWebhookConfig) AfterFind(_ *gorm.DB) error {
	if !crypto.IsEnabled() {
		return nil
	}
	return decryptFields(&s.SecretValue)
}
