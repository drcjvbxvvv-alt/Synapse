package models

import (
	"time"

	"gorm.io/gorm"
)

// TerminalSession 終端會話模型
type TerminalSession struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	UserID     uint           `json:"user_id" gorm:"not null"`
	ClusterID  uint           `json:"cluster_id" gorm:"not null"`
	TargetType string         `json:"target_type" gorm:"not null;size:20"` // pod, node, cluster
	TargetRef  string         `json:"target_ref" gorm:"type:jsonb"`         // JSON格式儲存目標引用資訊
	Namespace  string         `json:"namespace" gorm:"size:100"`
	Pod        string         `json:"pod" gorm:"size:100"`
	Container  string         `json:"container" gorm:"size:100"`
	Node       string         `json:"node" gorm:"size:100"`
	StartAt    time.Time      `json:"start_at"`
	EndAt      *time.Time     `json:"end_at"`
	InputSize  int64          `json:"input_size" gorm:"default:0"`          // 輸入流大小（位元組）
	Status     string         `json:"status" gorm:"default:active;size:20"` // active, closed, error
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`

	// 關聯關係
	User     User              `json:"user" gorm:"foreignKey:UserID"`
	Cluster  Cluster           `json:"cluster" gorm:"foreignKey:ClusterID"`
	Commands []TerminalCommand `json:"commands" gorm:"foreignKey:SessionID"`
}

// TerminalCommand 終端命令記錄模型
type TerminalCommand struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	SessionID uint      `json:"session_id" gorm:"not null;index"`
	Timestamp time.Time `json:"timestamp"`
	RawInput  string    `json:"raw_input" gorm:"type:text"`  // 原始輸入
	ParsedCmd string    `json:"parsed_cmd" gorm:"size:1024"` // 解析後的命令
	ExitCode  *int      `json:"exit_code"`                   // 命令退出碼
	CreatedAt time.Time `json:"created_at"`

	// 關聯關係
	Session TerminalSession `json:"session" gorm:"foreignKey:SessionID"`
}

// AuditLog 審計日誌模型
//
// Hash chain integrity (P2-2):
//   - PrevHash = Hash of the previous record (zeroHash for the first record).
//   - Hash     = SHA-256 of (PrevHash, UserID, Action, ResourceType,
//                ResourceRef, Result, IP, CreatedAt.UnixNano).
//
// Records written before the hash chain feature was enabled have empty Hash /
// PrevHash fields; VerifyChain skips them automatically.
type AuditLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       uint      `json:"user_id" gorm:"not null;index"`
	Action       string    `json:"action" gorm:"not null;size:100"`       // 操作型別
	ResourceType string    `json:"resource_type" gorm:"not null;size:50"` // 資源型別
	ResourceRef  string    `json:"resource_ref" gorm:"type:jsonb"`         // 資源引用資訊
	Result       string    `json:"result" gorm:"not null;size:20"`        // success, failed
	IP           string    `json:"ip" gorm:"size:45"`                     // 客戶端IP
	UserAgent    string    `json:"user_agent" gorm:"size:500"`            // 使用者代理
	Details      string    `json:"details" gorm:"type:text"`              // 詳細資訊
	PrevHash     string    `json:"prev_hash" gorm:"size:64;not null;default:''"` // P2-2 hash chain
	Hash         string    `json:"hash"      gorm:"size:64;not null;default:'';index"` // P2-2 hash chain
	CreatedAt    time.Time `json:"created_at"`

	// 關聯關係
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// TableName 指定終端會話表名
func (TerminalSession) TableName() string {
	return "terminal_sessions"
}

// TableName 指定終端命令表名
func (TerminalCommand) TableName() string {
	return "terminal_commands"
}

// TableName 指定審計日誌表名
func (AuditLog) TableName() string {
	return "audit_logs"
}
