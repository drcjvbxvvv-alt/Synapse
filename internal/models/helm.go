package models

import "gorm.io/gorm"

// HelmRepository Helm Chart 倉庫配置（全域共用）
type HelmRepository struct {
	gorm.Model
	Name     string `json:"name" gorm:"uniqueIndex;size:128"`
	URL      string `json:"url" gorm:"size:512"`
	Username string `json:"username,omitempty" gorm:"size:256"`
	Password string `json:"password,omitempty" gorm:"size:256"`
}
