package models

import (
	"time"

	"gorm.io/gorm"
)

// SystemSetting 系統設定模型
type SystemSetting struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	ConfigKey string         `json:"key" gorm:"column:config_key;uniqueIndex;not null;size:100"` // 配置鍵
	Value     string         `json:"value" gorm:"type:text"`                                     // 配置值（JSON格式）
	Type      string         `json:"type" gorm:"size:50"`                                        // 配置型別：ldap, smtp, etc.
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// LDAPConfig LDAP配置結構
type LDAPConfig struct {
	Enabled         bool   `json:"enabled"`           // 是否啟用LDAP
	Server          string `json:"server"`            // LDAP伺服器地址
	Port            int    `json:"port"`              // LDAP連接埠
	UseTLS          bool   `json:"use_tls"`           // 是否使用TLS
	SkipTLSVerify   bool   `json:"skip_tls_verify"`   // 是否跳過TLS驗證
	BindDN          string `json:"bind_dn"`           // 繫結DN
	BindPassword    string `json:"bind_password"`     // 繫結密碼
	BaseDN          string `json:"base_dn"`           // 搜尋基礎DN
	UserFilter      string `json:"user_filter"`       // 使用者搜尋過濾器
	UsernameAttr    string `json:"username_attr"`     // 使用者名稱屬性
	EmailAttr       string `json:"email_attr"`        // 郵箱屬性
	DisplayNameAttr string `json:"display_name_attr"` // 顯示名稱屬性
	GroupFilter     string `json:"group_filter"`      // 組搜尋過濾器
	GroupAttr       string `json:"group_attr"`        // 組屬性
}

// GetDefaultLDAPConfig 獲取預設LDAP配置
func GetDefaultLDAPConfig() LDAPConfig {
	return LDAPConfig{
		Enabled:         false,
		Server:          "",
		Port:            389,
		UseTLS:          false,
		SkipTLSVerify:   false,
		BindDN:          "",
		BindPassword:    "",
		BaseDN:          "",
		UserFilter:      "(uid=%s)",
		UsernameAttr:    "uid",
		EmailAttr:       "mail",
		DisplayNameAttr: "cn",
		GroupFilter:     "(memberUid=%s)",
		GroupAttr:       "cn",
	}
}

// SSHConfig 全域性SSH配置結構
type SSHConfig struct {
	Enabled    bool   `json:"enabled"`     // 是否啟用全域性SSH配置
	Username   string `json:"username"`    // SSH使用者名稱，預設 root
	Port       int    `json:"port"`        // SSH連接埠，預設 22
	AuthType   string `json:"auth_type"`   // 認證方式: password 或 key
	Password   string `json:"password"`    // 密碼（加密儲存）
	PrivateKey string `json:"private_key"` // 私鑰內容
}

// GetDefaultSSHConfig 獲取預設SSH配置
func GetDefaultSSHConfig() SSHConfig {
	return SSHConfig{
		Enabled:  false,
		Username: "root",
		Port:     22,
		AuthType: "password",
	}
}

// GrafanaSettingConfig Grafana 系統配置結構（儲存在 system_settings 表中）
type GrafanaSettingConfig struct {
	URL    string `json:"url"`     // Grafana 地址，如 http://grafana:3000
	APIKey string `json:"api_key"` // Grafana Service Account Token 或 API Key
}

// GetDefaultGrafanaSettingConfig 獲取預設 Grafana 配置
func GetDefaultGrafanaSettingConfig() GrafanaSettingConfig {
	return GrafanaSettingConfig{
		URL:    "",
		APIKey: "",
	}
}

// SystemSecurityConfig 登入安全設定（存於 system_settings，key=security_config）
type SystemSecurityConfig struct {
	SessionTTLMinutes      int `json:"session_ttl_minutes"`       // Session 逾時（分鐘）
	LoginFailLockThreshold int `json:"login_fail_lock_threshold"` // 登入失敗鎖定閾值
	LockDurationMinutes    int `json:"lock_duration_minutes"`     // 鎖定持續時間（分鐘）
	PasswordMinLength      int `json:"password_min_length"`       // 密碼最短長度
}

// GetDefaultSystemSecurityConfig 取得預設安全設定
func GetDefaultSystemSecurityConfig() SystemSecurityConfig {
	return SystemSecurityConfig{
		SessionTTLMinutes:      480,
		LoginFailLockThreshold: 5,
		LockDurationMinutes:    30,
		PasswordMinLength:      8,
	}
}

// TableName 指定表名
func (SystemSetting) TableName() string {
	return "system_settings"
}
