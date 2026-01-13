package models

// HealthDiagnosisResponse 集群健康诊断响应
type HealthDiagnosisResponse struct {
	HealthScore    int            `json:"health_score"`    // 健康评分 (0-100)
	Status         string         `json:"status"`          // 健康状态: healthy, warning, critical
	RiskItems      []RiskItem     `json:"risk_items"`      // 风险项列表
	Suggestions    []string       `json:"suggestions"`     // 诊断建议
	DiagnosisTime  int64          `json:"diagnosis_time"`  // 诊断时间戳
	CategoryScores map[string]int `json:"category_scores"` // 各分类评分
}

// RiskItem 风险项
type RiskItem struct {
	ID          string `json:"id"`          // 唯一标识
	Category    string `json:"category"`    // 分类: node, workload, resource, network, storage
	Severity    string `json:"severity"`    // 严重程度: critical, warning, info
	Title       string `json:"title"`       // 标题
	Description string `json:"description"` // 描述
	Resource    string `json:"resource"`    // 相关资源名称
	Namespace   string `json:"namespace"`   // 命名空间（如果适用）
	Solution    string `json:"solution"`    // 解决方案
}

// ResourceTopRequest 资源消耗 Top N 请求参数
type ResourceTopRequest struct {
	Type  string `form:"type" binding:"required,oneof=cpu memory disk network"` // 资源类型
	Level string `form:"level" binding:"required,oneof=namespace workload pod"` // 统计级别
	Limit int    `form:"limit,default=10"`                                      // 返回数量
}

// ResourceTopResponse 资源消耗 Top N 响应
type ResourceTopResponse struct {
	Type      string            `json:"type"`       // 资源类型
	Level     string            `json:"level"`      // 统计级别
	Items     []ResourceTopItem `json:"items"`      // Top N 列表
	QueryTime int64             `json:"query_time"` // 查询时间戳
}

// ResourceTopItem 资源消耗项
type ResourceTopItem struct {
	Rank      int     `json:"rank"`                // 排名
	Name      string  `json:"name"`                // 名称
	Namespace string  `json:"namespace,omitempty"` // 命名空间（workload/pod级别时有值）
	Usage     float64 `json:"usage"`               // 使用量
	UsageRate float64 `json:"usage_rate"`          // 使用率 (%)
	Request   float64 `json:"request,omitempty"`   // 请求值
	Limit     float64 `json:"limit,omitempty"`     // 限制值
	Unit      string  `json:"unit"`                // 单位
}

// ControlPlaneStatusResponse 控制面组件状态响应
type ControlPlaneStatusResponse struct {
	Overall    string                  `json:"overall"`    // 整体状态: healthy, degraded, unhealthy
	Components []ControlPlaneComponent `json:"components"` // 组件列表
	CheckTime  int64                   `json:"check_time"` // 检查时间戳
}

// ControlPlaneComponent 控制面组件状态
type ControlPlaneComponent struct {
	Name          string              `json:"name"`                // 组件名称
	Type          string              `json:"type"`                // 组件类型: apiserver, scheduler, controller-manager, etcd
	Status        string              `json:"status"`              // 状态: healthy, unhealthy, unknown
	Message       string              `json:"message"`             // 状态消息
	LastCheckTime int64               `json:"last_check_time"`     // 最后检查时间
	Metrics       *ComponentMetrics   `json:"metrics,omitempty"`   // 组件指标
	Instances     []ComponentInstance `json:"instances,omitempty"` // 实例列表（高可用场景）
}

// ComponentMetrics 组件指标
type ComponentMetrics struct {
	RequestRate  float64 `json:"request_rate,omitempty"`  // 请求速率 (req/s)
	ErrorRate    float64 `json:"error_rate,omitempty"`    // 错误率 (%)
	Latency      float64 `json:"latency,omitempty"`       // 延迟 (ms)
	QueueLength  int     `json:"queue_length,omitempty"`  // 队列长度
	LeaderStatus bool    `json:"leader_status,omitempty"` // Leader状态（etcd/scheduler）
	DBSize       float64 `json:"db_size,omitempty"`       // 数据库大小 (bytes，etcd用)
	MemberCount  int     `json:"member_count,omitempty"`  // 成员数量（etcd用）
}

// ComponentInstance 组件实例
type ComponentInstance struct {
	Name      string `json:"name"`       // 实例名称/Pod名称
	Node      string `json:"node"`       // 所在节点
	Status    string `json:"status"`     // 状态
	IP        string `json:"ip"`         // IP地址
	StartTime int64  `json:"start_time"` // 启动时间
}
