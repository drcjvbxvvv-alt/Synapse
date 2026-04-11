package models

import "time"

// ComplianceReport stores a generated compliance assessment report.
type ComplianceReport struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	ClusterID   uint       `json:"cluster_id" gorm:"index;not null"`
	Framework   string     `json:"framework" gorm:"size:50;not null;index"` // SOC2, ISO27001, CIS_K8S
	Version     string     `json:"version" gorm:"size:20"`
	Status      string     `json:"status" gorm:"size:20;default:pending"` // pending, generating, completed, failed
	Score       float64    `json:"score"`                                 // 0–100
	PassCount   int        `json:"pass_count"`
	FailCount   int        `json:"fail_count"`
	WarnCount   int        `json:"warn_count"`
	ResultJSON  string     `json:"result_json,omitempty" gorm:"type:text"` // control-by-control results
	Error       string     `json:"error,omitempty" gorm:"size:512"`
	GeneratedBy uint       `json:"generated_by"`
	GeneratedAt *time.Time `json:"generated_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (ComplianceReport) TableName() string { return "compliance_reports" }

// ComplianceEvidence stores captured evidence for a compliance control.
type ComplianceEvidence struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ReportID     *uint     `json:"report_id" gorm:"index"`
	ClusterID    uint      `json:"cluster_id" gorm:"index;not null"`
	Framework    string    `json:"framework" gorm:"size:50;not null"`
	ControlID    string    `json:"control_id" gorm:"size:50;not null;index"`
	ControlTitle string    `json:"control_title" gorm:"size:255"`
	EvidenceType string    `json:"evidence_type" gorm:"size:30"` // api_snapshot, log_export, config_dump
	DataJSON     string    `json:"data_json,omitempty" gorm:"type:text"`
	CapturedAt   time.Time `json:"captured_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ComplianceEvidence) TableName() string { return "compliance_evidence" }

// ViolationEvent stores a unified compliance/security violation.
type ViolationEvent struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	ClusterID    uint       `json:"cluster_id" gorm:"index;not null"`
	Source       string     `json:"source" gorm:"size:30;not null;index"` // gatekeeper, trivy, bench, audit
	Severity     string     `json:"severity" gorm:"size:20;index"`       // critical, high, medium, low, info
	Title        string     `json:"title" gorm:"size:512;not null"`
	Description  string     `json:"description" gorm:"type:text"`
	ResourceType string     `json:"resource_type" gorm:"size:100"`
	ResourceRef  string     `json:"resource_ref" gorm:"size:512"` // namespace/kind/name
	Fingerprint  string     `json:"fingerprint" gorm:"size:64;uniqueIndex"` // dedup key
	RawData      string     `json:"-" gorm:"type:text"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	ResolvedBy   string     `json:"resolved_by" gorm:"size:100"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index"`
}

func (ViolationEvent) TableName() string { return "violation_events" }
