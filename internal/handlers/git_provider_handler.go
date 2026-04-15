package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// GitProviderHandler — Git Provider CRUD（CICD_ARCHITECTURE §10, M14）
//
// PlatformAdmin 專用：管理 GitHub / GitLab / Gitea 連線設定。
// ---------------------------------------------------------------------------

// GitProviderHandler 管理 Git Provider CRUD API。
type GitProviderHandler struct {
	providerSvc *services.GitProviderService
}

// NewGitProviderHandler 建立 GitProviderHandler。
func NewGitProviderHandler(providerSvc *services.GitProviderService) *GitProviderHandler {
	return &GitProviderHandler{providerSvc: providerSvc}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// CreateGitProviderRequest 建立 Git Provider 的請求。
type CreateGitProviderRequest struct {
	Name          string `json:"name" binding:"required,max=255"`
	Type          string `json:"type" binding:"required,oneof=github gitlab gitea"`
	BaseURL       string `json:"base_url" binding:"required,url,max=512"`
	AccessToken   string `json:"access_token"`   // 明文傳入，BeforeSave 加密
	WebhookSecret string `json:"webhook_secret"`  // 明文傳入，BeforeSave 加密
}

// UpdateGitProviderRequest 更新 Git Provider 的請求。
type UpdateGitProviderRequest struct {
	Name          *string `json:"name,omitempty"`
	BaseURL       *string `json:"base_url,omitempty"`
	AccessToken   *string `json:"access_token,omitempty"`
	WebhookSecret *string `json:"webhook_secret,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// List 列出所有 Git Provider。
// GET /admin/git-providers
func (h *GitProviderHandler) List(c *gin.Context) {
	providers, err := h.providerSvc.ListProviders(c.Request.Context())
	if err != nil {
		logger.Error("failed to list git providers", "error", err)
		response.InternalError(c, "failed to list git providers: "+err.Error())
		return
	}
	response.List(c, providers, int64(len(providers)))
}

// Get 取得單一 Git Provider。
// GET /admin/git-providers/:id
func (h *GitProviderHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider ID")
		return
	}

	provider, err := h.providerSvc.GetProvider(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "git provider not found")
		return
	}
	response.OK(c, provider)
}

// Create 建立新的 Git Provider。
// POST /admin/git-providers
func (h *GitProviderHandler) Create(c *gin.Context) {
	var req CreateGitProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	// 從 auth context 取得當前用戶 ID
	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)

	provider := &models.GitProvider{
		Name:             req.Name,
		Type:             req.Type,
		BaseURL:          req.BaseURL,
		AccessTokenEnc:   req.AccessToken,
		WebhookSecretEnc: req.WebhookSecret,
		Enabled:          true,
		CreatedBy:        uid,
	}

	if err := h.providerSvc.CreateProvider(c.Request.Context(), provider); err != nil {
		logger.Error("failed to create git provider", "error", err)
		response.InternalError(c, "failed to create git provider: "+err.Error())
		return
	}

	logger.Info("git provider created",
		"provider_id", provider.ID,
		"name", provider.Name,
		"type", provider.Type,
	)

	c.JSON(http.StatusCreated, gin.H{
		"id":            provider.ID,
		"name":          provider.Name,
		"type":          provider.Type,
		"webhook_token": provider.WebhookToken,
		"webhook_url":   "/api/v1/webhooks/git/" + provider.WebhookToken,
	})
}

// Update 更新 Git Provider。
// PUT /admin/git-providers/:id
func (h *GitProviderHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider ID")
		return
	}

	var req UpdateGitProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.AccessToken != nil {
		updates["access_token_enc"] = *req.AccessToken
	}
	if req.WebhookSecret != nil {
		updates["webhook_secret_enc"] = *req.WebhookSecret
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	if err := h.providerSvc.UpdateProvider(c.Request.Context(), uint(id), updates); err != nil {
		logger.Error("failed to update git provider", "provider_id", id, "error", err)
		response.InternalError(c, "failed to update git provider: "+err.Error())
		return
	}

	logger.Info("git provider updated", "provider_id", id)
	response.OK(c, gin.H{"message": "updated"})
}

// Delete 刪除 Git Provider。
// DELETE /admin/git-providers/:id
func (h *GitProviderHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider ID")
		return
	}

	if err := h.providerSvc.DeleteProvider(c.Request.Context(), uint(id)); err != nil {
		logger.Error("failed to delete git provider", "provider_id", id, "error", err)
		response.InternalError(c, "failed to delete git provider: "+err.Error())
		return
	}

	logger.Info("git provider deleted", "provider_id", id)
	response.OK(c, gin.H{"message": "deleted"})
}

// RegenerateToken 重新生成 Webhook Token。
// POST /admin/git-providers/:id/regenerate-token
func (h *GitProviderHandler) RegenerateToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider ID")
		return
	}

	newToken, err := h.providerSvc.RegenerateWebhookToken(c.Request.Context(), uint(id))
	if err != nil {
		logger.Error("failed to regenerate webhook token", "provider_id", id, "error", err)
		response.InternalError(c, "failed to regenerate token: "+err.Error())
		return
	}

	logger.Info("webhook token regenerated", "provider_id", id)
	response.OK(c, gin.H{
		"webhook_token": newToken,
		"webhook_url":   "/api/v1/webhooks/git/" + newToken,
	})
}
