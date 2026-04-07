package models

import (
	"time"

	"gorm.io/gorm"
)

// NotifyChannel 通知渠道（全域設定）
type NotifyChannel struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	Name            string         `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Type            string         `json:"type" gorm:"size:20;not null"` // webhook / telegram / slack / teams
	WebhookURL      string         `json:"webhookUrl" gorm:"size:1000;not null"`
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
