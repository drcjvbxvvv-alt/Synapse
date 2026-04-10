package handlers

import (
	"github.com/shaia/Synapse/internal/services"
)

// PermissionHandler 權限管理處理器
type PermissionHandler struct {
	permissionService *services.PermissionService
	clusterService    *services.ClusterService
	rbacService       *services.RBACService
}

// NewPermissionHandler 建立權限管理處理器
func NewPermissionHandler(permissionService *services.PermissionService, clusterService *services.ClusterService, rbacService *services.RBACService) *PermissionHandler {
	return &PermissionHandler{
		permissionService: permissionService,
		clusterService:    clusterService,
		rbacService:       rbacService,
	}
}

// ========== DTOs ==========

// CreateUserGroupRequest 建立使用者組請求
type CreateUserGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateUserGroupRequest 更新使用者組請求
type UpdateUserGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// AddUserToGroupRequest 新增使用者到使用者組請求
type AddUserToGroupRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

// CreateClusterPermissionRequest 建立叢集權限請求
type CreateClusterPermissionRequest struct {
	ClusterID      uint     `json:"cluster_id" binding:"required"`
	UserID         *uint    `json:"user_id"`
	UserGroupID    *uint    `json:"user_group_id"`
	PermissionType string   `json:"permission_type" binding:"required"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
	// 批次欄位：與 UserID/UserGroupID 互斥，支援同時為多個使用者和使用者組建立權限
	UserIDs      []uint `json:"user_ids"`
	UserGroupIDs []uint `json:"user_group_ids"`
}

// UpdateClusterPermissionRequest 更新叢集權限請求
type UpdateClusterPermissionRequest struct {
	PermissionType string   `json:"permission_type"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
}

// BatchDeleteClusterPermissionsRequest 批次刪除請求
type BatchDeleteClusterPermissionsRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}
