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

// ModuleNames maps module codes to English display names (translated on frontend via i18n)
var ModuleNames = map[string]string{
	ModuleAuth:       "Auth",
	ModuleCluster:    "Cluster",
	ModuleNode:       "Node",
	ModulePod:        "Pod",
	ModuleWorkload:   "Workload",
	ModuleConfig:     "Config",
	ModuleNetwork:    "Network",
	ModuleStorage:    "Storage",
	ModuleNamespace:  "Namespace",
	ModulePermission: "Permission",
	ModuleSystem:     "System",
	ModuleMonitoring: "Monitoring",
	ModuleAlert:      "Alert",
	ModuleArgoCD:     "ArgoCD",
	ModuleUnknown:    "Unknown",
}

// ActionNames maps action codes to English display names (translated on frontend via i18n)
var ActionNames = map[string]string{ // #nosec G101 -- action name mappings, not credentials
	ActionLogin:          "Login",
	ActionLogout:         "Logout",
	ActionLoginFailed:    "Login Failed",
	ActionChangePassword: "Change Password",
	ActionCreate:         "Create",
	ActionUpdate:         "Update",
	ActionDelete:         "Delete",
	ActionApply:          "Apply YAML",
	ActionScale:          "Scale",
	ActionRollback:       "Rollback",
	ActionRestart:        "Restart",
	ActionCordon:         "Cordon",
	ActionUncordon:       "Uncordon",
	ActionDrain:          "Drain",
	ActionSync:           "Sync",
	ActionTest:           "Test",
	ActionImport:         "Import",
}
