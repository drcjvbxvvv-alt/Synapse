package models

import (
	"time"

	"gorm.io/gorm"
)

// LogEntry 統一日誌條目模型
type LogEntry struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`  // container, event, audit
	Level       string                 `json:"level"` // debug, info, warn, error
	ClusterID   uint                   `json:"cluster_id"`
	ClusterName string                 `json:"cluster_name"`
	Namespace   string                 `json:"namespace"`
	PodName     string                 `json:"pod_name"`
	Container   string                 `json:"container"`
	NodeName    string                 `json:"node_name"`
	Message     string                 `json:"message"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LogQuery 日誌查詢參數
type LogQuery struct {
	ClusterID  uint      `form:"clusterId"`
	Namespaces []string  `form:"namespaces"`
	Pods       []string  `form:"pods"`
	Containers []string  `form:"containers"`
	Nodes      []string  `form:"nodes"`
	LogTypes   []string  `form:"logTypes"`
	Levels     []string  `form:"levels"`
	Keyword    string    `form:"keyword"`
	Regex      string    `form:"regex"`
	StartTime  time.Time `form:"startTime"`
	EndTime    time.Time `form:"endTime"`
	Limit      int       `form:"limit"`
	Offset     int       `form:"offset"`
	Direction  string    `form:"direction"` // forward, backward
}

// LogStats 日誌統計模型
type LogStats struct {
	TotalCount       int64           `json:"total_count"`
	ErrorCount       int64           `json:"error_count"`
	WarnCount        int64           `json:"warn_count"`
	InfoCount        int64           `json:"info_count"`
	TimeDistribution []TimePoint     `json:"time_distribution,omitempty"`
	NamespaceStats   []NamespaceStat `json:"namespace_stats,omitempty"`
	LevelStats       []LevelStat     `json:"level_stats,omitempty"`
}

// TimePoint 時間點統計
type TimePoint struct {
	Time  time.Time `json:"time"`
	Count int64     `json:"count"`
}

// NamespaceStat 命名空間統計
type NamespaceStat struct {
	Namespace string `json:"namespace"`
	Count     int64  `json:"count"`
}

// LevelStat 日誌級別統計
type LevelStat struct {
	Level string `json:"level"`
	Count int64  `json:"count"`
}

// LogStreamConfig 日誌流配置
type LogStreamConfig struct {
	ClusterID     uint              `json:"cluster_id"`
	Targets       []LogStreamTarget `json:"targets"`
	TailLines     int64             `json:"tail_lines"`
	SinceSeconds  int64             `json:"since_seconds"`
	ShowTimestamp bool              `json:"show_timestamp"`
	ShowSource    bool              `json:"show_source"`
}

// LogStreamTarget 日誌流目標
type LogStreamTarget struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

// LogStreamOptions 日誌流選項
type LogStreamOptions struct {
	TailLines     int64
	SinceSeconds  int64
	Previous      bool
	ShowTimestamp bool
}

// LogSourceConfig 外部日誌源配置
type LogSourceConfig struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	ClusterID uint           `json:"cluster_id" gorm:"not null;index"`
	Type      string         `json:"type" gorm:"size:20"`  // loki, elasticsearch
	Name      string         `json:"name" gorm:"size:100"` // 日誌源名稱
	URL       string         `json:"url" gorm:"size:255"`
	Username  string         `json:"username,omitempty" gorm:"size:100"`
	Password  string         `json:"-" gorm:"size:255"` // 加密儲存
	APIKey    string         `json:"-" gorm:"size:255"`
	Enabled   bool           `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定日誌源配置表名
func (LogSourceConfig) TableName() string {
	return "log_source_configs"
}

// EventLogEntry K8s事件日誌條目
type EventLogEntry struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`             // Normal, Warning
	Reason          string    `json:"reason"`           // 事件原因
	Message         string    `json:"message"`          // 事件訊息
	Count           int32     `json:"count"`            // 發生次數
	FirstTimestamp  time.Time `json:"first_timestamp"`  // 首次發生時間
	LastTimestamp   time.Time `json:"last_timestamp"`   // 最後發生時間
	Namespace       string    `json:"namespace"`        // 命名空間
	InvolvedKind    string    `json:"involved_kind"`    // 關聯資源型別
	InvolvedName    string    `json:"involved_name"`    // 關聯資源名稱
	SourceComponent string    `json:"source_component"` // 事件來源元件
	SourceHost      string    `json:"source_host"`      // 事件來源主機
}
