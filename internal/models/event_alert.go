package models

import (
	"time"

	"gorm.io/gorm"
)

// EventAlertRule K8s Event 告警規則
type EventAlertRule struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	ClusterID   uint           `json:"clusterId" gorm:"index;not null"`
	Name        string         `json:"name" gorm:"size:100;not null"`
	Description string         `json:"description" gorm:"size:255"`
	Namespace   string         `json:"namespace" gorm:"size:100"`    // 空字串 = 全叢集
	EventReason string         `json:"eventReason" gorm:"size:100"`  // 例如 OOMKilling, BackOff
	EventType   string         `json:"eventType" gorm:"size:20"`     // Warning / Normal / "" (不限)
	MinCount    int            `json:"minCount" gorm:"default:1"`    // 觸發所需最小次數
	NotifyType  string         `json:"notifyType" gorm:"size:20"`    // webhook / email / dingtalk
	NotifyURL   string         `json:"notifyUrl" gorm:"size:500"`    // Webhook URL
	Enabled     bool           `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (EventAlertRule) TableName() string {
	return "event_alert_rules"
}

// EventAlertHistory Event 告警歷史紀錄
type EventAlertHistory struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	RuleID      uint      `json:"ruleId" gorm:"index;not null"`
	ClusterID   uint      `json:"clusterId" gorm:"index;not null"`
	RuleName    string    `json:"ruleName" gorm:"size:100"`
	Namespace   string    `json:"namespace" gorm:"size:100"`
	EventReason string    `json:"eventReason" gorm:"size:100"`
	EventType   string    `json:"eventType" gorm:"size:20"`
	Message     string    `json:"message" gorm:"type:text"`
	InvolvedObj string    `json:"involvedObj" gorm:"size:200"` // kind/name
	NotifyResult string   `json:"notifyResult" gorm:"size:50"` // sent / failed / disabled
	TriggeredAt time.Time `json:"triggeredAt" gorm:"index"`
}

// TableName 指定表名
func (EventAlertHistory) TableName() string {
	return "event_alert_histories"
}
