package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Project — Git Provider 下的程式碼倉庫（CICD_ARCHITECTURE §M14.1）
//
// 設計：
//   - 屬於某個 GitProvider（git_provider_id FK）
//   - 每個 Project 對應一個 repo_url（唯一索引）
//   - Pipeline 可選擇性關聯 Project（project_id nullable FK）
//   - 有了 Project 層，Webhook 觸發才能精確比對 repo URL
// ---------------------------------------------------------------------------

// Project 代表 GitProvider 下的一個程式碼倉庫。
type Project struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	GitProviderID uint           `json:"git_provider_id" gorm:"not null;index"`
	Name          string         `json:"name" gorm:"not null;size:255"`
	RepoURL       string         `json:"repo_url" gorm:"not null;size:512;uniqueIndex"`
	DefaultBranch string         `json:"default_branch" gorm:"not null;size:255;default:'main'"`
	Description   string         `json:"description" gorm:"type:text"`
	CreatedBy     uint           `json:"created_by" gorm:"not null"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Project) TableName() string { return "projects" }
