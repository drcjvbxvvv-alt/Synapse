package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Pipeline 狀態與觸發類型常數
// ---------------------------------------------------------------------------

// Pipeline Run 狀態
const (
	PipelineRunStatusQueued     = "queued"
	PipelineRunStatusRunning    = "running"
	PipelineRunStatusSuccess    = "success"
	PipelineRunStatusFailed     = "failed"
	PipelineRunStatusCancelled  = "cancelled"
	PipelineRunStatusCancelling = "cancelling"
	PipelineRunStatusRejected   = "rejected"
)

// Step Run 狀態
const (
	StepRunStatusPending          = "pending"
	StepRunStatusRunning          = "running"
	StepRunStatusSuccess          = "success"
	StepRunStatusFailed           = "failed"
	StepRunStatusCancelled        = "cancelled"
	StepRunStatusSkipped          = "skipped"
	StepRunStatusWaitingApproval  = "waiting_approval"
)

// Pipeline 觸發來源
const (
	TriggerTypeManual  = "manual"
	TriggerTypeWebhook = "webhook"
	TriggerTypeCron    = "cron"
	TriggerTypeRerun   = "rerun"
)

// Concurrency Group 策略
const (
	ConcurrencyPolicyCancelPrevious = "cancel_previous"
	ConcurrencyPolicyQueue          = "queue"
	ConcurrencyPolicyReject         = "reject"
)

// ---------------------------------------------------------------------------
// Pipeline — 定義，指向當前版本
// ---------------------------------------------------------------------------

// Pipeline 是 CI/CD Pipeline 的頂層定義，不綁定任何叢集。
// 叢集與命名空間由 Environment 提供（Pipeline → Environment → Cluster）。
// 實際 Steps 定義儲存於 PipelineVersion（不可變快照），Pipeline 僅持有
// current_version_id 指標。
type Pipeline struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	Name              string         `json:"name" gorm:"not null;size:255;uniqueIndex:idx_pipeline_name"`
	Description       string         `json:"description" gorm:"type:text"`
	CurrentVersionID  *uint          `json:"current_version_id"`
	ConcurrencyGroup  string         `json:"concurrency_group" gorm:"size:255"`
	ConcurrencyPolicy string         `json:"concurrency_policy" gorm:"size:30;default:'cancel_previous'"`
	MaxConcurrentRuns int            `json:"max_concurrent_runs" gorm:"default:1"`
	NotifyOnSuccess   string         `json:"notify_on_success" gorm:"type:jsonb;default:'[]'"` // channel_id JSON 陣列
	NotifyOnFailure   string         `json:"notify_on_failure" gorm:"type:jsonb;default:'[]'"`
	NotifyOnScan      string         `json:"notify_on_scan" gorm:"type:jsonb;default:'[]'"`
	CreatedBy         uint           `json:"created_by" gorm:"not null"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

// ---------------------------------------------------------------------------
// PipelineVersion — 不可變版本快照
// ---------------------------------------------------------------------------

// PipelineVersion 記錄 Pipeline 每次編輯產生的不可變快照。
// PipelineRun 必引用某一版本，確保歷史 Run 可重現。
type PipelineVersion struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	PipelineID   uint      `json:"pipeline_id" gorm:"not null;index;uniqueIndex:idx_pipeline_version"`
	Version      int       `json:"version" gorm:"not null;uniqueIndex:idx_pipeline_version"`
	StepsJSON    string    `json:"steps_json" gorm:"type:text;not null"`    // 不可變 Steps DAG JSON
	TriggersJSON string    `json:"triggers_json" gorm:"type:text"`          // 觸發條件 JSON
	EnvJSON      string    `json:"env_json" gorm:"type:text"`              // 預設環境變數 JSON
	RuntimeJSON  string    `json:"runtime_json" gorm:"type:text"`          // runtime 設定（SA、白名單、Pod Security）
	WorkspaceJSON string   `json:"workspace_json" gorm:"type:text"`        // workspace 設定
	HashSHA256   string    `json:"hash_sha256" gorm:"not null;size:64;index"` // 內容 hash，相同內容復用版本
	CreatedBy    uint      `json:"created_by" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// PipelineRun — 一次具體執行記錄
// ---------------------------------------------------------------------------

// PipelineRun 記錄一次 Pipeline 執行，含觸發來源、狀態、時間軸。
// EnvironmentID 為執行目標環境；ClusterID/Namespace 為反正規化快取（來源：Environment）。
type PipelineRun struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	PipelineID       uint           `json:"pipeline_id" gorm:"not null;index"`
	EnvironmentID    uint           `json:"environment_id" gorm:"not null;index"` // FK → environments.id
	SnapshotID       uint           `json:"snapshot_id" gorm:"not null;index"`    // FK → pipeline_versions.id
	ClusterID        uint           `json:"cluster_id" gorm:"not null;index"`     // denormalized from Environment
	Namespace        string         `json:"namespace" gorm:"not null;size:253"`   // denormalized from Environment
	Status           string         `json:"status" gorm:"not null;size:20;default:'queued';index"`
	TriggerType      string         `json:"trigger_type" gorm:"not null;size:20"` // manual / webhook / cron / rerun
	TriggerPayload   string         `json:"trigger_payload,omitempty" gorm:"type:text"` // webhook payload hash 等
	TriggeredByUser  uint           `json:"triggered_by_user" gorm:"not null"`
	ConcurrencyGroup string         `json:"concurrency_group" gorm:"size:255;index"`
	RerunFromID      *uint          `json:"rerun_from_id"`      // 若為 rerun，指向原始 RunID
	RerunFromStep    string         `json:"rerun_from_step,omitempty" gorm:"size:255"` // 從此 Step 開始重跑（空 = 全部重跑）
	Error            string         `json:"error,omitempty" gorm:"type:text"`
	QueuedAt         time.Time      `json:"queued_at"`
	StartedAt        *time.Time     `json:"started_at"`
	FinishedAt       *time.Time     `json:"finished_at"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`

	// 跨 Step 共享 Workspace 的 Node 綁定（vSphere CSI RWO 拓撲約束）
	BoundNodeName string `json:"bound_node_name,omitempty" gorm:"size:255"`
}

// ---------------------------------------------------------------------------
// StepRun — 每個 Step 的執行記錄，對應一個 K8s Job
// ---------------------------------------------------------------------------

// StepRun 記錄 Pipeline 中單一 Step 的執行狀態，對應一個 K8s Job。
type StepRun struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	PipelineRunID uint       `json:"pipeline_run_id" gorm:"not null;index"`
	StepName      string     `json:"step_name" gorm:"not null;size:255"`
	StepType      string     `json:"step_type" gorm:"not null;size:50"` // build-image / deploy / trivy-scan / ...
	StepIndex     int        `json:"step_index" gorm:"not null"`        // DAG 層級排序
	Status        string     `json:"status" gorm:"not null;size:20;default:'pending';index"`
	Image         string     `json:"image" gorm:"size:512"`
	Command       string     `json:"command,omitempty" gorm:"type:text"`    // 執行指令快照
	ConfigJSON    string     `json:"config_json,omitempty" gorm:"type:text"` // Step 特定設定
	JobName       string     `json:"job_name,omitempty" gorm:"size:255"`    // K8s Job 名稱
	JobNamespace  string     `json:"job_namespace,omitempty" gorm:"size:253"`
	ExitCode      *int       `json:"exit_code"`
	Error         string     `json:"error,omitempty" gorm:"type:text"`
	RetryCount    int        `json:"retry_count" gorm:"default:0"`
	MaxRetries    int        `json:"max_retries" gorm:"default:0"`
	DependsOn     string     `json:"depends_on" gorm:"type:jsonb"` // 前置 Step 名稱陣列
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// Trivy 掃描結果關聯（trivy-scan Step 專用）
	ScanResultID *uint `json:"scan_result_id"` // FK → image_scan_results.id

	// Argo Rollouts 狀態（deploy-rollout Step 專用）
	RolloutStatus string `json:"rollout_status,omitempty" gorm:"size:30"`
	RolloutWeight *int   `json:"rollout_weight"`

	// Approval Step 專用
	ApprovedBy *string    `json:"approved_by,omitempty" gorm:"size:255"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
}
