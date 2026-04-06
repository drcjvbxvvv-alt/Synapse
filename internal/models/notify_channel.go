package models

import (
	"time"

	"gorm.io/gorm"
)

// NotifyChannel 通知渠道（全域設定）
type NotifyChannel struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	Name            string         `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Type            string         `json:"type" gorm:"size:20;not null"` // webhook / dingtalk / slack / teams
	WebhookURL      string         `json:"webhookUrl" gorm:"size:1000;not null"`
	DingTalkSecret  string         `json:"dingTalkSecret,omitempty" gorm:"size:200"` // DingTalk HMAC 簽名密鑰（選填）
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
