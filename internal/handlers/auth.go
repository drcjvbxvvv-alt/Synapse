package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/constants"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// AuthHandler 認證處理器
type AuthHandler struct {
	authService  *services.AuthService
	opLogSvc     *services.OperationLogService
	blacklistSvc *services.TokenBlacklistService
}

// NewAuthHandler 建立認證處理器
func NewAuthHandler(
	authService *services.AuthService,
	opLogSvc *services.OperationLogService,
	blacklistSvc *services.TokenBlacklistService,
) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		opLogSvc:     opLogSvc,
		blacklistSvc: blacklistSvc,
	}
}

// LoginRequest 登入請求結構
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	AuthType string `json:"auth_type"` // 認證型別：local, ldap，預設local
}

// Login 使用者登入 - 支援本地密碼和LDAP兩種認證方式
//
// @Summary     使用者登入
// @Description 支援 local / ldap 兩種認證方式，成功後回傳 access token
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body LoginRequest true "登入資訊"
// @Success     200 {object} services.LoginResult
// @Failure     400 {object} response.ErrorBody
// @Failure     401 {object} response.ErrorBody
// @Router      /auth/login [post]
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

	// 簽發 refresh token 並存入 httpOnly cookie
	refreshToken, _, err := h.authService.GenerateRefreshToken(&result.User)
	if err != nil {
		logger.Warn("生成 refresh token 失敗，不影響登入", "error", err)
	} else {
		secure := c.Request.TLS != nil
		c.SetCookie(
			services.RefreshTokenCookieName,
			refreshToken,
			int(services.RefreshTokenExpireDays*24*60*60), // MaxAge in seconds
			"/api/v1/auth",
			"",
			secure,
			true, // HttpOnly
		)
	}

	response.OK(c, result)
}

// RefreshToken 用 httpOnly cookie 中的 refresh token 換取新的 access token
//
// @Summary     Refresh access token
// @Description 使用 httpOnly cookie 中的 refresh token 無聲換取新 access token（頁面重新整理使用）
// @Tags        auth
// @Produce     json
// @Success     200 {object} services.LoginResult
// @Failure     401 {object} response.ErrorBody
// @Router      /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(services.RefreshTokenCookieName)
	if err != nil || refreshToken == "" {
		response.Unauthorized(c, "缺少 refresh token")
		return
	}

	result, err := h.authService.IssueAccessToken(refreshToken)
	if err != nil {
		// clear stale cookie
		c.SetCookie(services.RefreshTokenCookieName, "", -1, "/api/v1/auth", "", false, true)
		response.FromError(c, err)
		return
	}

	response.OK(c, result)
}

// Logout 使用者登出
//
// 流程（P0-5）：
//  1. 從 context 取出 jti 與 token_exp（由 AuthRequired 中介軟體寫入）
//  2. 呼叫 TokenBlacklistService.Revoke 將 jti 寫入黑名單（含 DB + 記憶體快取）
//  3. 記錄審計日誌
//
// 若 blacklistSvc 尚未啟用（測試場景），僅略過撤銷步驟，其餘流程正常進行。
// @Summary     使用者登出
// @Description 撤銷當前 access token（加入黑名單），清除 refresh token cookie
// @Tags        auth
// @Produce     json
// @Security    BearerAuth
// @Success     200
// @Failure     401 {object} response.ErrorBody
// @Router      /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var userIDPtr *uint
	var userID uint
	username := ""
	if uid := c.GetUint("user_id"); uid > 0 {
		userID = uid
		userIDPtr = &uid
	}
	if un := c.GetString("username"); un != "" {
		username = un
	}

	// 將 token 加入黑名單
	if h.blacklistSvc != nil {
		jti := c.GetString("jti")
		tokenExp, _ := c.Get("token_exp")
		expTime, _ := tokenExp.(time.Time)
		if jti != "" && !expTime.IsZero() {
			if err := h.blacklistSvc.Revoke(
				c.Request.Context(),
				jti,
				userID,
				expTime,
				models.TokenRevokeReasonLogout,
			); err != nil {
				logger.Error("登出時寫入 token 黑名單失敗",
					"error", err,
					"jti", jti,
					"user_id", userID,
				)
			}
		}
	}

	// 清除 refresh token cookie
	c.SetCookie(services.RefreshTokenCookieName, "", -1, "/api/v1/auth", "", false, true)

	logger.Info("使用者登出",
		"user_id", userID,
		"username", username,
	)

	// 記錄登出審計日誌
	if h.opLogSvc != nil {
		h.opLogSvc.RecordAsync(&services.LogEntry{
			UserID:       userIDPtr,
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
//
// @Summary     取得目前登入使用者資訊
// @Tags        auth
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} models.User
// @Failure     401 {object} response.ErrorBody
// @Router      /auth/me [get]
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
//
// @Summary     取得認證系統狀態
// @Description 回傳 LDAP 是否已啟用（供登入頁判斷顯示選項）
// @Tags        auth
// @Produce     json
// @Success     200 {object} AuthStatusResponse
// @Router      /auth/status [get]
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
//
// @Summary     修改密碼
// @Tags        auth
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body ChangePasswordRequest true "新舊密碼"
// @Success     200
// @Failure     400 {object} response.ErrorBody
// @Failure     401 {object} response.ErrorBody
// @Router      /auth/change-password [post]
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
