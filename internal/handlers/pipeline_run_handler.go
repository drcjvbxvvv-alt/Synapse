package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineRunHandler — Pipeline Run 觸發 / 取消 / 查詢
// ---------------------------------------------------------------------------

// PipelineRunHandler 管理 Pipeline Run 的 HTTP 端點。
type PipelineRunHandler struct {
	pipelineSvc *services.PipelineService
	scheduler   *services.PipelineScheduler
}

// NewPipelineRunHandler 建立 PipelineRunHandler。
func NewPipelineRunHandler(
	pipelineSvc *services.PipelineService,
	scheduler *services.PipelineScheduler,
) *PipelineRunHandler {
	return &PipelineRunHandler{
		pipelineSvc: pipelineSvc,
		scheduler:   scheduler,
	}
}

// TriggerRunRequest 手動觸發 Pipeline Run 的請求。
type TriggerRunRequest struct {
	VersionID *uint `json:"version_id"` // 指定版本（可選，預設用 current_version_id）
}

// TriggerRun 手動觸發 Pipeline Run。
// POST /clusters/:clusterID/pipelines/:pipelineID/runs
func (h *PipelineRunHandler) TriggerRun(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var req TriggerRunRequest
	// body 是可選的，空 body 也合法
	_ = c.ShouldBindJSON(&req)

	pipeline, err := h.pipelineSvc.GetPipeline(c.Request.Context(), pipelineID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get pipeline: "+err.Error())
		return
	}

	// 確認有可用版本
	snapshotID := pipeline.CurrentVersionID
	if req.VersionID != nil {
		snapshotID = req.VersionID
	}
	if snapshotID == nil {
		response.BadRequest(c, "pipeline has no active version; create a version first")
		return
	}

	userID := c.GetUint("user_id")

	run := &models.PipelineRun{
		PipelineID:       pipeline.ID,
		SnapshotID:       *snapshotID,
		ClusterID:        pipeline.ClusterID,
		Namespace:        pipeline.Namespace,
		TriggerType:      models.TriggerTypeManual,
		TriggeredByUser:  userID,
		ConcurrencyGroup: pipeline.ConcurrencyGroup,
	}

	if err := h.scheduler.EnqueueRun(c.Request.Context(), run); err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to trigger pipeline: "+err.Error())
		return
	}

	logger.Info("pipeline triggered manually",
		"pipeline_id", pipelineID,
		"run_id", run.ID,
		"user_id", userID,
		"snapshot_id", *snapshotID,
	)

	c.JSON(http.StatusAccepted, gin.H{
		"run_id":  run.ID,
		"status":  run.Status,
		"message": "pipeline triggered",
	})
}

// CancelRun 取消 Pipeline Run。
// POST /clusters/:clusterID/pipelines/:pipelineID/runs/:runID/cancel
func (h *PipelineRunHandler) CancelRun(c *gin.Context) {
	runID, err := parseUintParam(c, "runID")
	if err != nil {
		response.BadRequest(c, "invalid run ID")
		return
	}

	if err := h.scheduler.CancelRun(c.Request.Context(), runID); err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to cancel run: "+err.Error())
		return
	}

	logger.Info("pipeline run cancel requested",
		"run_id", runID,
		"user_id", c.GetUint("user_id"),
	)

	response.OK(c, gin.H{"message": "cancel requested", "run_id": runID})
}

// ListRuns 列出 Pipeline 的執行記錄。
// GET /clusters/:clusterID/pipelines/:pipelineID/runs
func (h *PipelineRunHandler) ListRuns(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	params := &services.ListPipelineRunsParams{
		PipelineID: pipelineID,
		Status:     c.Query("status"),
		Page:       parsePage(c),
		PageSize:   parsePageSize(c, 20),
	}

	runs, total, err := h.pipelineSvc.ListPipelineRuns(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to list runs: "+err.Error())
		return
	}

	response.List(c, runs, total)
}

// GetRun 取得 Pipeline Run 詳情（含 StepRun 清單）。
// GET /clusters/:clusterID/pipelines/:pipelineID/runs/:runID
func (h *PipelineRunHandler) GetRun(c *gin.Context) {
	runID, err := parseUintParam(c, "runID")
	if err != nil {
		response.BadRequest(c, "invalid run ID")
		return
	}

	run, err := h.pipelineSvc.GetPipelineRun(c.Request.Context(), runID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get run: "+err.Error())
		return
	}

	steps, err := h.pipelineSvc.ListStepRuns(c.Request.Context(), runID)
	if err != nil {
		response.InternalError(c, "failed to list step runs: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"run":   run,
		"steps": steps,
	})
}

// RerunPipeline 從失敗的 Run 重跑（建立新 Run，繼承 snapshot_id）。
// POST /clusters/:clusterID/pipelines/:pipelineID/runs/:runID/rerun
func (h *PipelineRunHandler) RerunPipeline(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	runID, err := parseUintParam(c, "runID")
	if err != nil {
		response.BadRequest(c, "invalid run ID")
		return
	}

	// 取得原始 Run
	originalRun, err := h.pipelineSvc.GetPipelineRun(c.Request.Context(), runID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get original run: "+err.Error())
		return
	}

	// 確認是同一個 Pipeline
	if originalRun.PipelineID != pipelineID {
		response.BadRequest(c, "run does not belong to this pipeline")
		return
	}

	userID := c.GetUint("user_id")

	newRun := &models.PipelineRun{
		PipelineID:       originalRun.PipelineID,
		SnapshotID:       originalRun.SnapshotID,
		ClusterID:        originalRun.ClusterID,
		Namespace:        originalRun.Namespace,
		TriggerType:      models.TriggerTypeRerun,
		TriggeredByUser:  userID,
		ConcurrencyGroup: originalRun.ConcurrencyGroup,
		RerunFromID:      &originalRun.ID,
	}

	if err := h.scheduler.EnqueueRun(c.Request.Context(), newRun); err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to rerun pipeline: "+err.Error())
		return
	}

	logger.Info("pipeline rerun triggered",
		"pipeline_id", pipelineID,
		"original_run_id", runID,
		"new_run_id", newRun.ID,
		"user_id", userID,
	)

	c.JSON(http.StatusAccepted, gin.H{
		"run_id":          newRun.ID,
		"rerun_from_id":   originalRun.ID,
		"status":          newRun.Status,
		"message":         "pipeline rerun triggered",
	})
}

// ListStepTypes 列出所有支援的 Step 類型。
// GET /pipeline-step-types
func (h *PipelineRunHandler) ListStepTypes(c *gin.Context) {
	types := services.ListStepTypes()
	response.OK(c, types)
}
