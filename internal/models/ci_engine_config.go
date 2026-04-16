package models

import (
	"time"

	"github.com/shaia/Synapse/pkg/crypto"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// CIEngineConfig — external CI engine connection profile (M18a)
// ---------------------------------------------------------------------------
//
// Each row stores the connection details for a single external CI engine
// instance (GitLab project, Jenkins controller, Tekton-enabled cluster, …).
// The built-in "native" engine does NOT require a row in this table.
//
// Pipelines reference a config through Pipeline.EngineConfigID. When
// EngineType == "native" the reference is nil.
//
// Sensitive fields (Token, Password, PrivateKey, Webhook Secret) are
// encrypted at rest via pkg/crypto AES-256-GCM in the GORM hooks. They are
// transparently decrypted on read and MUST be redacted (`json:"-"`) in API
// responses — expose them only through dedicated admin endpoints.

// CIEngine* auth type constants. Not all engines support every type; the
// service layer validates the combination (EngineType, AuthType).
const (
	CIEngineAuthToken       = "token"        // Personal Access Token / API Token
	CIEngineAuthBasic       = "basic"        // username + password
	CIEngineAuthKubeconfig  = "kubeconfig"   // Tekton / Argo (reuse cluster access)
	CIEngineAuthServiceAcct = "service_acct" // Native / Tekton via SA token
)

// CIEngineConfig holds connection + credentials for an external CI engine.
type CIEngineConfig struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	Name       string `json:"name" gorm:"size:100;not null;uniqueIndex:idx_ci_engine_name"`
	EngineType string `json:"engine_type" gorm:"size:20;not null;index"` // native / gitlab / jenkins / tekton / argo / github
	Enabled    bool   `json:"enabled" gorm:"not null;default:true"`

	// Connection
	Endpoint string `json:"endpoint" gorm:"size:500"` // https://gitlab.example.com

	// Authentication
	AuthType string `json:"auth_type" gorm:"size:20"`
	Username string `json:"username" gorm:"size:100"`
	Token    string `json:"-" gorm:"type:text"` // encrypted (Token / PAT)
	Password string `json:"-" gorm:"type:text"` // encrypted (basic auth)

	// Webhook (for reverse integration: engine → Synapse status push)
	WebhookSecret string `json:"-" gorm:"type:text"` // encrypted HMAC shared secret

	// Reference to a Synapse-managed cluster for cluster-scoped engines
	// (Tekton / Argo Workflows). Null for off-cluster engines.
	ClusterID *uint `json:"cluster_id,omitempty" gorm:"index"`

	// Engine-specific free-form settings (JSON blob).
	// Example (GitLab): {"project_id": 42, "default_ref": "main"}
	// Example (Jenkins): {"folder": "saas", "crumb_enabled": true}
	ExtraJSON string `json:"extra_json,omitempty" gorm:"type:text"`

	// TLS
	InsecureSkipVerify bool   `json:"insecure_skip_verify" gorm:"default:false"`
	CABundle           string `json:"-" gorm:"type:text"` // optional PEM CA, encrypted

	// Health tracking (updated by the factory / probe worker; not user input)
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	LastHealthy   bool       `json:"last_healthy"`
	LastVersion   string     `json:"last_version,omitempty" gorm:"size:50"`
	LastError     string     `json:"last_error,omitempty" gorm:"type:text"`

	CreatedBy uint           `json:"created_by" gorm:"not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName returns the explicit DB table name.
func (CIEngineConfig) TableName() string { return "ci_engine_configs" }

// ---------------------------------------------------------------------------
// GORM hooks — AES-256-GCM encryption for credential fields
// ---------------------------------------------------------------------------

// BeforeSave encrypts sensitive credential fields in place.
func (c *CIEngineConfig) BeforeSave(_ *gorm.DB) error {
	return encryptFields(&c.Token, &c.Password, &c.WebhookSecret, &c.CABundle)
}

func (c *CIEngineConfig) afterDecrypt() error {
	return decryptFields(&c.Token, &c.Password, &c.WebhookSecret, &c.CABundle)
}

// AfterCreate / AfterUpdate / AfterFind restore the in-memory value to
// plaintext so service code sees decrypted values transparently.
func (c *CIEngineConfig) AfterCreate(_ *gorm.DB) error { return c.afterDecrypt() }
func (c *CIEngineConfig) AfterUpdate(_ *gorm.DB) error { return c.afterDecrypt() }

// AfterFind follows the same pattern as other encrypted models: bail out
// early when crypto is disabled so tests without a KeyProvider still work.
func (c *CIEngineConfig) AfterFind(_ *gorm.DB) error {
	if !crypto.IsEnabled() {
		return nil
	}
	return c.afterDecrypt()
}

// ---------------------------------------------------------------------------
// Request DTO — decouples API input from storage model
// ---------------------------------------------------------------------------

// CIEngineConfigRequest is the shape accepted from HTTP clients. Unlike the
// storage model, the credential fields are exposed as normal JSON keys so
// clients can set them; responses use the storage model (redacted).
type CIEngineConfigRequest struct {
	ID         uint   `json:"id,omitempty"`
	Name       string `json:"name" binding:"required,min=1,max=100"`
	EngineType string `json:"engine_type" binding:"required"`
	Enabled    *bool  `json:"enabled,omitempty"`

	Endpoint string `json:"endpoint"`

	AuthType      string `json:"auth_type"`
	Username      string `json:"username"`
	Token         string `json:"token,omitempty"`
	Password      string `json:"password,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`

	ClusterID *uint  `json:"cluster_id,omitempty"`
	ExtraJSON string `json:"extra_json,omitempty"`

	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CABundle           string `json:"ca_bundle,omitempty"`
}

// ApplyTo copies request fields into the supplied model. Empty credential
// strings are preserved (callers decide whether to keep existing values) —
// the service layer handles the "don't overwrite secrets with blank" case.
func (r *CIEngineConfigRequest) ApplyTo(m *CIEngineConfig) {
	m.Name = r.Name
	m.EngineType = r.EngineType
	if r.Enabled != nil {
		m.Enabled = *r.Enabled
	}
	m.Endpoint = r.Endpoint
	m.AuthType = r.AuthType
	m.Username = r.Username
	if r.Token != "" {
		m.Token = r.Token
	}
	if r.Password != "" {
		m.Password = r.Password
	}
	if r.WebhookSecret != "" {
		m.WebhookSecret = r.WebhookSecret
	}
	m.ClusterID = r.ClusterID
	m.ExtraJSON = r.ExtraJSON
	m.InsecureSkipVerify = r.InsecureSkipVerify
	if r.CABundle != "" {
		m.CABundle = r.CABundle
	}
}
