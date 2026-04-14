package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Registry — 映像 Registry 連線設定（Harbor / Docker Hub / ECR / GCR）
//
// 設計（CICD_ARCHITECTURE §11）：
//   - 儲存 Registry 連線資訊 + 密碼（AES-256-GCM 加密）
//   - CA Bundle 獨立加密（自簽 CA 用）
//   - insecure_tls 預設 false，符合 CLAUDE.md §10 規則
// ---------------------------------------------------------------------------

// Registry 類型常數
const (
	RegistryTypeHarbor    = "harbor"
	RegistryTypeDockerHub = "dockerhub"
	RegistryTypeACR       = "acr" // 阿里雲 ACR
	RegistryTypeECR       = "ecr" // AWS ECR
	RegistryTypeGCR       = "gcr" // GCR / GAR
)

// Registry 儲存映像 Registry 的連線設定。
type Registry struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	Name           string         `json:"name" gorm:"not null;size:255;uniqueIndex"`
	Type           string         `json:"type" gorm:"not null;size:50"` // harbor / dockerhub / ecr / gcr / acr
	URL            string         `json:"url" gorm:"not null;size:512"`
	Username       string         `json:"username,omitempty" gorm:"size:255"`
	PasswordEnc    string         `json:"-" gorm:"type:text"`          // AES-256-GCM 加密
	InsecureTLS    bool           `json:"insecure_tls" gorm:"default:false"`
	CABundleEnc    string         `json:"-" gorm:"type:text"`          // AES-256-GCM 加密（自簽 CA）
	DefaultProject string        `json:"default_project,omitempty" gorm:"size:255"` // Harbor project 預設值
	Enabled        bool           `json:"enabled" gorm:"default:true"`
	CreatedBy      uint           `json:"created_by" gorm:"not null"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Registry) TableName() string { return "registries" }

// ---------------------------------------------------------------------------
// GORM hooks — transparent AES-256-GCM encryption
// ---------------------------------------------------------------------------

func (r *Registry) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&r.PasswordEnc, &r.CABundleEnc)
}

func (r *Registry) afterSave() {
	_ = decryptFields(&r.PasswordEnc, &r.CABundleEnc)
}

func (r *Registry) AfterCreate(_ *gorm.DB) error {
	r.afterSave()
	return nil
}

func (r *Registry) AfterUpdate(_ *gorm.DB) error {
	r.afterSave()
	return nil
}

func (r *Registry) AfterFind(_ *gorm.DB) error {
	return decryptFields(&r.PasswordEnc, &r.CABundleEnc)
}
