package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Username     string         `json:"username" gorm:"uniqueIndex;not null;size:50"`
	PasswordHash string         `json:"-" gorm:"size:255"`
	Salt         string         `json:"-" gorm:"size:32"`
	Email        string         `json:"email" gorm:"size:100"`
	DisplayName  string         `json:"display_name" gorm:"size:100"`
	Phone        string         `json:"phone" gorm:"size:20"`
	AuthType     string         `json:"auth_type" gorm:"default:local;size:20"` // local, ldap
	Status       string         `json:"status" gorm:"default:active;size:20"`   // active, inactive, locked
	LastLoginAt  *time.Time     `json:"last_login_at"`
	LastLoginIP  string         `json:"last_login_ip" gorm:"size:50"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}
