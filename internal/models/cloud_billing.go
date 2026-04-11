package models

import (
	"time"

	"github.com/shaia/Synapse/pkg/crypto"
	"gorm.io/gorm"
)

// CloudBillingConfig 雲端帳單整合設定（每叢集）
type CloudBillingConfig struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ClusterID uint   `json:"cluster_id" gorm:"uniqueIndex;not null"`
	Provider  string `json:"provider" gorm:"size:20;not null;default:disabled"` // disabled | aws | gcp

	// AWS Cost Explorer
	AWSAccessKeyID     string `json:"aws_access_key_id" gorm:"size:128"`
	AWSSecretAccessKey string `json:"-" gorm:"size:256"` // 敏感欄位，不序列化至 JSON
	AWSRegion          string `json:"aws_region" gorm:"size:32;default:us-east-1"`
	AWSLinkedAccountID string `json:"aws_linked_account_id" gorm:"size:20"` // 可選，多帳號環境

	// GCP Cloud Billing
	GCPProjectID          string `json:"gcp_project_id" gorm:"size:128"`
	GCPBillingAccountID   string `json:"gcp_billing_account_id" gorm:"size:64"` // "billingAccounts/XXXX-XXXX-XXXX"
	GCPServiceAccountJSON string `json:"-" gorm:"type:text"`                     // service account JSON key，敏感

	LastSyncedAt *time.Time `json:"last_synced_at"`
	LastError    string     `json:"last_error,omitempty" gorm:"size:512"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (CloudBillingConfig) TableName() string { return "cloud_billing_configs" }

// ---------------------------------------------------------------------------
// GORM hooks — AES-256-GCM encryption for cloud billing credentials (P2-3).
// ---------------------------------------------------------------------------

func (c *CloudBillingConfig) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&c.AWSSecretAccessKey, &c.GCPServiceAccountJSON)
}

func (c *CloudBillingConfig) AfterCreate(_ *gorm.DB) error {
	return decryptFields(&c.AWSSecretAccessKey, &c.GCPServiceAccountJSON)
}
func (c *CloudBillingConfig) AfterUpdate(_ *gorm.DB) error {
	return decryptFields(&c.AWSSecretAccessKey, &c.GCPServiceAccountJSON)
}
func (c *CloudBillingConfig) AfterFind(_ *gorm.DB) error {
	if !crypto.IsEnabled() {
		return nil
	}
	return decryptFields(&c.AWSSecretAccessKey, &c.GCPServiceAccountJSON)
}

// CloudBillingRecord 已同步的帳單記錄（按服務分類，每月一筆）
type CloudBillingRecord struct {
	ID        uint    `json:"id" gorm:"primaryKey"`
	ClusterID uint    `json:"cluster_id" gorm:"index;not null"`
	Month     string  `json:"month" gorm:"size:7;index;not null"` // "2026-04"
	Provider  string  `json:"provider" gorm:"size:20"`
	Service   string  `json:"service" gorm:"size:256"` // e.g. "Amazon Elastic Kubernetes Service"
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency" gorm:"size:10;default:USD"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CloudBillingRecord) TableName() string { return "cloud_billing_records" }
