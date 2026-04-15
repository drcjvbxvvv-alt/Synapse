package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineSecretHandler — Pipeline Secret CRUD
// ---------------------------------------------------------------------------

// PipelineSecretHandler 管理 Pipeline Secret 的 HTTP 端點。
type PipelineSecretHandler struct {
	secretSvc *services.PipelineSecretService
}

// NewPipelineSecretHandler 建立 PipelineSecretHandler。
func NewPipelineSecretHandler(secretSvc *services.PipelineSecretService) *PipelineSecretHandler {
	return &PipelineSecretHandler{secretSvc: secretSvc}
}

// CreateSecret 建立 Pipeline Secret。
// POST /clusters/:clusterID/pipeline-secrets
func (h *PipelineSecretHandler) CreateSecret(c *gin.Context) {
	var req services.CreateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	userID := c.GetUint("user_id")
	secret, err := h.secretSvc.CreateSecret(c.Request.Context(), &req, userID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to create secret: "+err.Error())
		return
	}

	logger.Info("pipeline secret created",
		"secret_id", secret.ID,
		"scope", secret.Scope,
		"name", secret.Name,
		"user_id", userID,
	)
	response.Created(c, gin.H{
		"id":          secret.ID,
		"scope":       secret.Scope,
		"scope_ref":   secret.ScopeRef,
		"name":        secret.Name,
		"description": secret.Description,
		"created_at":  secret.CreatedAt,
	})
}

// GetSecret 取得單一 Secret（不含值）。
// GET /clusters/:clusterID/pipeline-secrets/:secretID
func (h *PipelineSecretHandler) GetSecret(c *gin.Context) {
	secretID, err := parseUintParam(c, "secretID")
	if err != nil {
		response.BadRequest(c, "invalid secret ID")
		return
	}

	secret, err := h.secretSvc.GetSecret(c.Request.Context(), secretID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get secret: "+err.Error())
		return
	}

	// 永遠不回傳加密值
	response.OK(c, gin.H{
		"id":          secret.ID,
		"scope":       secret.Scope,
		"scope_ref":   secret.ScopeRef,
		"name":        secret.Name,
		"description": secret.Description,
		"created_by":  secret.CreatedBy,
		"created_at":  secret.CreatedAt,
		"updated_at":  secret.UpdatedAt,
	})
}

// ListSecrets 列出 Secrets。
// GET /clusters/:clusterID/pipeline-secrets?scope=cluster&scope_ref=1
func (h *PipelineSecretHandler) ListSecrets(c *gin.Context) {
	scope := c.Query("scope")
	var scopeRef *uint
	if scopeRefStr := c.Query("scope_ref"); scopeRefStr != "" {
		if v, err := parseClusterID(scopeRefStr); err == nil {
			scopeRef = &v
		}
	}

	secrets, err := h.secretSvc.ListSecrets(c.Request.Context(), scope, scopeRef)
	if err != nil {
		response.InternalError(c, "failed to list secrets: "+err.Error())
		return
	}

	response.OK(c, secrets)
}

// UpdateSecret 更新 Secret。
// PUT /clusters/:clusterID/pipeline-secrets/:secretID
func (h *PipelineSecretHandler) UpdateSecret(c *gin.Context) {
	secretID, err := parseUintParam(c, "secretID")
	if err != nil {
		response.BadRequest(c, "invalid secret ID")
		return
	}

	var req services.UpdateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	secret, err := h.secretSvc.UpdateSecret(c.Request.Context(), secretID, &req)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to update secret: "+err.Error())
		return
	}

	logger.Info("pipeline secret updated", "secret_id", secretID, "user_id", c.GetUint("user_id"))
	response.OK(c, gin.H{
		"id":          secret.ID,
		"scope":       secret.Scope,
		"scope_ref":   secret.ScopeRef,
		"name":        secret.Name,
		"description": secret.Description,
		"updated_at":  secret.UpdatedAt,
	})
}

// ---------------------------------------------------------------------------
// 3-level scope convenience endpoints（H5）
// ---------------------------------------------------------------------------

// ListPipelineSecrets 列出 Pipeline 範疇的 Secrets。
// GET /pipelines/:pipelineID/secrets
func (h *PipelineSecretHandler) ListPipelineSecrets(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}
	secrets, err := h.secretSvc.ListSecrets(c.Request.Context(), "pipeline", &pipelineID)
	if err != nil {
		response.InternalError(c, "failed to list pipeline secrets: "+err.Error())
		return
	}
	response.OK(c, secrets)
}

// scopedSecretBody is the request body for scope-convenience endpoints.
// Scope and ScopeRef are determined by the URL — not required from the client.
type scopedSecretBody struct {
	Name        string `json:"name"        binding:"required,max=100"`
	Value       string `json:"value"       binding:"required"`
	Description string `json:"description" binding:"max=255"`
}

// CreatePipelineSecret 在 Pipeline 範疇建立 Secret。
// POST /pipelines/:pipelineID/secrets
func (h *PipelineSecretHandler) CreatePipelineSecret(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var body scopedSecretBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	req := services.CreateSecretRequest{
		Scope:       "pipeline",
		ScopeRef:    &pipelineID,
		Name:        body.Name,
		Value:       body.Value,
		Description: body.Description,
	}

	userID := c.GetUint("user_id")
	secret, err := h.secretSvc.CreateSecret(c.Request.Context(), &req, userID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to create pipeline secret: "+err.Error())
		return
	}

	logger.Info("pipeline-scoped secret created",
		"secret_id", secret.ID,
		"pipeline_id", pipelineID,
		"name", secret.Name,
	)
	response.Created(c, gin.H{
		"id": secret.ID, "scope": secret.Scope,
		"scope_ref": secret.ScopeRef, "name": secret.Name,
		"description": secret.Description, "created_at": secret.CreatedAt,
	})
}

// ListEnvSecrets 列出 Environment 範疇的 Secrets。
// GET /pipelines/:pipelineID/environments/:envID/secrets
func (h *PipelineSecretHandler) ListEnvSecrets(c *gin.Context) {
	envID, err := parseUintParam(c, "envID")
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}
	secrets, err := h.secretSvc.ListSecrets(c.Request.Context(), "environment", &envID)
	if err != nil {
		response.InternalError(c, "failed to list environment secrets: "+err.Error())
		return
	}
	response.OK(c, secrets)
}

// CreateEnvSecret 在 Environment 範疇建立 Secret。
// POST /pipelines/:pipelineID/environments/:envID/secrets
func (h *PipelineSecretHandler) CreateEnvSecret(c *gin.Context) {
	envID, err := parseUintParam(c, "envID")
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}

	var body scopedSecretBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	req := services.CreateSecretRequest{
		Scope:       "environment",
		ScopeRef:    &envID,
		Name:        body.Name,
		Value:       body.Value,
		Description: body.Description,
	}

	userID := c.GetUint("user_id")
	secret, err := h.secretSvc.CreateSecret(c.Request.Context(), &req, userID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to create environment secret: "+err.Error())
		return
	}

	logger.Info("environment-scoped secret created",
		"secret_id", secret.ID,
		"env_id", envID,
		"name", secret.Name,
	)
	response.Created(c, gin.H{
		"id": secret.ID, "scope": secret.Scope,
		"scope_ref": secret.ScopeRef, "name": secret.Name,
		"description": secret.Description, "created_at": secret.CreatedAt,
	})
}

// DeleteSecret 刪除 Secret。
// DELETE /clusters/:clusterID/pipeline-secrets/:secretID
func (h *PipelineSecretHandler) DeleteSecret(c *gin.Context) {
	secretID, err := parseUintParam(c, "secretID")
	if err != nil {
		response.BadRequest(c, "invalid secret ID")
		return
	}

	if err := h.secretSvc.DeleteSecret(c.Request.Context(), secretID); err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to delete secret: "+err.Error())
		return
	}

	logger.Info("pipeline secret deleted", "secret_id", secretID, "user_id", c.GetUint("user_id"))
	response.OK(c, gin.H{"message": "deleted"})
}
