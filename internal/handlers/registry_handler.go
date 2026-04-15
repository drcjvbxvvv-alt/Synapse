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
// RegistryHandler — Registry CRUD（CICD_ARCHITECTURE §11, M15）
//
// PlatformAdmin 專用：管理 Harbor / DockerHub / ECR / GCR / ACR 連線設定。
// ---------------------------------------------------------------------------

// RegistryHandler 管理 Registry CRUD API。
type RegistryHandler struct {
	registrySvc *services.RegistryService
}

// NewRegistryHandler 建立 RegistryHandler。
func NewRegistryHandler(registrySvc *services.RegistryService) *RegistryHandler {
	return &RegistryHandler{registrySvc: registrySvc}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// CreateRegistryRequest 建立 Registry 的請求。
type CreateRegistryRequest struct {
	Name           string `json:"name" binding:"required,max=255"`
	Type           string `json:"type" binding:"required,oneof=harbor dockerhub acr ecr gcr"`
	URL            string `json:"url" binding:"required,url,max=512"`
	Username       string `json:"username"`
	Password       string `json:"password"`        // 明文傳入，BeforeSave 加密
	InsecureTLS    bool   `json:"insecure_tls"`
	CABundle       string `json:"ca_bundle"`        // 明文傳入，BeforeSave 加密
	DefaultProject string `json:"default_project"`
}

// UpdateRegistryRequest 更新 Registry 的請求。
type UpdateRegistryRequest struct {
	Name           *string `json:"name,omitempty"`
	URL            *string `json:"url,omitempty"`
	Username       *string `json:"username,omitempty"`
	Password       *string `json:"password,omitempty"`
	InsecureTLS    *bool   `json:"insecure_tls,omitempty"`
	CABundle       *string `json:"ca_bundle,omitempty"`
	DefaultProject *string `json:"default_project,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// List 列出所有 Registry。
// GET /system/registries
func (h *RegistryHandler) List(c *gin.Context) {
	registries, err := h.registrySvc.ListRegistries(c.Request.Context())
	if err != nil {
		logger.Error("failed to list registries", "error", err)
		response.InternalError(c, "failed to list registries: "+err.Error())
		return
	}
	response.List(c, registries, int64(len(registries)))
}

// Get 取得單一 Registry。
// GET /system/registries/:id
func (h *RegistryHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	registry, err := h.registrySvc.GetRegistry(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "registry not found")
		return
	}
	response.OK(c, registry)
}

// Create 建立新的 Registry。
// POST /system/registries
func (h *RegistryHandler) Create(c *gin.Context) {
	var req CreateRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)

	registry := &models.Registry{
		Name:           req.Name,
		Type:           req.Type,
		URL:            req.URL,
		Username:       req.Username,
		PasswordEnc:    req.Password,
		InsecureTLS:    req.InsecureTLS,
		CABundleEnc:    req.CABundle,
		DefaultProject: req.DefaultProject,
		Enabled:        true,
		CreatedBy:      uid,
	}

	if err := h.registrySvc.CreateRegistry(c.Request.Context(), registry); err != nil {
		logger.Error("failed to create registry", "error", err)
		response.InternalError(c, "failed to create registry: "+err.Error())
		return
	}

	logger.Info("registry created",
		"registry_id", registry.ID,
		"name", registry.Name,
		"type", registry.Type,
	)

	c.JSON(http.StatusCreated, gin.H{
		"id":   registry.ID,
		"name": registry.Name,
		"type": registry.Type,
		"url":  registry.URL,
	})
}

// Update 更新 Registry。
// PUT /system/registries/:id
func (h *RegistryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	var req UpdateRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		updates["password_enc"] = *req.Password
	}
	if req.InsecureTLS != nil {
		updates["insecure_tls"] = *req.InsecureTLS
	}
	if req.CABundle != nil {
		updates["ca_bundle_enc"] = *req.CABundle
	}
	if req.DefaultProject != nil {
		updates["default_project"] = *req.DefaultProject
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	if err := h.registrySvc.UpdateRegistry(c.Request.Context(), uint(id), updates); err != nil {
		logger.Error("failed to update registry", "registry_id", id, "error", err)
		response.InternalError(c, "failed to update registry: "+err.Error())
		return
	}

	logger.Info("registry updated", "registry_id", id)
	response.OK(c, gin.H{"message": "updated"})
}

// Delete 刪除 Registry。
// DELETE /system/registries/:id
func (h *RegistryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	if err := h.registrySvc.DeleteRegistry(c.Request.Context(), uint(id)); err != nil {
		logger.Error("failed to delete registry", "registry_id", id, "error", err)
		response.InternalError(c, "failed to delete registry: "+err.Error())
		return
	}

	logger.Info("registry deleted", "registry_id", id)
	response.OK(c, gin.H{"message": "deleted"})
}

// TestConnection 測試 Registry 連線。
// POST /system/registries/:id/test-connection
func (h *RegistryHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	if err := h.registrySvc.TestConnection(c.Request.Context(), uint(id)); err != nil {
		logger.Warn("registry connection test failed", "registry_id", id, "error", err)
		response.OK(c, gin.H{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}

	logger.Info("registry connection test passed", "registry_id", id)
	response.OK(c, gin.H{"connected": true})
}

// ListRepositories 列出 Registry 中的 Repository。
// GET /system/registries/:id/repositories
func (h *RegistryHandler) ListRepositories(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	registry, err := h.registrySvc.GetRegistry(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "registry not found")
		return
	}

	adapter, err := services.NewRegistryAdapter(registry)
	if err != nil {
		response.InternalError(c, "failed to create registry adapter: "+err.Error())
		return
	}

	project := c.DefaultQuery("project", registry.DefaultProject)
	repos, err := adapter.ListRepositories(c.Request.Context(), project)
	if err != nil {
		logger.Error("failed to list repositories", "registry_id", id, "error", err)
		response.InternalError(c, "failed to list repositories: "+err.Error())
		return
	}

	response.List(c, repos, int64(len(repos)))
}

// ListTags 列出 Repository 中的 Tag。
// GET /system/registries/:id/tags?repository=xxx
func (h *RegistryHandler) ListTags(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	repository := c.Query("repository")
	if repository == "" {
		response.BadRequest(c, "repository query parameter is required")
		return
	}

	registry, err := h.registrySvc.GetRegistry(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "registry not found")
		return
	}

	adapter, err := services.NewRegistryAdapter(registry)
	if err != nil {
		response.InternalError(c, "failed to create registry adapter: "+err.Error())
		return
	}

	tags, err := adapter.ListTags(c.Request.Context(), repository)
	if err != nil {
		logger.Error("failed to list tags", "registry_id", id, "repository", repository, "error", err)
		response.InternalError(c, "failed to list tags: "+err.Error())
		return
	}

	response.List(c, tags, int64(len(tags)))
}
