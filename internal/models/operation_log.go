package models

import "time"

// OperationLog 操作审计日志
// 用于记录所有非GET请求的操作，包括登录登出、资源变更、配置修改等
type OperationLog struct {
	ID uint `json:"id" gorm:"primaryKey"`

	// 操作者信息
	UserID   *uint  `json:"user_id" gorm:"index"`           // 可为空（如登录失败场景）
	Username string `json:"username" gorm:"size:100;index"` // 冗余存储，便于查询

	// 请求信息
	Method string `json:"method" gorm:"size:10;index"` // POST/PUT/DELETE/PATCH
	Path   string `json:"path" gorm:"size:500"`        // 请求路径
	Query  string `json:"query" gorm:"size:1000"`      // 查询参数

	// 操作分类
	Module string `json:"module" gorm:"size:50;index"`  // auth/cluster/node/pod/workload/config/permission/...
	Action string `json:"action" gorm:"size:100;index"` // login/logout/create/update/delete/scale/...

	// 资源信息（可选，根据操作类型）
	ClusterID    *uint  `json:"cluster_id" gorm:"index"`
	ClusterName  string `json:"cluster_name" gorm:"size:100"`
	Namespace    string `json:"namespace" gorm:"size:100"`
	ResourceType string `json:"resource_type" gorm:"size:50"` // deployment/pod/node/...
	ResourceName string `json:"resource_name" gorm:"size:200"`

	// 请求/响应
	RequestBody string `json:"request_body" gorm:"type:text"` // 敏感信息脱敏后的请求体
	StatusCode  int    `json:"status_code"`                   // HTTP 状态码

	// 结果
	Success      bool   `json:"success" gorm:"index"`           // 是否成功
	ErrorMessage string `json:"error_message" gorm:"size:1000"` // 失败时的错误信息

	// 客户端信息
	ClientIP  string `json:"client_ip" gorm:"size:45"`
	UserAgent string `json:"user_agent" gorm:"size:500"`

	// 其他
	Duration  int64     `json:"duration"`               // 请求耗时(ms)
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}

