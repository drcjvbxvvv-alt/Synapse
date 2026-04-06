package constants

// 操作模組定義
const (
	ModuleAuth       = "auth"       // 認證：登入、登出、密碼修改
	ModuleCluster    = "cluster"    // 叢集：匯入、刪除、配置
	ModuleNode       = "node"       // 節點：cordon、uncordon、drain
	ModulePod        = "pod"        // Pod：刪除
	ModuleWorkload   = "workload"   // 工作負載：deployment/sts/ds/job/cronjob
	ModuleConfig     = "config"     // 配置：configmap、secret
	ModuleNetwork    = "network"    // 網路：service、ingress
	ModuleStorage    = "storage"    // 儲存：pvc、pv、storageclass
	ModuleNamespace  = "namespace"  // 命名空間
	ModulePermission = "permission" // 權限：使用者組、叢集權限
	ModuleSystem     = "system"     // 系統：LDAP、SSH配置
	ModuleMonitoring = "monitoring" // 監控：Prometheus、Grafana配置
	ModuleAlert      = "alert"      // 告警：AlertManager、靜默規則
	ModuleArgoCD     = "argocd"     // GitOps：ArgoCD應用
	ModuleUnknown    = "unknown"    // 未知模組
)

// 操作動作定義
const (
	// 認證相關
	ActionLogin          = "login"
	ActionLogout         = "logout"
	ActionLoginFailed    = "login_failed"
	ActionChangePassword = "change_password"

	// CRUD 操作
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionApply  = "apply" // YAML apply

	// 工作負載操作
	ActionScale    = "scale"
	ActionRollback = "rollback"
	ActionRestart  = "restart"

	// 節點操作
	ActionCordon   = "cordon"
	ActionUncordon = "uncordon"
	ActionDrain    = "drain"

	// ArgoCD 操作
	ActionSync = "sync"

	// 測試操作
	ActionTest = "test"

	// 匯入操作
	ActionImport = "import"
)

// ModuleNames 模組中文名稱對映
var ModuleNames = map[string]string{
	ModuleAuth:       "認證管理",
	ModuleCluster:    "叢集管理",
	ModuleNode:       "節點管理",
	ModulePod:        "Pod管理",
	ModuleWorkload:   "工作負載",
	ModuleConfig:     "配置管理",
	ModuleNetwork:    "網路管理",
	ModuleStorage:    "儲存管理",
	ModuleNamespace:  "命名空間",
	ModulePermission: "權限管理",
	ModuleSystem:     "系統設定",
	ModuleMonitoring: "監控配置",
	ModuleAlert:      "告警管理",
	ModuleArgoCD:     "GitOps",
	ModuleUnknown:    "未知",
}

// ActionNames 操作中文名稱對映
var ActionNames = map[string]string{ // #nosec G101 -- 操作名稱對映，非憑據
	ActionLogin:          "登入",
	ActionLogout:         "登出",
	ActionLoginFailed:    "登入失敗",
	ActionChangePassword: "修改密碼",
	ActionCreate:         "建立",
	ActionUpdate:         "更新",
	ActionDelete:         "刪除",
	ActionApply:          "應用YAML",
	ActionScale:          "擴縮容",
	ActionRollback:       "回滾",
	ActionRestart:        "重啟",
	ActionCordon:         "禁止排程",
	ActionUncordon:       "允許排程",
	ActionDrain:          "驅逐節點",
	ActionSync:           "同步",
	ActionTest:           "測試",
	ActionImport:         "匯入",
}
