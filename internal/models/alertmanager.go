package models

import "time"

// AlertManagerConfig Alertmanager 配置
type AlertManagerConfig struct {
	Enabled              bool                   `json:"enabled"`
	Endpoint             string                 `json:"endpoint"` // http://alertmanager:9093
	Auth                 *MonitoringAuth        `json:"auth,omitempty"`
	Options              map[string]interface{} `json:"options,omitempty"`
	// ConfigMap 相關（用於 Receiver CRUD）
	ConfigMapNamespace   string                 `json:"configMapNamespace,omitempty"`
	ConfigMapName        string                 `json:"configMapName,omitempty"`
}

// Alert Alertmanager 告警
type Alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
	Status       AlertStatus       `json:"status"`
}

// AlertStatus 告警狀態
type AlertStatus struct {
	State       string   `json:"state"` // active, suppressed, resolved
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
}

// AlertGroup 告警分組
type AlertGroup struct {
	Labels   map[string]string `json:"labels"`
	Receiver string            `json:"receiver"`
	Alerts   []Alert           `json:"alerts"`
}

// AlertsResponse Alertmanager 告警響應
type AlertsResponse struct {
	Status string  `json:"status"`
	Data   []Alert `json:"data,omitempty"`
}

// AlertGroupsResponse Alertmanager 告警分組響應
type AlertGroupsResponse struct {
	Status string       `json:"status"`
	Data   []AlertGroup `json:"data,omitempty"`
}

// Silence 靜默規則
type Silence struct {
	ID        string        `json:"id"`
	Matchers  []Matcher     `json:"matchers"`
	StartsAt  time.Time     `json:"startsAt"`
	EndsAt    time.Time     `json:"endsAt"`
	CreatedBy string        `json:"createdBy"`
	Comment   string        `json:"comment"`
	Status    SilenceStatus `json:"status"`
	UpdatedAt time.Time     `json:"updatedAt,omitempty"`
}

// Matcher 匹配器
type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

// SilenceStatus 靜默狀態
type SilenceStatus struct {
	State string `json:"state"` // active, pending, expired
}

// CreateSilenceRequest 建立靜默規則請求
type CreateSilenceRequest struct {
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
}

// AlertManagerStatus Alertmanager 狀態
type AlertManagerStatus struct {
	Cluster     ClusterStatus `json:"cluster"`
	VersionInfo VersionInfo   `json:"versionInfo"`
	Config      ConfigInfo    `json:"config"`
	Uptime      time.Time     `json:"uptime"`
}

// ClusterStatus 叢集狀態
type ClusterStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Peers  []Peer `json:"peers"`
}

// Peer 對等節點
type Peer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// VersionInfo 版本資訊
type VersionInfo struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	Branch    string `json:"branch"`
	BuildUser string `json:"buildUser"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
}

// ConfigInfo 配置資訊
type ConfigInfo struct {
	Original string `json:"original"`
}

// Receiver 接收器（來自 Alertmanager /api/v2/receivers）
type Receiver struct {
	Name string `json:"name"`
}

// ReceiverConfig 完整 Receiver 配置（用於 CRUD）
type ReceiverConfig struct {
	Name          string           `json:"name" yaml:"name"`
	EmailConfigs  []EmailConfig    `json:"emailConfigs,omitempty" yaml:"email_configs,omitempty"`
	SlackConfigs  []SlackConfig    `json:"slackConfigs,omitempty" yaml:"slack_configs,omitempty"`
	WebhookConfigs []WebhookConfig `json:"webhookConfigs,omitempty" yaml:"webhook_configs,omitempty"`
	PagerdutyConfigs []PagerdutyConfig `json:"pagerdutyConfigs,omitempty" yaml:"pagerduty_configs,omitempty"`
	DingtalkConfigs []DingtalkConfig  `json:"dingtalkConfigs,omitempty" yaml:"dingtalk_configs,omitempty"`
}

// EmailConfig Email 告警配置
type EmailConfig struct {
	To           string            `json:"to" yaml:"to"`
	From         string            `json:"from,omitempty" yaml:"from,omitempty"`
	Smarthost    string            `json:"smarthost,omitempty" yaml:"smarthost,omitempty"`
	AuthUsername string            `json:"authUsername,omitempty" yaml:"auth_username,omitempty"`
	AuthPassword string            `json:"authPassword,omitempty" yaml:"auth_password,omitempty"`
	RequireTLS   *bool             `json:"requireTls,omitempty" yaml:"require_tls,omitempty"`
	Headers      map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// SlackConfig Slack 告警配置
type SlackConfig struct {
	APIURL   string `json:"apiUrl" yaml:"api_url"`
	Channel  string `json:"channel" yaml:"channel"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	IconEmoji string `json:"iconEmoji,omitempty" yaml:"icon_emoji,omitempty"`
	Text     string `json:"text,omitempty" yaml:"text,omitempty"`
	Title    string `json:"title,omitempty" yaml:"title,omitempty"`
}

// WebhookConfig Webhook 告警配置
type WebhookConfig struct {
	URL          string `json:"url" yaml:"url"`
	SendResolved bool   `json:"sendResolved,omitempty" yaml:"send_resolved,omitempty"`
	MaxAlerts    int    `json:"maxAlerts,omitempty" yaml:"max_alerts,omitempty"`
}

// PagerdutyConfig PagerDuty 告警配置
type PagerdutyConfig struct {
	RoutingKey  string `json:"routingKey" yaml:"routing_key"`
	ServiceKey  string `json:"serviceKey,omitempty" yaml:"service_key,omitempty"`
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// DingtalkConfig 釘釘告警配置
type DingtalkConfig struct {
	APIURL  string `json:"apiUrl" yaml:"api_url"`
	Secret  string `json:"secret,omitempty" yaml:"secret,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// AlertmanagerFullConfig 用於 YAML 解析的 Alertmanager 完整配置結構
type AlertmanagerFullConfig struct {
	Global    interface{}      `yaml:"global,omitempty"`
	Route     interface{}      `yaml:"route,omitempty"`
	Receivers []ReceiverConfig `yaml:"receivers,omitempty"`
	Templates []string         `yaml:"templates,omitempty"`
	InhibitRules interface{}   `yaml:"inhibit_rules,omitempty"`
}

// TestReceiverRequest 測試 Receiver 請求
type TestReceiverRequest struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// CreateReceiverRequest 建立 Receiver 請求
type CreateReceiverRequest = ReceiverConfig

// UpdateReceiverRequest 更新 Receiver 請求
type UpdateReceiverRequest = ReceiverConfig

// AlertStats 告警統計
type AlertStats struct {
	Total      int            `json:"total"`
	Firing     int            `json:"firing"`
	Pending    int            `json:"pending"`
	Resolved   int            `json:"resolved"`
	Suppressed int            `json:"suppressed"`
	BySeverity map[string]int `json:"bySeverity"`
}
