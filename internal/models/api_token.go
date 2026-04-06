package models

import "time"

// APIToken 個人 API Token（SHA-256 hash 儲存，明文僅建立時回傳一次）
type APIToken struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	UserID     uint       `json:"user_id" gorm:"not null;index"`
	Name       string     `json:"name" gorm:"not null;size:100"`
	TokenHash  string     `json:"-" gorm:"not null;uniqueIndex;size:64"` // SHA-256 hex，不回傳
	Scopes     string     `json:"scopes" gorm:"size:200"`                // JSON 陣列，如 ["read","write"]
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
