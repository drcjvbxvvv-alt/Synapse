package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
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
}

// NewPipelineHandler 建立 PipelineHandler。
func NewPipelineHandler(pipelineSvc *services.PipelineService) *PipelineHandler {
	return &PipelineHandler{pipelineSvc: pipelineSvc}
}

// ─── Pipeline CRUD ─────────────────────────────────────────────────────────

// CreatePipeline 建立 Pipeline。
// POST /clusters/:clusterID/pipelines
func (h *PipelineHandler) CreatePipeline(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req services.CreatePipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	req.ClusterID = clusterID

	userID := c.GetUint("user_id")
	pipeline, err := h.pipelineSvc.CreatePipeline(c.Request.Context(), &req, userID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to create pipeline: "+err.Error())
		return
	}

	response.Created(c, pipeline)
}

// GetPipeline 取得單一 Pipeline。
// GET /clusters/:clusterID/pipelines/:pipelineID
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
// GET /clusters/:clusterID/pipelines
func (h *PipelineHandler) ListPipelines(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	params := &services.ListPipelinesParams{
		ClusterID: clusterID,
		Namespace: c.Query("namespace"),
		Search:    c.Query("search"),
		Page:      parsePage(c),
		PageSize:  parsePageSize(c, 20),
	}

	pipelines, total, err := h.pipelineSvc.ListPipelines(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to list pipelines: "+err.Error())
		return
	}

	response.List(c, pipelines, total)
}

// UpdatePipeline 更新 Pipeline。
// PUT /clusters/:clusterID/pipelines/:pipelineID
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
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to update pipeline: "+err.Error())
		return
	}

	response.OK(c, pipeline)
}

// DeletePipeline 刪除 Pipeline。
// DELETE /clusters/:clusterID/pipelines/:pipelineID
func (h *PipelineHandler) DeletePipeline(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	if err := h.pipelineSvc.DeletePipeline(c.Request.Context(), pipelineID); err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to delete pipeline: "+err.Error())
		return
	}

	logger.Info("pipeline deleted via API", "pipeline_id", pipelineID, "user_id", c.GetUint("user_id"))
	response.OK(c, gin.H{"message": "deleted"})
}

// ─── Version 快照 ──────────────────────────────────────────────────────────

// CreateVersion 建立不可變版本快照。
// POST /clusters/:clusterID/pipelines/:pipelineID/versions
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
// GET /clusters/:clusterID/pipelines/:pipelineID/versions/:version
func (h *PipelineHandler) GetVersion(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	versionNum := parseIntQuery(c, "version", 0)
	if vn, err2 := parseUintParam(c, "version"); err2 == nil {
		versionNum = int(vn)
	}
	if versionNum <= 0 {
		response.BadRequest(c, "invalid version number")
		return
	}

	version, err := h.pipelineSvc.GetVersion(c.Request.Context(), pipelineID, versionNum)
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
// GET /clusters/:clusterID/pipelines/:pipelineID/versions
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

// ─── Helpers ───────────────────────────────────────────────────────────────

func parseUintParam(c *gin.Context, key string) (uint, error) {
	return parseClusterID(c.Param(key)) // reuse same logic
}
