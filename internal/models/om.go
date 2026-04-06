package models

// HealthDiagnosisResponse 叢集健康診斷響應
type HealthDiagnosisResponse struct {
	HealthScore    int            `json:"health_score"`    // 健康評分 (0-100)
	Status         string         `json:"status"`          // 健康狀態: healthy, warning, critical
	RiskItems      []RiskItem     `json:"risk_items"`      // 風險項列表
	Suggestions    []string       `json:"suggestions"`     // 診斷建議
	DiagnosisTime  int64          `json:"diagnosis_time"`  // 診斷時間戳
	CategoryScores map[string]int `json:"category_scores"` // 各分類評分
}

// RiskItem 風險項
type RiskItem struct {
	ID          string `json:"id"`          // 唯一標識
	Category    string `json:"category"`    // 分類: node, workload, resource, network, storage
	Severity    string `json:"severity"`    // 嚴重程度: critical, warning, info
	Title       string `json:"title"`       // 標題
	Description string `json:"description"` // 描述
	Resource    string `json:"resource"`    // 相關資源名稱
	Namespace   string `json:"namespace"`   // 命名空間（如果適用）
	Solution    string `json:"solution"`    // 解決方案
}

// ResourceTopRequest 資源消耗 Top N 請求參數
type ResourceTopRequest struct {
	Type  string `form:"type" binding:"required,oneof=cpu memory disk network"` // 資源型別
	Level string `form:"level" binding:"required,oneof=namespace workload pod"` // 統計級別
	Limit int    `form:"limit,default=10"`                                      // 返回數量
}

// ResourceTopResponse 資源消耗 Top N 響應
type ResourceTopResponse struct {
	Type      string            `json:"type"`       // 資源型別
	Level     string            `json:"level"`      // 統計級別
	Items     []ResourceTopItem `json:"items"`      // Top N 列表
	QueryTime int64             `json:"query_time"` // 查詢時間戳
}

// ResourceTopItem 資源消耗項
type ResourceTopItem struct {
	Rank      int     `json:"rank"`                // 排名
	Name      string  `json:"name"`                // 名稱
	Namespace string  `json:"namespace,omitempty"` // 命名空間（workload/pod級別時有值）
	Usage     float64 `json:"usage"`               // 使用量
	UsageRate float64 `json:"usage_rate"`          // 使用率 (%)
	Request   float64 `json:"request,omitempty"`   // 請求值
	Limit     float64 `json:"limit,omitempty"`     // 限制值
	Unit      string  `json:"unit"`                // 單位
}

// ControlPlaneStatusResponse 控制面元件狀態響應
type ControlPlaneStatusResponse struct {
	Overall    string                  `json:"overall"`    // 整體狀態: healthy, degraded, unhealthy
	Components []ControlPlaneComponent `json:"components"` // 元件列表
	CheckTime  int64                   `json:"check_time"` // 檢查時間戳
}

// ControlPlaneComponent 控制面元件狀態
type ControlPlaneComponent struct {
	Name          string              `json:"name"`                // 元件名稱
	Type          string              `json:"type"`                // 元件型別: apiserver, scheduler, controller-manager, etcd
	Status        string              `json:"status"`              // 狀態: healthy, unhealthy, unknown
	Message       string              `json:"message"`             // 狀態訊息
	LastCheckTime int64               `json:"last_check_time"`     // 最後檢查時間
	Metrics       *ComponentMetrics   `json:"metrics,omitempty"`   // 元件指標
	Instances     []ComponentInstance `json:"instances,omitempty"` // 例項列表（高可用場景）
}

// ComponentMetrics 元件指標
type ComponentMetrics struct {
	RequestRate  float64 `json:"request_rate,omitempty"`  // 請求速率 (req/s)
	ErrorRate    float64 `json:"error_rate,omitempty"`    // 錯誤率 (%)
	Latency      float64 `json:"latency,omitempty"`       // 延遲 (ms)
	QueueLength  int     `json:"queue_length,omitempty"`  // 佇列長度
	LeaderStatus bool    `json:"leader_status,omitempty"` // Leader狀態（etcd/scheduler）
	DBSize       float64 `json:"db_size,omitempty"`       // 資料庫大小 (bytes，etcd用)
	MemberCount  int     `json:"member_count,omitempty"`  // 成員數量（etcd用）
}

// ComponentInstance 元件例項
type ComponentInstance struct {
	Name      string `json:"name"`       // 例項名稱/Pod名稱
	Node      string `json:"node"`       // 所在節點
	Status    string `json:"status"`     // 狀態
	IP        string `json:"ip"`         // IP地址
	StartTime int64  `json:"start_time"` // 啟動時間
}
