package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
)

// ========== 權限型別 ==========

// GetPermissionTypes 獲取權限型別列表
func (h *PermissionHandler) GetPermissionTypes(c *gin.Context) {
	types := models.GetPermissionTypes()
	response.OK(c, types)
}

// ========== 使用者組管理 ==========

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
