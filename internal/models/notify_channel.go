package models

import (
	"time"

	"github.com/shaia/Synapse/pkg/crypto"
	"gorm.io/gorm"
)

// NotifyChannel 通知渠道（全域設定）
type NotifyChannel struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	Name            string         `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Type            string         `json:"type" gorm:"size:20;not null"` // webhook / telegram / slack / teams
	WebhookURL      string         `json:"webhookUrl" gorm:"type:text;not null"` // 加密儲存；webhook URL 可能含認證 token
	TelegramChatID  string         `json:"telegramChatId,omitempty" gorm:"column:telegram_chat_id;size:200"` // Telegram Chat ID（Type 為 telegram 時必填）
	Description     string         `json:"description" gorm:"size:255"`
	Enabled         bool           `json:"enabled" gorm:"default:true"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (NotifyChannel) TableName() string {
	return "notify_channels"
}

// ---------------------------------------------------------------------------
// GORM hooks — AES-256-GCM encryption for webhook URL (P2-3).
// Webhook URLs often embed authentication tokens in the path or query string.
// ---------------------------------------------------------------------------

func (n *NotifyChannel) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&n.WebhookURL)
}

func (n *NotifyChannel) AfterCreate(_ *gorm.DB) error { return decryptFields(&n.WebhookURL) }
func (n *NotifyChannel) AfterUpdate(_ *gorm.DB) error { return decryptFields(&n.WebhookURL) }
func (n *NotifyChannel) AfterFind(_ *gorm.DB) error {
	if !crypto.IsEnabled() {
		return nil
	}
	return decryptFields(&n.WebhookURL)
}
