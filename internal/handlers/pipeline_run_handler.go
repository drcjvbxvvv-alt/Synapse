package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/constants"
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
	auditSvc    *services.AuditService
}

// NewPipelineRunHandler 建立 PipelineRunHandler。
func NewPipelineRunHandler(
	pipelineSvc *services.PipelineService,
	scheduler *services.PipelineScheduler,
	auditSvc *services.AuditService,
) *PipelineRunHandler {
	return &PipelineRunHandler{
		pipelineSvc: pipelineSvc,
		scheduler:   scheduler,
		auditSvc:    auditSvc,
	}
}

// logRunAudit 非同步寫入 Pipeline Run 操作的 hash-chain audit log。
func (h *PipelineRunHandler) logRunAudit(c *gin.Context, action, resourceRef, result string) {
	if h.auditSvc == nil {
		return
	}
	req := services.LogAuditRequest{
		UserID:       c.GetUint("user_id"),
		Action:       action,
		ResourceType: "pipeline_run",
		ResourceRef:  resourceRef,
		Result:       result,
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
	}
	go func() {
		if err := h.auditSvc.LogAudit(context.Background(), req); err != nil {
			logger.Warn("pipeline run audit log failed", "error", err, "action", action)
		}
	}()
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
		h.logRunAudit(c, constants.ActionTrigger, fmt.Sprintf("pipeline:%d", pipelineID), "failed")
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
	h.logRunAudit(c, constants.ActionTrigger, fmt.Sprintf("pipeline:%d/run:%d", pipelineID, run.ID), "success")

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
		h.logRunAudit(c, constants.ActionCancel, fmt.Sprintf("run:%d", runID), "failed")
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
	h.logRunAudit(c, constants.ActionCancel, fmt.Sprintf("run:%d", runID), "success")

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

// RerunRequest rerun 的請求 body。
type RerunRequest struct {
	FromFailed bool `json:"from_failed"` // true = 從第一個失敗 Step 開始重跑，跳過已成功的
}

// RerunPipeline 從失敗的 Run 重跑（建立新 Run，繼承 snapshot_id）。
// 若 from_failed=true，跳過原始 Run 中已成功的 Steps。
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

	var req RerunRequest
	_ = c.ShouldBindJSON(&req) // body optional

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

	// from_failed: 找出第一個失敗的 Step 名稱
	var rerunFromStep string
	if req.FromFailed {
		if originalRun.Status != models.PipelineRunStatusFailed {
			response.BadRequest(c, "from_failed only works on failed runs")
			return
		}
		rerunFromStep, err = h.findFirstFailedStep(c.Request.Context(), runID)
		if err != nil {
			response.InternalError(c, "failed to find failed step: "+err.Error())
			return
		}
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
		RerunFromStep:    rerunFromStep,
	}

	if err := h.scheduler.EnqueueRun(c.Request.Context(), newRun); err != nil {
		h.logRunAudit(c, constants.ActionRerun, fmt.Sprintf("pipeline:%d/from-run:%d", pipelineID, runID), "failed")
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
		"from_failed", req.FromFailed,
		"rerun_from_step", rerunFromStep,
	)
	h.logRunAudit(c, constants.ActionRerun, fmt.Sprintf("pipeline:%d/from-run:%d/new-run:%d", pipelineID, runID, newRun.ID), "success")

	c.JSON(http.StatusAccepted, gin.H{
		"run_id":          newRun.ID,
		"rerun_from_id":   originalRun.ID,
		"rerun_from_step": rerunFromStep,
		"status":          newRun.Status,
		"message":         "pipeline rerun triggered",
	})
}

// findFirstFailedStep 找出 Run 中第一個失敗的 Step 名稱（按 step_index 排序）。
func (h *PipelineRunHandler) findFirstFailedStep(ctx context.Context, runID uint) (string, error) {
	var sr models.StepRun
	if err := h.pipelineSvc.DB().WithContext(ctx).
		Where("pipeline_run_id = ? AND status = ?", runID, models.StepRunStatusFailed).
		Order("step_index ASC").
		First(&sr).Error; err != nil {
		return "", fmt.Errorf("find first failed step in run %d: %w", runID, err)
	}
	return sr.StepName, nil
}

// ApproveStep 批准 Approval Step。
// POST /clusters/:clusterID/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/approve
func (h *PipelineRunHandler) ApproveStep(c *gin.Context) {
	stepRunID, err := parseUintParam(c, "stepRunID")
	if err != nil {
		response.BadRequest(c, "invalid step run ID")
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	if err := h.pipelineSvc.ApproveStepRun(c.Request.Context(), stepRunID, username); err != nil {
		response.InternalError(c, "failed to approve step: "+err.Error())
		return
	}

	logger.Info("approval step approved via API",
		"step_run_id", stepRunID,
		"approved_by", username,
	)
	response.OK(c, gin.H{"message": "step approved"})
}

// RejectStep 拒絕 Approval Step。
// POST /clusters/:clusterID/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/reject
func (h *PipelineRunHandler) RejectStep(c *gin.Context) {
	stepRunID, err := parseUintParam(c, "stepRunID")
	if err != nil {
		response.BadRequest(c, "invalid step run ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req) // reason is optional

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	if err := h.pipelineSvc.RejectStepRun(c.Request.Context(), stepRunID, username, req.Reason); err != nil {
		response.InternalError(c, "failed to reject step: "+err.Error())
		return
	}

	logger.Info("approval step rejected via API",
		"step_run_id", stepRunID,
		"rejected_by", username,
	)
	response.OK(c, gin.H{"message": "step rejected"})
}

// ListStepTypes 列出所有支援的 Step 類型。
// GET /pipeline-step-types
func (h *PipelineRunHandler) ListStepTypes(c *gin.Context) {
	types := services.ListStepTypes()
	response.OK(c, types)
}
