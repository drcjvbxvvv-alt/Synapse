package constants

// 操作模块定义
const (
	ModuleAuth       = "auth"       // 认证：登录、登出、密码修改
	ModuleCluster    = "cluster"    // 集群：导入、删除、配置
	ModuleNode       = "node"       // 节点：cordon、uncordon、drain
	ModulePod        = "pod"        // Pod：删除
	ModuleWorkload   = "workload"   // 工作负载：deployment/sts/ds/job/cronjob
	ModuleConfig     = "config"     // 配置：configmap、secret
	ModuleNetwork    = "network"    // 网络：service、ingress
	ModuleStorage    = "storage"    // 存储：pvc、pv、storageclass
	ModuleNamespace  = "namespace"  // 命名空间
	ModulePermission = "permission" // 权限：用户组、集群权限
	ModuleSystem     = "system"     // 系统：LDAP、SSH配置
	ModuleMonitoring = "monitoring" // 监控：Prometheus、Grafana配置
	ModuleAlert      = "alert"      // 告警：AlertManager、静默规则
	ModuleArgoCD     = "argocd"     // GitOps：ArgoCD应用
	ModuleUnknown    = "unknown"    // 未知模块
)

// 操作动作定义
const (
	// 认证相关
	ActionLogin          = "login"
	ActionLogout         = "logout"
	ActionLoginFailed    = "login_failed"
	ActionChangePassword = "change_password"

	// CRUD 操作
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionApply  = "apply" // YAML apply

	// 工作负载操作
	ActionScale    = "scale"
	ActionRollback = "rollback"
	ActionRestart  = "restart"

	// 节点操作
	ActionCordon   = "cordon"
	ActionUncordon = "uncordon"
	ActionDrain    = "drain"

	// ArgoCD 操作
	ActionSync = "sync"

	// 测试操作
	ActionTest = "test"

	// 导入操作
	ActionImport = "import"
)

// ModuleNames 模块中文名称映射
var ModuleNames = map[string]string{
	ModuleAuth:       "认证管理",
	ModuleCluster:    "集群管理",
	ModuleNode:       "节点管理",
	ModulePod:        "Pod管理",
	ModuleWorkload:   "工作负载",
	ModuleConfig:     "配置管理",
	ModuleNetwork:    "网络管理",
	ModuleStorage:    "存储管理",
	ModuleNamespace:  "命名空间",
	ModulePermission: "权限管理",
	ModuleSystem:     "系统设置",
	ModuleMonitoring: "监控配置",
	ModuleAlert:      "告警管理",
	ModuleArgoCD:     "GitOps",
	ModuleUnknown:    "未知",
}

// ActionNames 操作中文名称映射
var ActionNames = map[string]string{
	ActionLogin:          "登录",
	ActionLogout:         "登出",
	ActionLoginFailed:    "登录失败",
	ActionChangePassword: "修改密码",
	ActionCreate:         "创建",
	ActionUpdate:         "更新",
	ActionDelete:         "删除",
	ActionApply:          "应用YAML",
	ActionScale:          "扩缩容",
	ActionRollback:       "回滚",
	ActionRestart:        "重启",
	ActionCordon:         "禁止调度",
	ActionUncordon:       "允许调度",
	ActionDrain:          "驱逐节点",
	ActionSync:           "同步",
	ActionTest:           "测试",
	ActionImport:         "导入",
}
