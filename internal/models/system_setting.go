package models

import (
	"time"

	"gorm.io/gorm"
)

// SystemSetting 系统设置模型
type SystemSetting struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	ConfigKey string         `json:"key" gorm:"column:config_key;uniqueIndex;not null;size:100"` // 配置键
	Value     string         `json:"value" gorm:"type:text"`                                     // 配置值（JSON格式）
	Type      string         `json:"type" gorm:"size:50"`                                        // 配置类型：ldap, smtp, etc.
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// LDAPConfig LDAP配置结构
type LDAPConfig struct {
	Enabled         bool   `json:"enabled"`           // 是否启用LDAP
	Server          string `json:"server"`            // LDAP服务器地址
	Port            int    `json:"port"`              // LDAP端口
	UseTLS          bool   `json:"use_tls"`           // 是否使用TLS
	SkipTLSVerify   bool   `json:"skip_tls_verify"`   // 是否跳过TLS验证
	BindDN          string `json:"bind_dn"`           // 绑定DN
	BindPassword    string `json:"bind_password"`     // 绑定密码
	BaseDN          string `json:"base_dn"`           // 搜索基础DN
	UserFilter      string `json:"user_filter"`       // 用户搜索过滤器
	UsernameAttr    string `json:"username_attr"`     // 用户名属性
	EmailAttr       string `json:"email_attr"`        // 邮箱属性
	DisplayNameAttr string `json:"display_name_attr"` // 显示名称属性
	GroupFilter     string `json:"group_filter"`      // 组搜索过滤器
	GroupAttr       string `json:"group_attr"`        // 组属性
}

// GetDefaultLDAPConfig 获取默认LDAP配置
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

// TableName 指定表名
func (SystemSetting) TableName() string {
	return "system_settings"
}
