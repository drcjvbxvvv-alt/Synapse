package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// UserHandler 使用者管理處理器
type UserHandler struct {
	userService *services.UserService
}

// NewUserHandler 建立使用者管理處理器
func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// ListUsers 獲取使用者列表
//
// @Summary     取得使用者列表
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Param       page      query int    false "頁碼（預設 1）"
// @Param       pageSize  query int    false "每頁筆數（預設 20，最大 100）"
// @Param       search    query string false "搜尋使用者名稱 / Email"
// @Param       status    query string false "狀態篩選（active / inactive）"
// @Param       auth_type query string false "認證類型篩選（local / ldap）"
// @Success     200 {object} response.PagedListResult
// @Failure     401 {object} response.ErrorBody
// @Failure     403 {object} response.ErrorBody
// @Router      /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	params := &services.ListUsersParams{
		Page:     page,
		PageSize: pageSize,
		Search:   c.Query("search"),
		Status:   c.Query("status"),
		AuthType: c.Query("auth_type"),
	}

	users, total, err := h.userService.ListUsers(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.PagedList(c, users, total, page, pageSize)
}

// GetUser 獲取使用者詳情
//
// @Summary     取得單一使用者
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "使用者 ID"
// @Success     200 {object} models.User
// @Failure     401 {object} response.ErrorBody
// @Failure     404 {object} response.ErrorBody
// @Router      /users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者ID")
		return
	}

	user, err := h.userService.GetUser(c.Request.Context(), uint(id))
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, user)
}

// CreateUser 建立使用者
//
// @Summary     建立使用者（平台管理員）
// @Tags        users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body services.CreateUserRequest true "使用者資訊"
// @Success     200 {object} models.User
// @Failure     400 {object} response.ErrorBody
// @Failure     401 {object} response.ErrorBody
// @Failure     403 {object} response.ErrorBody
// @Router      /users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req services.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	user, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	logger.Info("建立使用者: %s", user.Username)
	response.OK(c, user)
}

// UpdateUser 更新使用者
//
// @Summary     更新使用者資訊
// @Tags        users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path int                       true "使用者 ID"
// @Param       body body services.UpdateUserRequest true "更新資訊"
// @Success     200 {object} models.User
// @Failure     400 {object} response.ErrorBody
// @Failure     404 {object} response.ErrorBody
// @Router      /users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者ID")
		return
	}

	var req services.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	user, err := h.userService.UpdateUser(c.Request.Context(), uint(id), &req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, user)
}

// DeleteUser 刪除使用者
//
// @Summary     刪除使用者（平台管理員）
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "使用者 ID"
// @Success     200
// @Failure     400 {object} response.ErrorBody
// @Failure     404 {object} response.ErrorBody
// @Router      /users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者ID")
		return
	}

	// 不能刪除自己
	currentUserID := c.GetUint("user_id")
	if currentUserID == uint(id) {
		response.BadRequest(c, "不能刪除自己")
		return
	}

	if err := h.userService.DeleteUser(c.Request.Context(), uint(id)); err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, nil)
}

// UpdateStatusRequest 更新使用者狀態請求
type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateUserStatus 更新使用者狀態
//
// @Summary     啟用 / 停用使用者
// @Tags        users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path int                 true "使用者 ID"
// @Param       body body UpdateStatusRequest true "狀態（active / inactive）"
// @Success     200
// @Failure     400 {object} response.ErrorBody
// @Router      /users/{id}/status [put]
func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者ID")
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	if err := h.userService.UpdateUserStatus(c.Request.Context(), uint(id), req.Status); err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, nil)
}

// ResetPasswordRequest 重置密碼請求
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPassword 重置使用者密碼
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的使用者ID")
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	if err := h.userService.ResetPassword(c.Request.Context(), uint(id), req.NewPassword); err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, nil)
}
