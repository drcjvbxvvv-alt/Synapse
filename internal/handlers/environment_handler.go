package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// EnvironmentHandler — Pipeline Environment CRUD
// ---------------------------------------------------------------------------

// EnvironmentHandler 管理 Pipeline 的執行目標 Environment。
type EnvironmentHandler struct {
	envSvc *services.EnvironmentService
}

// NewEnvironmentHandler 建立 EnvironmentHandler。
func NewEnvironmentHandler(envSvc *services.EnvironmentService) *EnvironmentHandler {
	return &EnvironmentHandler{envSvc: envSvc}
}

// ListEnvironments 列出 Pipeline 的所有 Environments。
// GET /pipelines/:pipelineID/environments
func (h *EnvironmentHandler) ListEnvironments(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	envs, err := h.envSvc.GetEnvironmentsForPipeline(c.Request.Context(), pipelineID)
	if err != nil {
		response.InternalError(c, "failed to list environments: "+err.Error())
		return
	}

	response.List(c, envs, int64(len(envs)))
}

// GetEnvironment 取得單一 Environment。
// GET /pipelines/:pipelineID/environments/:envID
func (h *EnvironmentHandler) GetEnvironment(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}
	envID, err := parseUintParam(c, "envID")
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}

	env, err := h.envSvc.GetEnvironmentByPipelineAndID(c.Request.Context(), pipelineID, envID)
	if err != nil {
		response.NotFound(c, "environment not found")
		return
	}

	response.OK(c, env)
}

// CreateEnvironment 為 Pipeline 新增 Environment。
// POST /pipelines/:pipelineID/environments
func (h *EnvironmentHandler) CreateEnvironment(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var req services.CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	env, err := h.envSvc.CreateEnvironment(c.Request.Context(), pipelineID, &req)
	if err != nil {
		response.InternalError(c, "failed to create environment: "+err.Error())
		return
	}

	logger.Info("environment created via API",
		"pipeline_id", pipelineID,
		"env_id", env.ID,
		"name", env.Name,
	)
	response.OK(c, env)
}

// UpdateEnvironment 更新 Environment 設定。
// PUT /pipelines/:pipelineID/environments/:envID
func (h *EnvironmentHandler) UpdateEnvironment(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}
	envID, err := parseUintParam(c, "envID")
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}

	var req services.UpdateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	env, err := h.envSvc.UpdateEnvironment(c.Request.Context(), pipelineID, envID, &req)
	if err != nil {
		response.NotFound(c, "environment not found")
		return
	}

	logger.Info("environment updated via API",
		"pipeline_id", pipelineID,
		"env_id", envID,
	)
	response.OK(c, env)
}

// DeleteEnvironment 刪除 Environment。
// DELETE /pipelines/:pipelineID/environments/:envID
func (h *EnvironmentHandler) DeleteEnvironment(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}
	envID, err := parseUintParam(c, "envID")
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}

	if err := h.envSvc.DeleteEnvironment(c.Request.Context(), pipelineID, envID); err != nil {
		response.NotFound(c, "environment not found")
		return
	}

	logger.Info("environment deleted via API",
		"pipeline_id", pipelineID,
		"env_id", envID,
	)
	response.OK(c, gin.H{"message": "environment deleted"})
}
