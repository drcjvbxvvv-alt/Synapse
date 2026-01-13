package models

import (
	"time"

	"gorm.io/gorm"
)

// LogEntry 统一日志条目模型
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

// LogQuery 日志查询参数
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

// LogStats 日志统计模型
type LogStats struct {
	TotalCount       int64           `json:"total_count"`
	ErrorCount       int64           `json:"error_count"`
	WarnCount        int64           `json:"warn_count"`
	InfoCount        int64           `json:"info_count"`
	TimeDistribution []TimePoint     `json:"time_distribution,omitempty"`
	NamespaceStats   []NamespaceStat `json:"namespace_stats,omitempty"`
	LevelStats       []LevelStat     `json:"level_stats,omitempty"`
}

// TimePoint 时间点统计
type TimePoint struct {
	Time  time.Time `json:"time"`
	Count int64     `json:"count"`
}

// NamespaceStat 命名空间统计
type NamespaceStat struct {
	Namespace string `json:"namespace"`
	Count     int64  `json:"count"`
}

// LevelStat 日志级别统计
type LevelStat struct {
	Level string `json:"level"`
	Count int64  `json:"count"`
}

// LogStreamConfig 日志流配置
type LogStreamConfig struct {
	ClusterID     uint              `json:"cluster_id"`
	Targets       []LogStreamTarget `json:"targets"`
	TailLines     int64             `json:"tail_lines"`
	SinceSeconds  int64             `json:"since_seconds"`
	ShowTimestamp bool              `json:"show_timestamp"`
	ShowSource    bool              `json:"show_source"`
}

// LogStreamTarget 日志流目标
type LogStreamTarget struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

// LogStreamOptions 日志流选项
type LogStreamOptions struct {
	TailLines     int64
	SinceSeconds  int64
	Previous      bool
	ShowTimestamp bool
}

// LogSourceConfig 外部日志源配置
type LogSourceConfig struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	ClusterID uint           `json:"cluster_id" gorm:"not null;index"`
	Type      string         `json:"type" gorm:"size:20"`  // loki, elasticsearch
	Name      string         `json:"name" gorm:"size:100"` // 日志源名称
	URL       string         `json:"url" gorm:"size:255"`
	Username  string         `json:"username,omitempty" gorm:"size:100"`
	Password  string         `json:"-" gorm:"size:255"` // 加密存储
	APIKey    string         `json:"-" gorm:"size:255"`
	Enabled   bool           `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定日志源配置表名
func (LogSourceConfig) TableName() string {
	return "log_source_configs"
}

// EventLogEntry K8s事件日志条目
type EventLogEntry struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`             // Normal, Warning
	Reason          string    `json:"reason"`           // 事件原因
	Message         string    `json:"message"`          // 事件消息
	Count           int32     `json:"count"`            // 发生次数
	FirstTimestamp  time.Time `json:"first_timestamp"`  // 首次发生时间
	LastTimestamp   time.Time `json:"last_timestamp"`   // 最后发生时间
	Namespace       string    `json:"namespace"`        // 命名空间
	InvolvedKind    string    `json:"involved_kind"`    // 关联资源类型
	InvolvedName    string    `json:"involved_name"`    // 关联资源名称
	SourceComponent string    `json:"source_component"` // 事件来源组件
	SourceHost      string    `json:"source_host"`      // 事件来源主机
}
