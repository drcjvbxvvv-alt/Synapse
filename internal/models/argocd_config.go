package models

import (
	"time"

	"gorm.io/gorm"
)

// ArgoCDConfig ArgoCD 集成配置
type ArgoCDConfig struct {
	ID        uint `json:"id" gorm:"primaryKey"`
	ClusterID uint `json:"cluster_id" gorm:"uniqueIndex"` // 关联的 KubePolaris 集群

	// ArgoCD 连接配置
	Enabled   bool   `json:"enabled" gorm:"default:false"`  // 是否启用
	ServerURL string `json:"server_url" gorm:"size:255"`    // ArgoCD 服务器地址，如 https://argocd.example.com
	AuthType  string `json:"auth_type" gorm:"size:20"`      // token, username
	Token     string `json:"-" gorm:"type:text"`            // ArgoCD API Token (加密存储)
	Username  string `json:"username" gorm:"size:100"`      // ArgoCD 用户名
	Password  string `json:"-" gorm:"type:text"`            // ArgoCD 密码 (加密存储)
	Insecure  bool   `json:"insecure" gorm:"default:false"` // 是否跳过 TLS 验证

	// Git 仓库配置
	GitRepoURL  string `json:"git_repo_url" gorm:"size:500"`            // Git 仓库地址
	GitBranch   string `json:"git_branch" gorm:"size:100;default:main"` // 默认分支
	GitPath     string `json:"git_path" gorm:"size:255"`                // 应用配置路径，如 /apps
	GitAuthType string `json:"git_auth_type" gorm:"size:20"`            // ssh, https, token
	GitUsername string `json:"git_username" gorm:"size:100"`
	GitPassword string `json:"-" gorm:"type:text"`
	GitSSHKey   string `json:"-" gorm:"type:text"` // SSH 私钥 (加密存储)

	// ArgoCD 中的集群名称
	ArgoCDClusterName string `json:"argocd_cluster_name" gorm:"size:100"`            // 在 ArgoCD 中注册的集群名称
	ArgoCDProject     string `json:"argocd_project" gorm:"size:100;default:default"` // ArgoCD 项目名称

	// 状态
	ConnectionStatus string     `json:"connection_status" gorm:"size:20"` // connected, disconnected, error
	LastTestAt       *time.Time `json:"last_test_at"`
	ErrorMessage     string     `json:"error_message" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Cluster Cluster `json:"cluster" gorm:"foreignKey:ClusterID"`
}

// TableName 指定表名
func (ArgoCDConfig) TableName() string {
	return "argocd_configs"
}

// ArgoCDConfigRequest 用于接收前端请求（包含敏感字段）
type ArgoCDConfigRequest struct {
	ID        uint `json:"id"`
	ClusterID uint `json:"cluster_id"`

	// ArgoCD 连接配置
	Enabled   bool   `json:"enabled"`
	ServerURL string `json:"server_url"`
	AuthType  string `json:"auth_type"` // token, username
	Token     string `json:"token"`     // 前端可以传 token
	Username  string `json:"username"`
	Password  string `json:"password"` // 前端可以传 password
	Insecure  bool   `json:"insecure"`

	// Git 仓库配置
	GitRepoURL  string `json:"git_repo_url"`
	GitBranch   string `json:"git_branch"`
	GitPath     string `json:"git_path"`
	GitAuthType string `json:"git_auth_type"` // ssh, https, token
	GitUsername string `json:"git_username"`
	GitPassword string `json:"git_password"` // 前端可以传
	GitSSHKey   string `json:"git_ssh_key"`  // 前端可以传

	// ArgoCD 中的集群名称
	ArgoCDClusterName string `json:"argocd_cluster_name"`
	ArgoCDProject     string `json:"argocd_project"`
}

// ToModel 转换为数据库模型
func (r *ArgoCDConfigRequest) ToModel() *ArgoCDConfig {
	return &ArgoCDConfig{
		ID:                r.ID,
		ClusterID:         r.ClusterID,
		Enabled:           r.Enabled,
		ServerURL:         r.ServerURL,
		AuthType:          r.AuthType,
		Token:             r.Token,
		Username:          r.Username,
		Password:          r.Password,
		Insecure:          r.Insecure,
		GitRepoURL:        r.GitRepoURL,
		GitBranch:         r.GitBranch,
		GitPath:           r.GitPath,
		GitAuthType:       r.GitAuthType,
		GitUsername:       r.GitUsername,
		GitPassword:       r.GitPassword,
		GitSSHKey:         r.GitSSHKey,
		ArgoCDClusterName: r.ArgoCDClusterName,
		ArgoCDProject:     r.ArgoCDProject,
	}
}

// ArgoCDApplication ArgoCD 应用（从 ArgoCD API 获取，不存数据库）
type ArgoCDApplication struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Project   string `json:"project"`

	// 源配置
	Source ArgoCDSource `json:"source"`

	// 目标配置
	Destination ArgoCDDestination `json:"destination"`

	// 状态
	SyncStatus   string `json:"sync_status"`   // Synced, OutOfSync, Unknown
	HealthStatus string `json:"health_status"` // Healthy, Degraded, Progressing, Suspended, Missing, Unknown

	// 同步信息
	SyncedRevision string `json:"synced_revision"`
	TargetRevision string `json:"target_revision"`

	// 时间
	CreatedAt    string `json:"created_at"`
	ReconciledAt string `json:"reconciled_at"`

	// 资源树
	Resources []ArgoCDResource `json:"resources,omitempty"`

	// 同步历史
	History []ArgoCDSyncHistory `json:"history,omitempty"`
}

// ArgoCDSource ArgoCD 应用源配置
type ArgoCDSource struct {
	RepoURL        string            `json:"repo_url"`
	Path           string            `json:"path"`
	TargetRevision string            `json:"target_revision"`
	Helm           *ArgoCDHelmSource `json:"helm,omitempty"`
	Kustomize      *ArgoCDKustomize  `json:"kustomize,omitempty"`
}

// ArgoCDHelmSource Helm 配置
type ArgoCDHelmSource struct {
	ValueFiles []string          `json:"value_files,omitempty"`
	Values     string            `json:"values,omitempty"`
	Parameters []ArgoCDHelmParam `json:"parameters,omitempty"`
}

// ArgoCDHelmParam Helm 参数
type ArgoCDHelmParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ArgoCDKustomize Kustomize 配置
type ArgoCDKustomize struct {
	Images []string `json:"images,omitempty"`
}

// ArgoCDDestination ArgoCD 应用目标配置
type ArgoCDDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
	Name      string `json:"name,omitempty"` // ArgoCD 集群名称
}

// ArgoCDResource ArgoCD 管理的资源
type ArgoCDResource struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Health    string `json:"health"`
	Message   string `json:"message,omitempty"`
}

// ArgoCDSyncHistory 同步历史
type ArgoCDSyncHistory struct {
	ID         int64        `json:"id"`
	Revision   string       `json:"revision"`
	DeployedAt string       `json:"deployed_at"`
	Source     ArgoCDSource `json:"source"`
}

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Name      string `json:"name" binding:"required"`
	Namespace string `json:"namespace"`
	Project   string `json:"project"`

	// 源配置（使用集群配置的 Git 仓库）
	Path           string `json:"path" binding:"required"` // Git 仓库中的路径
	TargetRevision string `json:"target_revision"`         // 分支/Tag/Commit

	// 目标配置（使用集群配置的 ArgoCD 集群名）
	DestNamespace string `json:"dest_namespace" binding:"required"`

	// Helm 配置（可选）
	HelmValues     string            `json:"helm_values,omitempty"`
	HelmParameters map[string]string `json:"helm_parameters,omitempty"`

	// 同步策略
	AutoSync bool `json:"auto_sync"`
	SelfHeal bool `json:"self_heal"`
	Prune    bool `json:"prune"`
}

// SyncApplicationRequest 同步应用请求
type SyncApplicationRequest struct {
	Revision  string   `json:"revision"`
	Prune     bool     `json:"prune"`
	DryRun    bool     `json:"dry_run"`
	Resources []string `json:"resources,omitempty"` // 指定同步的资源
}

// RollbackApplicationRequest 回滚应用请求
type RollbackApplicationRequest struct {
	RevisionID int64 `json:"revision_id" binding:"required"`
}
