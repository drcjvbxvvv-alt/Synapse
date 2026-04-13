package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// PermissionType 權限型別常量
const (
	PermissionTypeAdmin    = "admin"    // 管理員權限：全部命名空間所有資源的讀寫權限
	PermissionTypeOps      = "ops"      // 運維權限：大多數資源讀寫，節點/儲存/配額只讀
	PermissionTypeDev      = "dev"      // 開發權限：指定命名空間內資源讀寫
	PermissionTypeReadonly = "readonly" // 只讀權限：資源只讀
	PermissionTypeCustom   = "custom"   // 自定義權限：使用者選擇 ClusterRole/Role
)

// UserGroup 使用者組模型
type UserGroup struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"uniqueIndex;not null;size:50"`
	Description string         `json:"description" gorm:"size:255"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 關聯關係
	Users []User `json:"users,omitempty" gorm:"many2many:user_group_members;"`
}

// UserGroupMember 使用者組成員關聯表
type UserGroupMember struct {
	UserID      uint `json:"user_id" gorm:"primaryKey"`
	UserGroupID uint `json:"user_group_id" gorm:"primaryKey"`
}

// TableName 指定使用者組成員關聯表名
func (UserGroupMember) TableName() string {
	return "user_group_members"
}

// ClusterPermission 叢集級別權限配置
type ClusterPermission struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	ClusterID      uint           `json:"cluster_id" gorm:"index;not null"`        // 關聯叢集
	UserID         *uint          `json:"user_id" gorm:"index"`                    // 使用者ID（與使用者組二選一）
	UserGroupID    *uint          `json:"user_group_id" gorm:"index"`              // 使用者組ID
	PermissionType string         `json:"permission_type" gorm:"not null;size:50"` // admin, ops, dev, readonly, custom
	Namespaces     string         `json:"namespaces" gorm:"type:text"`             // 命名空間範圍，JSON格式，["*"] 表示全部
	CustomRoleRef  string         `json:"custom_role_ref" gorm:"size:200"`         // 自定義權限時引用的 ClusterRole/Role 名稱
	FeaturePolicy  string         `json:"feature_policy" gorm:"type:text"`         // 功能開關策略，JSON格式，NULL = 使用預設值
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	// 關聯（預載入用）
	Cluster   *Cluster   `json:"cluster,omitempty" gorm:"foreignKey:ClusterID"`
	User      *User      `json:"user,omitempty" gorm:"foreignKey:UserID"`
	UserGroup *UserGroup `json:"user_group,omitempty" gorm:"foreignKey:UserGroupID"`
}

// GetNamespaceList 獲取命名空間列表
func (cp *ClusterPermission) GetNamespaceList() []string {
	if cp.Namespaces == "" {
		return []string{"*"}
	}
	var namespaces []string
	if err := json.Unmarshal([]byte(cp.Namespaces), &namespaces); err != nil {
		return []string{}
	}
	return namespaces
}

// SetNamespaceList 設定命名空間列表
func (cp *ClusterPermission) SetNamespaceList(namespaces []string) error {
	data, err := json.Marshal(namespaces)
	if err != nil {
		return err
	}
	cp.Namespaces = string(data)
	return nil
}

// PermissionTypeInfo 權限型別資訊
type PermissionTypeInfo struct {
	Type                   string   `json:"type"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Resources              []string `json:"resources"`
	Actions                []string `json:"actions"`
	AllowPartialNamespaces bool     `json:"allowPartialNamespaces"` // 是否允許選擇部分命名空間
	RequireAllNamespaces   bool     `json:"requireAllNamespaces"`   // 是否必須選擇全部命名空間
	ClusterRoleName        string   `json:"clusterRoleName"`        // 對應的 ClusterRole 名稱
	ServiceAccountName     string   `json:"serviceAccountName"`     // 對應的 ServiceAccount 名稱
}

// ClusterRole 和 ServiceAccount 常量
const (
	// ClusterRole 名稱
	ClusterRoleClusterAdmin = "synapse-cluster-admin"
	ClusterRoleOps          = "synapse-ops"
	ClusterRoleDev          = "synapse-dev"
	ClusterRoleReadonly     = "synapse-readonly"

	// ServiceAccount 名稱
	SAClusterAdmin = "synapse-admin-sa"
	SAOps          = "synapse-ops-sa"
	SADev          = "synapse-dev-sa"
	SAReadonly     = "synapse-readonly-sa"

	// Namespace
	SynapseNamespace = "synapse-system"
)

// GetPermissionTypes 獲取所有權限型別資訊
func GetPermissionTypes() []PermissionTypeInfo {
	return []PermissionTypeInfo{
		{
			Type:                   PermissionTypeAdmin,
			Name:                   "管理員權限",
			Description:            "對全部命名空間下所有資源的讀寫權限（必須選擇全部命名空間）",
			Resources:              []string{"*"},
			Actions:                []string{"*"},
			AllowPartialNamespaces: false,
			RequireAllNamespaces:   true,
			ClusterRoleName:        ClusterRoleClusterAdmin,
			ServiceAccountName:     SAClusterAdmin,
		},
		{
			Type:                   PermissionTypeOps,
			Name:                   "運維權限",
			Description:            "對全部命名空間下大多數資源的讀寫權限，對節點、儲存卷、命名空間和配額管理的只讀權限（必須選擇全部命名空間）",
			Resources:              []string{"pods", "deployments", "statefulsets", "daemonsets", "services", "ingresses", "configmaps", "secrets"},
			Actions:                []string{"get", "list", "watch", "create", "update", "delete"},
			AllowPartialNamespaces: false,
			RequireAllNamespaces:   true,
			ClusterRoleName:        ClusterRoleOps,
			ServiceAccountName:     SAOps,
		},
		{
			Type:                   PermissionTypeDev,
			Name:                   "開發權限",
			Description:            "對全部或所選命名空間下大多數資源的讀寫權限",
			Resources:              []string{"pods", "deployments", "statefulsets", "daemonsets", "services", "ingresses", "configmaps", "secrets"},
			Actions:                []string{"get", "list", "watch", "create", "update", "delete"},
			AllowPartialNamespaces: true,
			RequireAllNamespaces:   false,
			ClusterRoleName:        ClusterRoleDev,
			ServiceAccountName:     SADev,
		},
		{
			Type:                   PermissionTypeReadonly,
			Name:                   "只讀權限",
			Description:            "對全部或所選命名空間下大多數資源的只讀權限",
			Resources:              []string{"*"},
			Actions:                []string{"get", "list", "watch"},
			AllowPartialNamespaces: true,
			RequireAllNamespaces:   false,
			ClusterRoleName:        ClusterRoleReadonly,
			ServiceAccountName:     SAReadonly,
		},
		{
			Type:                   PermissionTypeCustom,
			Name:                   "自定義權限",
			Description:            "權限由您所選擇的ClusterRole或Role決定",
			Resources:              []string{},
			Actions:                []string{},
			AllowPartialNamespaces: true,
			RequireAllNamespaces:   false,
			ClusterRoleName:        "", // 自定義權限由使用者指定
			ServiceAccountName:     "",
		},
	}
}

// GetClusterRoleByPermissionType 根據權限型別獲取 ClusterRole 名稱
func GetClusterRoleByPermissionType(permissionType string) string {
	switch permissionType {
	case PermissionTypeAdmin:
		return ClusterRoleClusterAdmin
	case PermissionTypeOps:
		return ClusterRoleOps
	case PermissionTypeDev:
		return ClusterRoleDev
	case PermissionTypeReadonly:
		return ClusterRoleReadonly
	default:
		return ""
	}
}

// GetServiceAccountByPermissionType 根據權限型別獲取 ServiceAccount 名稱
func GetServiceAccountByPermissionType(permissionType string) string {
	switch permissionType {
	case PermissionTypeAdmin:
		return SAClusterAdmin
	case PermissionTypeOps:
		return SAOps
	case PermissionTypeDev:
		return SADev
	case PermissionTypeReadonly:
		return SAReadonly
	default:
		return ""
	}
}

// ClusterPermissionResponse 叢集權限響應結構
type ClusterPermissionResponse struct {
	ID             uint      `json:"id"`
	ClusterID      uint      `json:"cluster_id"`
	ClusterName    string    `json:"cluster_name,omitempty"`
	UserID         *uint     `json:"user_id,omitempty"`
	Username       string    `json:"username,omitempty"`
	UserGroupID    *uint     `json:"user_group_id,omitempty"`
	UserGroupName  string    `json:"user_group_name,omitempty"`
	PermissionType string    `json:"permission_type"`
	PermissionName string    `json:"permission_name"`
	Namespaces     []string  `json:"namespaces"`
	CustomRoleRef  string    `json:"custom_role_ref,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ToResponse 轉換為響應結構
func (cp *ClusterPermission) ToResponse() ClusterPermissionResponse {
	resp := ClusterPermissionResponse{
		ID:             cp.ID,
		ClusterID:      cp.ClusterID,
		UserID:         cp.UserID,
		UserGroupID:    cp.UserGroupID,
		PermissionType: cp.PermissionType,
		Namespaces:     cp.GetNamespaceList(),
		CustomRoleRef:  cp.CustomRoleRef,
		CreatedAt:      cp.CreatedAt,
		UpdatedAt:      cp.UpdatedAt,
	}

	// 獲取權限型別名稱
	for _, pt := range GetPermissionTypes() {
		if pt.Type == cp.PermissionType {
			resp.PermissionName = pt.Name
			break
		}
	}

	// 填充關聯資訊
	if cp.Cluster != nil {
		resp.ClusterName = cp.Cluster.Name
	}
	if cp.User != nil {
		resp.Username = cp.User.Username
	}
	if cp.UserGroup != nil {
		resp.UserGroupName = cp.UserGroup.Name
	}

	return resp
}

// MyPermissionsResponse 使用者權限響應
type MyPermissionsResponse struct {
	ClusterID       uint     `json:"cluster_id"`
	ClusterName     string   `json:"cluster_name"`
	PermissionType  string   `json:"permission_type"`
	PermissionName  string   `json:"permission_name"`
	Namespaces      []string `json:"namespaces"`
	AllowedActions  []string `json:"allowed_actions"`
	CustomRoleRef   string   `json:"custom_role_ref,omitempty"`
	AllowedFeatures []string `json:"allowed_features"` // 已套用 ceiling ∩ feature_policy 後的有效功能集合
}
