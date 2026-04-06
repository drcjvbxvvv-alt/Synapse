package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/clay-wangzhi/Synapse/internal/apierrors"
	"github.com/clay-wangzhi/Synapse/internal/constants"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

// AuthHandler 認證處理器
type AuthHandler struct {
	authService *services.AuthService
	opLogSvc    *services.OperationLogService
}

// NewAuthHandler 建立認證處理器
func NewAuthHandler(authService *services.AuthService, opLogSvc *services.OperationLogService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		opLogSvc:    opLogSvc,
	}
}

// LoginRequest 登入請求結構
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	AuthType string `json:"auth_type"` // 認證型別：local, ldap，預設local
}

// Login 使用者登入 - 支援本地密碼和LDAP兩種認證方式
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	result, err := h.authService.Login(req.Username, req.Password, req.AuthType, c.ClientIP())
	if err != nil {
		logger.Warn("使用者登入失敗: %s, 錯誤: %v", req.Username, err)

		// 從 AppError 中提取狀態碼（fallback 401）
		statusCode := http.StatusUnauthorized
		if ae, ok := apierrors.As(err); ok {
			statusCode = ae.HTTPStatus
		}

		// 記錄登入失敗審計日誌
		if h.opLogSvc != nil {
			h.opLogSvc.RecordAsync(&services.LogEntry{
				Username:     req.Username,
				Method:       "POST",
				Path:         "/api/v1/auth/login",
				Module:       constants.ModuleAuth,
				Action:       constants.ActionLoginFailed,
				ResourceType: "user",
				ResourceName: req.Username,
				StatusCode:   statusCode,
				Success:      false,
				ErrorMessage: err.Error(),
				ClientIP:     c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
			})
		}

		response.FromError(c, err)
		return
	}

	// 記錄登入成功審計日誌
	if h.opLogSvc != nil {
		userID := result.User.ID
		h.opLogSvc.RecordAsync(&services.LogEntry{
			UserID:       &userID,
			Username:     result.User.Username,
			Method:       "POST",
			Path:         "/api/v1/auth/login",
			Module:       constants.ModuleAuth,
			Action:       constants.ActionLogin,
			ResourceType: "user",
			ResourceName: result.User.Username,
			StatusCode:   200,
			Success:      true,
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
		})
	}

	response.OK(c, result)
}

// Logout 使用者登出
func (h *AuthHandler) Logout(c *gin.Context) {
	var userID *uint
	username := ""
	if uid := c.GetUint("user_id"); uid > 0 {
		userID = &uid
	}
	if un := c.GetString("username"); un != "" {
		username = un
	}

	// 記錄登出審計日誌
	if h.opLogSvc != nil {
		h.opLogSvc.RecordAsync(&services.LogEntry{
			UserID:       userID,
			Username:     username,
			Method:       "POST",
			Path:         "/api/v1/auth/logout",
			Module:       constants.ModuleAuth,
			Action:       constants.ActionLogout,
			ResourceType: "user",
			ResourceName: username,
			StatusCode:   200,
			Success:      true,
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
		})
	}

	response.OK(c, nil)
}

// GetProfile 獲取使用者資訊
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		response.Unauthorized(c, "無效的使用者認證資訊")
		return
	}

	user, err := h.authService.GetProfile(userID)
	if err != nil {
		response.NotFound(c, "使用者不存在")
		return
	}

	response.OK(c, user)
}

// AuthStatusResponse 認證狀態響應
type AuthStatusResponse struct {
	LDAPEnabled bool `json:"ldap_enabled"`
}

// GetAuthStatus 獲取認證狀態（無需登入即可訪問）
func (h *AuthHandler) GetAuthStatus(c *gin.Context) {
	ldapEnabled, _ := h.authService.GetAuthStatus()

	response.OK(c, AuthStatusResponse{
		LDAPEnabled: ldapEnabled,
	})
}

// ChangePasswordRequest 修改密碼請求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword 修改密碼（僅限本地使用者）
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		response.Unauthorized(c, "無效的使用者認證資訊")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.OK(c, nil)
}
