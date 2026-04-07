package handlers

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
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

// ========== 權限型別 ==========

// GetPermissionTypes 獲取權限型別列表
func (h *PermissionHandler) GetPermissionTypes(c *gin.Context) {
	types := models.GetPermissionTypes()
	response.OK(c, types)
}

// ========== 使用者組管理 ==========

// CreateUserGroupRequest 建立使用者組請求
type CreateUserGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateUserGroup 建立使用者組
func (h *PermissionHandler) CreateUserGroup(c *gin.Context) {
	var req CreateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	group, err := h.permissionService.CreateUserGroup(req.Name, req.Description)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			response.Error(c, ae.HTTPStatus, ae.Code, ae.Message)
		} else {
			response.InternalError(c, err.Error())
		}
		return
	}

	response.OK(c, group)
}

// UpdateUserGroupRequest 更新使用者組請求
type UpdateUserGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateUserGroup 更新使用者組
func (h *PermissionHandler) UpdateUserGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者組ID")
		return
	}

	var req UpdateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	group, err := h.permissionService.UpdateUserGroup(uint(id), req.Name, req.Description)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, group)
}

// DeleteUserGroup 刪除使用者組
func (h *PermissionHandler) DeleteUserGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者組ID")
		return
	}

	if err := h.permissionService.DeleteUserGroup(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// GetUserGroup 獲取使用者組詳情
func (h *PermissionHandler) GetUserGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者組ID")
		return
	}

	group, err := h.permissionService.GetUserGroup(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.OK(c, group)
}

// ListUserGroups 獲取使用者組列表
func (h *PermissionHandler) ListUserGroups(c *gin.Context) {
	groups, err := h.permissionService.ListUserGroups()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, groups, int64(len(groups)))
}

// AddUserToGroupRequest 新增使用者到使用者組請求
type AddUserToGroupRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

// AddUserToGroup 新增使用者到使用者組
func (h *PermissionHandler) AddUserToGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者組ID")
		return
	}

	var req AddUserToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	if err := h.permissionService.AddUserToGroup(req.UserID, uint(groupID)); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// RemoveUserFromGroup 從使用者組移除使用者
func (h *PermissionHandler) RemoveUserFromGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者組ID")
		return
	}

	userID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者ID")
		return
	}

	if err := h.permissionService.RemoveUserFromGroup(uint(userID), uint(groupID)); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// ========== 叢集權限管理 ==========

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

// CreateClusterPermission 建立叢集權限（支援批次）
func (h *PermissionHandler) CreateClusterPermission(c *gin.Context) {
	var req CreateClusterPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	// 相容舊的單個使用者/使用者組欄位
	if req.UserID != nil && len(req.UserIDs) == 0 {
		req.UserIDs = []uint{*req.UserID}
	}
	if req.UserGroupID != nil && len(req.UserGroupIDs) == 0 {
		req.UserGroupIDs = []uint{*req.UserGroupID}
	}

	if len(req.UserIDs) == 0 && len(req.UserGroupIDs) == 0 {
		response.BadRequest(c, "至少需要指定一個使用者或使用者組")
		return
	}

	var created []models.ClusterPermissionResponse
	var errs []string

	// 為每個使用者建立權限
	for _, uid := range req.UserIDs {
		uidCopy := uid
		serviceReq := &services.CreateClusterPermissionRequest{
			ClusterID:      req.ClusterID,
			UserID:         &uidCopy,
			PermissionType: req.PermissionType,
			Namespaces:     req.Namespaces,
			CustomRoleRef:  req.CustomRoleRef,
		}
		permission, err := h.permissionService.CreateClusterPermission(serviceReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("使用者ID=%d: %s", uid, err.Error()))
			continue
		}
		go h.ensureUserRBACInCluster(permission)
		created = append(created, permission.ToResponse())
	}

	// 為每個使用者組建立權限
	for _, gid := range req.UserGroupIDs {
		gidCopy := gid
		serviceReq := &services.CreateClusterPermissionRequest{
			ClusterID:      req.ClusterID,
			UserGroupID:    &gidCopy,
			PermissionType: req.PermissionType,
			Namespaces:     req.Namespaces,
			CustomRoleRef:  req.CustomRoleRef,
		}
		permission, err := h.permissionService.CreateClusterPermission(serviceReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("使用者組ID=%d: %s", gid, err.Error()))
			continue
		}
		go h.ensureUserRBACInCluster(permission)
		created = append(created, permission.ToResponse())
	}

	if len(created) == 0 && len(errs) > 0 {
		response.BadRequest(c, "建立失敗")
		return
	}

	data := gin.H{"items": created, "count": len(created)}
	if len(errs) > 0 {
		data["errors"] = errs
	}
	response.OK(c, data)
}

// ensureUserRBACInCluster 確保使用者在叢集中有對應的 RBAC 資源
func (h *PermissionHandler) ensureUserRBACInCluster(permission *models.ClusterPermission) {
	// 只有使用者級別的權限才需要建立 RBAC（使用者組的權限需要特殊處理）
	if permission.UserID == nil {
		logger.Info("使用者組權限暫不自動建立 RBAC", "userGroupID", permission.UserGroupID)
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(permission.ClusterID)
	if err != nil {
		logger.Error("獲取叢集資訊失敗，無法建立 RBAC", "clusterID", permission.ClusterID, "error", err)
		return
	}

	// 建立 K8s 客戶端
	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		logger.Error("建立 K8s 客戶端失敗", "clusterID", permission.ClusterID, "error", err)
		return
	}

	// 解析命名空間
	namespaces := permission.GetNamespaceList()

	// 建立 RBAC 配置
	config := &services.UserRBACConfig{
		UserID:         *permission.UserID,
		PermissionType: permission.PermissionType,
		Namespaces:     namespaces,
		ClusterRoleRef: permission.CustomRoleRef,
	}

	// 確保 RBAC 資源存在
	if err := h.rbacService.EnsureUserRBAC(k8sClient.GetClientset(), config); err != nil {
		logger.Error("建立使用者 RBAC 失敗", "userID", *permission.UserID, "clusterID", permission.ClusterID, "error", err)
	} else {
		logger.Info("使用者 RBAC 建立成功", "userID", *permission.UserID, "clusterID", permission.ClusterID, "permissionType", permission.PermissionType)
	}
}

// UpdateClusterPermissionRequest 更新叢集權限請求
type UpdateClusterPermissionRequest struct {
	PermissionType string   `json:"permission_type"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
}

// UpdateClusterPermission 更新叢集權限
func (h *PermissionHandler) UpdateClusterPermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的權限ID")
		return
	}

	var req UpdateClusterPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	serviceReq := &services.UpdateClusterPermissionRequest{
		PermissionType: req.PermissionType,
		Namespaces:     req.Namespaces,
		CustomRoleRef:  req.CustomRoleRef,
	}

	// 獲取舊權限配置用於清理
	oldPermission, _ := h.permissionService.GetClusterPermission(uint(id))

	permission, err := h.permissionService.UpdateClusterPermission(uint(id), serviceReq)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 非同步更新 RBAC 資源
	go h.updateUserRBACInCluster(oldPermission, permission)

	response.OK(c, permission.ToResponse())
}

// updateUserRBACInCluster 更新使用者在叢集中的 RBAC 資源
func (h *PermissionHandler) updateUserRBACInCluster(oldPermission, newPermission *models.ClusterPermission) {
	if newPermission.UserID == nil {
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(newPermission.ClusterID)
	if err != nil {
		logger.Error("獲取叢集資訊失敗", "clusterID", newPermission.ClusterID, "error", err)
		return
	}

	// 建立 K8s 客戶端
	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		logger.Error("建立 K8s 客戶端失敗", "error", err)
		return
	}

	// 如果舊權限存在，先清理
	if oldPermission != nil && oldPermission.UserID != nil {
		oldNamespaces := oldPermission.GetNamespaceList()
		if err := h.rbacService.CleanupUserRBAC(k8sClient.GetClientset(), *oldPermission.UserID, oldPermission.PermissionType, oldNamespaces); err != nil {
			logger.Warn("清理舊 RBAC 失敗", "error", err)
		}
	}

	// 建立新 RBAC
	newNamespaces := newPermission.GetNamespaceList()
	config := &services.UserRBACConfig{
		UserID:         *newPermission.UserID,
		PermissionType: newPermission.PermissionType,
		Namespaces:     newNamespaces,
		ClusterRoleRef: newPermission.CustomRoleRef,
	}

	if err := h.rbacService.EnsureUserRBAC(k8sClient.GetClientset(), config); err != nil {
		logger.Error("更新使用者 RBAC 失敗", "error", err)
	} else {
		logger.Info("使用者 RBAC 更新成功", "userID", *newPermission.UserID, "clusterID", newPermission.ClusterID)
	}
}

// DeleteClusterPermission 刪除叢集權限
func (h *PermissionHandler) DeleteClusterPermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的權限ID")
		return
	}

	// 先獲取權限資訊用於清理 RBAC
	permission, _ := h.permissionService.GetClusterPermission(uint(id))

	if err := h.permissionService.DeleteClusterPermission(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 非同步清理 RBAC 資源
	if permission != nil {
		go h.cleanupUserRBACInCluster(permission)
	}

	response.OK(c, nil)
}

// cleanupUserRBACInCluster 清理使用者在叢集中的 RBAC 資源
func (h *PermissionHandler) cleanupUserRBACInCluster(permission *models.ClusterPermission) {
	if permission.UserID == nil {
		return
	}

	// 獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(permission.ClusterID)
	if err != nil {
		logger.Error("獲取叢集資訊失敗，無法清理 RBAC", "clusterID", permission.ClusterID, "error", err)
		return
	}

	// 建立 K8s 客戶端
	k8sClient, err := services.NewK8sClientForCluster(cluster)
	if err != nil {
		logger.Error("建立 K8s 客戶端失敗", "error", err)
		return
	}

	namespaces := permission.GetNamespaceList()
	if err := h.rbacService.CleanupUserRBAC(k8sClient.GetClientset(), *permission.UserID, permission.PermissionType, namespaces); err != nil {
		logger.Error("清理使用者 RBAC 失敗", "error", err)
	} else {
		logger.Info("使用者 RBAC 清理成功", "userID", *permission.UserID, "clusterID", permission.ClusterID)
	}
}

// BatchDeleteClusterPermissionsRequest 批次刪除請求
type BatchDeleteClusterPermissionsRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// BatchDeleteClusterPermissions 批次刪除叢集權限
func (h *PermissionHandler) BatchDeleteClusterPermissions(c *gin.Context) {
	var req BatchDeleteClusterPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	if err := h.permissionService.BatchDeleteClusterPermissions(req.IDs); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// GetClusterPermission 獲取叢集權限詳情
func (h *PermissionHandler) GetClusterPermission(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的權限ID")
		return
	}

	permission, err := h.permissionService.GetClusterPermission(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.OK(c, permission.ToResponse())
}

// ListClusterPermissions 獲取叢集權限列表
func (h *PermissionHandler) ListClusterPermissions(c *gin.Context) {
	clusterIDStr := c.Query("cluster_id")
	var clusterID uint
	if clusterIDStr != "" {
		id, err := strconv.ParseUint(clusterIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "無效的叢集ID")
			return
		}
		clusterID = uint(id)
	}

	permissions, err := h.permissionService.ListClusterPermissions(clusterID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 轉換為響應格式
	responses := make([]models.ClusterPermissionResponse, len(permissions))
	for i, p := range permissions {
		responses[i] = p.ToResponse()
	}

	response.List(c, responses, int64(len(responses)))
}

// ListAllClusterPermissions 獲取所有叢集權限列表
func (h *PermissionHandler) ListAllClusterPermissions(c *gin.Context) {
	permissions, err := h.permissionService.ListAllClusterPermissions()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 轉換為響應格式
	responses := make([]models.ClusterPermissionResponse, len(permissions))
	for i, p := range permissions {
		responses[i] = p.ToResponse()
	}

	response.List(c, responses, int64(len(responses)))
}

// ========== 使用者權限查詢 ==========

// GetMyPermissions 獲取當前使用者的權限
func (h *PermissionHandler) GetMyPermissions(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		response.Unauthorized(c, "未登入")
		return
	}

	permissions, err := h.permissionService.GetUserAllClusterPermissions(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 轉換為響應格式
	responses := make([]models.MyPermissionsResponse, len(permissions))
	for i, p := range permissions {
		permissionName := ""
		for _, pt := range models.GetPermissionTypes() {
			if pt.Type == p.PermissionType {
				permissionName = pt.Name
				break
			}
		}

		clusterName := ""
		if p.Cluster != nil {
			clusterName = p.Cluster.Name
		}

		responses[i] = models.MyPermissionsResponse{
			ClusterID:      p.ClusterID,
			ClusterName:    clusterName,
			PermissionType: p.PermissionType,
			PermissionName: permissionName,
			Namespaces:     p.GetNamespaceList(),
			CustomRoleRef:  p.CustomRoleRef,
		}
	}

	response.OK(c, responses)
}

// GetMyClusterPermission 獲取當前使用者在指定叢集的權限
func (h *PermissionHandler) GetMyClusterPermission(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		response.Unauthorized(c, "未登入")
		return
	}

	clusterID, err := strconv.ParseUint(c.Param("clusterID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	permission, err := h.permissionService.GetUserClusterPermission(userID, uint(clusterID))
	if err != nil {
		response.Forbidden(c, "無權限訪問該叢集")
		return
	}

	// 獲取權限型別資訊
	permissionName := ""
	var allowedActions []string
	for _, pt := range models.GetPermissionTypes() {
		if pt.Type == permission.PermissionType {
			permissionName = pt.Name
			allowedActions = pt.Actions
			break
		}
	}

	response.OK(c, models.MyPermissionsResponse{
		ClusterID:      permission.ClusterID,
		PermissionType: permission.PermissionType,
		PermissionName: permissionName,
		Namespaces:     permission.GetNamespaceList(),
		AllowedActions: allowedActions,
		CustomRoleRef:  permission.CustomRoleRef,
	})
}

// ========== 使用者列表 ==========

// ListUsers 獲取使用者列表（用於權限管理的使用者選擇）
func (h *PermissionHandler) ListUsers(c *gin.Context) {
	users, err := h.permissionService.ListUsers()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, users, int64(len(users)))
}
