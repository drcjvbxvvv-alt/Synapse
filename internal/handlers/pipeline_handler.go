package handlers

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/constants"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineHandler — Pipeline + Version CRUD
// ---------------------------------------------------------------------------

// PipelineHandler 管理 Pipeline 與版本快照的 HTTP 端點。
type PipelineHandler struct {
	pipelineSvc *services.PipelineService
	auditSvc    *services.AuditService
}

// NewPipelineHandler 建立 PipelineHandler。
func NewPipelineHandler(pipelineSvc *services.PipelineService, auditSvc *services.AuditService) *PipelineHandler {
	return &PipelineHandler{pipelineSvc: pipelineSvc, auditSvc: auditSvc}
}

// logPipelineAudit 非同步寫入 hash-chain audit log（不阻塞 HTTP 回應）。
func (h *PipelineHandler) logPipelineAudit(c *gin.Context, action, resourceType, resourceRef, result string) {
	if h.auditSvc == nil {
		return
	}
	userID := c.GetUint("user_id")
	req := services.LogAuditRequest{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceRef:  resourceRef,
		Result:       result,
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
	}
	go func() {
		if err := h.auditSvc.LogAudit(context.Background(), req); err != nil {
			logger.Warn("pipeline audit log failed", "error", err, "action", action)
		}
	}()
}

// ─── Pipeline CRUD ─────────────────────────────────────────────────────────

// CreatePipeline 建立 Pipeline。
// POST /pipelines
func (h *PipelineHandler) CreatePipeline(c *gin.Context) {
	var req services.CreatePipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	userID := c.GetUint("user_id")
	pipeline, err := h.pipelineSvc.CreatePipeline(c.Request.Context(), &req, userID)
	if err != nil {
		h.logPipelineAudit(c, constants.ActionCreate, "pipeline", req.Name, "failed")
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to create pipeline: "+err.Error())
		return
	}

	h.logPipelineAudit(c, constants.ActionCreate, "pipeline", fmt.Sprintf("%d/%s", pipeline.ID, pipeline.Name), "success")
	response.Created(c, pipeline)
}

// GetPipeline 取得單一 Pipeline。
// GET /pipelines/:pipelineID
func (h *PipelineHandler) GetPipeline(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	pipeline, err := h.pipelineSvc.GetPipeline(c.Request.Context(), pipelineID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get pipeline: "+err.Error())
		return
	}

	response.OK(c, pipeline)
}

// ListPipelines 列出 Pipeline。
// GET /pipelines
func (h *PipelineHandler) ListPipelines(c *gin.Context) {
	params := &services.ListPipelinesParams{
		Search:   c.Query("search"),
		Page:     parsePage(c),
		PageSize: parsePageSize(c, 20),
	}

	pipelines, total, err := h.pipelineSvc.ListPipelines(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to list pipelines: "+err.Error())
		return
	}

	response.List(c, pipelines, total)
}

// UpdatePipeline 更新 Pipeline。
// PUT /pipelines/:pipelineID
func (h *PipelineHandler) UpdatePipeline(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var req services.UpdatePipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	pipeline, err := h.pipelineSvc.UpdatePipeline(c.Request.Context(), pipelineID, &req)
	if err != nil {
		h.logPipelineAudit(c, constants.ActionUpdate, "pipeline", fmt.Sprintf("%d", pipelineID), "failed")
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to update pipeline: "+err.Error())
		return
	}

	h.logPipelineAudit(c, constants.ActionUpdate, "pipeline", fmt.Sprintf("%d/%s", pipeline.ID, pipeline.Name), "success")
	response.OK(c, pipeline)
}

// DeletePipeline 刪除 Pipeline。
// DELETE /pipelines/:pipelineID
func (h *PipelineHandler) DeletePipeline(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	if err := h.pipelineSvc.DeletePipeline(c.Request.Context(), pipelineID); err != nil {
		h.logPipelineAudit(c, constants.ActionDelete, "pipeline", fmt.Sprintf("%d", pipelineID), "failed")
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to delete pipeline: "+err.Error())
		return
	}

	logger.Info("pipeline deleted via API", "pipeline_id", pipelineID, "user_id", c.GetUint("user_id"))
	h.logPipelineAudit(c, constants.ActionDelete, "pipeline", fmt.Sprintf("%d", pipelineID), "success")
	response.OK(c, gin.H{"message": "deleted"})
}

// ─── Version 快照 ──────────────────────────────────────────────────────────

// CreateVersion 建立不可變版本快照。
// POST /pipelines/:pipelineID/versions
func (h *PipelineHandler) CreateVersion(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var req services.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	userID := c.GetUint("user_id")
	version, err := h.pipelineSvc.CreateVersion(c.Request.Context(), pipelineID, &req, userID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to create version: "+err.Error())
		return
	}

	response.Created(c, version)
}

// GetVersion 取得指定版本。
// GET /pipelines/:pipelineID/versions/:version
func (h *PipelineHandler) GetVersion(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	vn, err := parseUintParam(c, "version")
	if err != nil || vn == 0 {
		response.BadRequest(c, "invalid version number")
		return
	}

	version, err := h.pipelineSvc.GetVersion(c.Request.Context(), pipelineID, int(vn))
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get version: "+err.Error())
		return
	}

	response.OK(c, version)
}

// ListVersions 列出 Pipeline 所有版本。
// GET /pipelines/:pipelineID/versions
func (h *PipelineHandler) ListVersions(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	versions, err := h.pipelineSvc.ListVersions(c.Request.Context(), pipelineID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to list versions: "+err.Error())
		return
	}

	response.OK(c, versions)
}

// ─── AllowedImages ─────────────────────────────────────────────────────────

// GetAllowedImages returns the global Step image whitelist.
// GET /api/v1/system/pipeline/allowed-images
func (h *PipelineHandler) GetAllowedImages(c *gin.Context) {
	patterns, err := h.pipelineSvc.GetAllowedImages(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to get allowed images: "+err.Error())
		return
	}
	response.OK(c, gin.H{"patterns": patterns})
}

// UpdateAllowedImagesRequest is the PUT body for allowed images.
type UpdateAllowedImagesRequest struct {
	Patterns []string `json:"patterns" binding:"required"`
}

// UpdateAllowedImages overwrites the global Step image whitelist.
// PUT /api/v1/system/pipeline/allowed-images
func (h *PipelineHandler) UpdateAllowedImages(c *gin.Context) {
	var req UpdateAllowedImagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	if err := h.pipelineSvc.UpdateAllowedImages(c.Request.Context(), req.Patterns); err != nil {
		response.InternalError(c, "failed to update allowed images: "+err.Error())
		return
	}
	logger.Info("pipeline allowed images updated", "count", len(req.Patterns))
	response.OK(c, gin.H{"patterns": req.Patterns})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func parseUintParam(c *gin.Context, key string) (uint, error) {
	return parseClusterID(c.Param(key)) // reuse same logic
}
