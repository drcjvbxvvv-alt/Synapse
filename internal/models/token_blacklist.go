package models

import "time"

// TokenBlacklist JWT Token 黑名單
// 設計目標：提供 JWT 撤銷能力（P0-5）
//
// Token 的 `jti` claim 寫入此表即視為已撤銷，
// AuthRequired 中介軟體每次驗證時會查詢此表；
// 過期（ExpiresAt < now）的記錄由背景 worker 清理。
type TokenBlacklist struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	JTI       string    `json:"jti" gorm:"uniqueIndex;not null;size:64"` // JWT ID
	UserID    uint      `json:"user_id" gorm:"index"`                    // 便於「登出此使用者所有 token」查詢
	Reason    string    `json:"reason" gorm:"size:64"`                   // logout / password_change / forced
	ExpiresAt time.Time `json:"expires_at" gorm:"index;not null"`        // 原 token 的 exp，用於清理
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定資料表名稱
func (TokenBlacklist) TableName() string { return "token_blacklists" }

// Token 撤銷原因
const (
	TokenRevokeReasonLogout         = "logout"
	TokenRevokeReasonPasswordChange = "password_change"
	TokenRevokeReasonForced         = "forced"
)
