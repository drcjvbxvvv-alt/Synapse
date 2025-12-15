package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Username     string         `json:"username" gorm:"uniqueIndex;not null;size:50"`
	PasswordHash string         `json:"-" gorm:"size:255"` // 本地用户的密码哈希
	Salt         string         `json:"-" gorm:"size:32"`  // 密码盐
	Email        string         `json:"email" gorm:"size:100"`
	DisplayName  string         `json:"display_name" gorm:"size:100"`           // 显示名称
	AuthType     string         `json:"auth_type" gorm:"default:local;size:20"` // 认证类型：local, ldap
	Status       string         `json:"status" gorm:"default:active;size:20"`   // 状态：active, inactive, locked
	LastLoginAt  *time.Time     `json:"last_login_at"`
	LastLoginIP  string         `json:"last_login_ip" gorm:"size:50"` // 最后登录IP
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联关系
	Roles []Role `json:"roles" gorm:"many2many:user_roles;"`
}

// Role 角色模型
type Role struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"uniqueIndex;not null;size:50"`
	Description string         `json:"description" gorm:"size:255"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联关系
	Users       []User       `json:"users" gorm:"many2many:user_roles;"`
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions;"`
}

// Permission 权限模型
type Permission struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Code        string         `json:"code" gorm:"uniqueIndex;not null;size:100"`
	Name        string         `json:"name" gorm:"not null;size:100"`
	Description string         `json:"description" gorm:"size:255"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联关系
	Roles []Role `json:"roles" gorm:"many2many:role_permissions;"`
}

// UserRole 用户角色关联表
type UserRole struct {
	UserID uint `json:"user_id" gorm:"primaryKey"`
	RoleID uint `json:"role_id" gorm:"primaryKey"`
}

// RolePermission 角色权限关联表
type RolePermission struct {
	RoleID       uint `json:"role_id" gorm:"primaryKey"`
	PermissionID uint `json:"permission_id" gorm:"primaryKey"`
}

// TableName 指定用户角色关联表名
func (UserRole) TableName() string {
	return "user_roles"
}

// TableName 指定角色权限关联表名
func (RolePermission) TableName() string {
	return "role_permissions"
}
