package models

import "time"

// OperationLog 操作審計日誌
// 用於記錄所有非GET請求的操作，包括登入登出、資源變更、配置修改等
type OperationLog struct {
	ID uint `json:"id" gorm:"primaryKey"`

	// 操作者資訊
	UserID   *uint  `json:"user_id" gorm:"index:idx_op_user_time"` // 複合索引（user_id, created_at）
	Username string `json:"username" gorm:"size:100;index"`        // 冗餘儲存，便於查詢

	// 請求資訊
	Method string `json:"method" gorm:"size:10;index"` // POST/PUT/DELETE/PATCH
	Path   string `json:"path" gorm:"size:500"`        // 請求路徑
	Query  string `json:"query" gorm:"size:1000"`      // 查詢參數

	// 操作分類
	Module string `json:"module" gorm:"size:50;index"`  // auth/cluster/node/pod/workload/config/permission/...
	Action string `json:"action" gorm:"size:100;index"` // login/logout/create/update/delete/scale/...

	// 資源資訊（可選，根據操作型別）
	ClusterID    *uint  `json:"cluster_id" gorm:"index"`
	ClusterName  string `json:"cluster_name" gorm:"size:100"`
	Namespace    string `json:"namespace" gorm:"size:100"`
	ResourceType string `json:"resource_type" gorm:"size:50"` // deployment/pod/node/...
	ResourceName string `json:"resource_name" gorm:"size:200"`

	// 請求/響應
	RequestBody string `json:"request_body" gorm:"type:text"` // 敏感資訊脫敏後的請求體
	StatusCode  int    `json:"status_code"`                   // HTTP 狀態碼

	// 結果
	Success      bool   `json:"success" gorm:"index"`           // 是否成功
	ErrorMessage string `json:"error_message" gorm:"size:1000"` // 失敗時的錯誤資訊

	// 客戶端資訊
	ClientIP  string `json:"client_ip" gorm:"size:45"`
	UserAgent string `json:"user_agent" gorm:"size:500"`

	// 其他
	Duration  int64     `json:"duration"` // 請求耗時(ms)
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_op_user_time"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}
