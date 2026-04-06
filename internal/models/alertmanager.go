package models

import "time"

// AlertManagerConfig Alertmanager 配置
type AlertManagerConfig struct {
	Enabled  bool                   `json:"enabled"`
	Endpoint string                 `json:"endpoint"` // http://alertmanager:9093
	Auth     *MonitoringAuth        `json:"auth,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
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

// Receiver 接收器
type Receiver struct {
	Name string `json:"name"`
}

// AlertStats 告警統計
type AlertStats struct {
	Total      int            `json:"total"`
	Firing     int            `json:"firing"`
	Pending    int            `json:"pending"`
	Resolved   int            `json:"resolved"`
	Suppressed int            `json:"suppressed"`
	BySeverity map[string]int `json:"bySeverity"`
}
