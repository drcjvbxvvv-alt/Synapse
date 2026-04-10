package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
)

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
