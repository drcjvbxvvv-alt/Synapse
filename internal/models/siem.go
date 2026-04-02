package models

import "gorm.io/gorm"

// SIEMWebhookConfig 稽核日誌 SIEM 匯出設定
type SIEMWebhookConfig struct {
	gorm.Model
	Enabled     bool   `gorm:"default:false"`
	WebhookURL  string `gorm:"not null"`
	SecretHeader string // Header 名稱，例如 X-Auth-Token
	SecretValue string  // Header 值（明文，小型部署可接受）
	BatchSize   int    `gorm:"default:100"` // 批次推送大小（保留，當前為即時推送）
}
