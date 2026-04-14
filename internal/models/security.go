package models

import "time"

// ImageScanResult stores results of a Trivy image vulnerability scan.
type ImageScanResult struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ClusterID   uint      `json:"cluster_id" gorm:"index;not null"`
	Namespace   string    `json:"namespace" gorm:"size:100"`
	PodName     string    `json:"pod_name" gorm:"size:255"`
	ContainerName string  `json:"container_name" gorm:"size:255"`
	Image       string    `json:"image" gorm:"size:512;not null"`
	Status      string    `json:"status" gorm:"size:20;default:pending"` // pending, scanning, completed, failed
	Critical    int       `json:"critical"`
	High        int       `json:"high"`
	Medium      int       `json:"medium"`
	Low         int       `json:"low"`
	Unknown     int       `json:"unknown"`
	ResultJSON  string    `json:"result_json,omitempty" gorm:"type:text"` // full trivy JSON output
	Error       string    `json:"error,omitempty" gorm:"size:512"`
	ScannedAt   *time.Time `json:"scanned_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Pipeline CI/CD 關聯（由 trivy-scan Step 回寫）
	ScanSource    string `json:"scan_source" gorm:"size:20;default:'manual'"` // manual / pipeline
	PipelineRunID *uint  `json:"pipeline_run_id" gorm:"index"`
	StepRunID     *uint  `json:"step_run_id"`
}

// BenchResult stores results of a CIS kube-bench security benchmark run.
type BenchResult struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ClusterID  uint      `json:"cluster_id" gorm:"index;not null"`
	Status     string    `json:"status" gorm:"size:20;default:pending"` // pending, running, completed, failed
	Pass       int       `json:"pass"`
	Fail       int       `json:"fail"`
	Warn       int       `json:"warn"`
	Info       int       `json:"info"`
	Score      float64   `json:"score"` // pass / (pass + fail) * 100
	ResultJSON string    `json:"result_json,omitempty" gorm:"type:text"` // full kube-bench JSON output
	Error      string    `json:"error,omitempty" gorm:"size:512"`
	JobName    string    `json:"job_name" gorm:"size:255"` // k8s Job name created in kube-system
	RunAt      *time.Time `json:"run_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
