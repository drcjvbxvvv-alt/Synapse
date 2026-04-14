package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// GitProvider — Git 版本控制提供者（GitHub / GitLab / Gitea）
//
// 設計（CICD_ARCHITECTURE §10.4）：
//   - 儲存 API 連線資訊 + Token（AES-256-GCM 加密）
//   - Webhook Secret 獨立加密（HMAC 驗證用）
//   - WebhookToken 為 URL 路徑中的辨識 token（明文，用於快速查找）
// ---------------------------------------------------------------------------

// Git Provider 類型常數
const (
	GitProviderTypeGitHub = "github"
	GitProviderTypeGitLab = "gitlab"
	GitProviderTypeGitea  = "gitea"
)

// GitProvider 儲存 Git 版本控制提供者的連線資訊。
type GitProvider struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	Name             string         `json:"name" gorm:"not null;size:255;uniqueIndex"`
	Type             string         `json:"type" gorm:"not null;size:50"` // github / gitlab / gitea
	BaseURL          string         `json:"base_url" gorm:"not null;size:512"`
	AccessTokenEnc   string         `json:"-" gorm:"type:text"`           // AES-256-GCM 加密
	WebhookSecretEnc string         `json:"-" gorm:"type:text"`           // AES-256-GCM 加密
	WebhookToken     string         `json:"webhook_token" gorm:"not null;size:64;uniqueIndex"` // URL 路徑辨識 token
	Enabled          bool           `json:"enabled" gorm:"default:true"`
	CreatedBy        uint           `json:"created_by" gorm:"not null"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (GitProvider) TableName() string { return "git_providers" }

// ---------------------------------------------------------------------------
// GORM hooks — transparent AES-256-GCM encryption
// ---------------------------------------------------------------------------

func (g *GitProvider) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&g.AccessTokenEnc, &g.WebhookSecretEnc)
}

func (g *GitProvider) afterSave() {
	_ = decryptFields(&g.AccessTokenEnc, &g.WebhookSecretEnc)
}

func (g *GitProvider) AfterCreate(_ *gorm.DB) error {
	g.afterSave()
	return nil
}

func (g *GitProvider) AfterUpdate(_ *gorm.DB) error {
	g.afterSave()
	return nil
}

func (g *GitProvider) AfterFind(_ *gorm.DB) error {
	return decryptFields(&g.AccessTokenEnc, &g.WebhookSecretEnc)
}
