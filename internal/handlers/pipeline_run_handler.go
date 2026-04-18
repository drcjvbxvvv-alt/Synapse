package handlers

import (
	"context"
	"fmt"
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

// ---------------------------------------------------------------------------
// PipelineRunHandler — Pipeline Run 觸發 / 取消 / 查詢
// ---------------------------------------------------------------------------

// PipelineRunHandler 管理 Pipeline Run 的 HTTP 端點。
type PipelineRunHandler struct {
	pipelineSvc *services.PipelineService
	scheduler   *services.PipelineScheduler
	envSvc      *services.EnvironmentService
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

// SetEnvironmentService 設定 EnvironmentService（支援 env-based trigger 和 promote）。
func (h *PipelineRunHandler) SetEnvironmentService(envSvc *services.EnvironmentService) {
	h.envSvc = envSvc
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
		// Use an independent context: request context is cancelled when the handler
		// returns (before this goroutine runs), so we bound it with its own timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := h.auditSvc.LogAudit(ctx, req); err != nil {
			logger.Warn("pipeline run audit log failed", "error", err, "action", action)
		}
	}()
}

// TriggerRunRequest 手動觸發 Pipeline Run 的請求。
// cluster_id 和 namespace 可選 — 若未提供，使用 Pipeline 的構建環境預設值。
type TriggerRunRequest struct {
	VersionID *uint  `json:"version_id"`  // 指定版本（可選，預設用 current_version_id）
	ClusterID *uint  `json:"cluster_id"`  // 目標叢集（可選，預設用 Pipeline.BuildClusterID）
	Namespace string `json:"namespace"`   // 目標 namespace（可選，預設用 Pipeline.BuildNamespace）
}

// TriggerRun 手動觸發 Pipeline Run。
// POST /pipelines/:pipelineID/runs (or legacy /clusters/:clusterID/pipelines/:pipelineID/runs)
func (h *PipelineRunHandler) TriggerRun(c *gin.Context) {
	pipelineID, err := parseUintParam(c, "pipelineID")
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var req TriggerRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
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

	// 確認有可用版本
	snapshotID := pipeline.CurrentVersionID
	if req.VersionID != nil {
		snapshotID = req.VersionID
	}
	if snapshotID == nil {
		response.BadRequest(c, "pipeline has no active version; create a version first")
		return
	}

	// 智慧填充：優先用請求值，否則用 Pipeline 構建環境預設值
	clusterID := uint(0)
	namespace := req.Namespace
	if req.ClusterID != nil && *req.ClusterID > 0 {
		clusterID = *req.ClusterID
	} else if pipeline.BuildClusterID != nil {
		clusterID = *pipeline.BuildClusterID
	}
	if namespace == "" {
		namespace = pipeline.BuildNamespace
	}
	if clusterID == 0 {
		response.BadRequest(c, "cluster_id is required: set build environment in pipeline settings or provide in request")
		return
	}
	if namespace == "" {
		response.BadRequest(c, "namespace is required: set build environment in pipeline settings or provide in request")
		return
	}

	userID := c.GetUint("user_id")

	run := &models.PipelineRun{
		PipelineID:       pipeline.ID,
		SnapshotID:       *snapshotID,
		ClusterID:        clusterID,
		Namespace:        namespace,
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

	// 附帶掃描結果摘要
	type scanSummary struct {
		Status   string `json:"status"`
		Critical int    `json:"critical"`
		High     int    `json:"high"`
		Medium   int    `json:"medium"`
		Low      int    `json:"low"`
	}
	type runWithScan struct {
		models.PipelineRun
		ScanResult *scanSummary `json:"scan_result,omitempty"`
	}

	results := make([]runWithScan, len(runs))
	for i, run := range runs {
		results[i] = runWithScan{PipelineRun: run}
		var scan models.ImageScanResult
		if err := h.pipelineSvc.DB().WithContext(c.Request.Context()).
			Where("pipeline_run_id = ?", run.ID).
			Order("id DESC").
			First(&scan).Error; err == nil {
			results[i].ScanResult = &scanSummary{
				Status:   scan.Status,
				Critical: scan.Critical,
				High:     scan.High,
				Medium:   scan.Medium,
				Low:      scan.Low,
			}
		}
	}

	response.List(c, results, total)
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

// RollbackRequest 回滾 Pipeline Run 的請求。
type RollbackRequest struct {
	// 可選：覆蓋回滾執行的目標叢集（空 = 沿用來源 Run 的叢集）
	ClusterID *uint  `json:"cluster_id,omitempty"`
	// 可選：覆蓋回滾執行的目標 namespace（空 = 沿用來源 Run 的 namespace）
	Namespace string `json:"namespace,omitempty"`
}

// RollbackRun 回滾到指定成功 Run 的版本。
// 跳過所有 build/scan 類型的 Step，僅重新執行 deploy 類型的 Step。
// POST /pipelines/:pipelineID/runs/:runID/rollback
func (h *PipelineRunHandler) RollbackRun(c *gin.Context) {
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

	var req RollbackRequest
	_ = c.ShouldBindJSON(&req) // body 為選填

	// 取得來源 Run
	sourceRun, err := h.pipelineSvc.GetPipelineRun(c.Request.Context(), runID)
	if err != nil {
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to get source run: "+err.Error())
		return
	}

	// 確認是同一個 Pipeline
	if sourceRun.PipelineID != pipelineID {
		response.BadRequest(c, "run does not belong to this pipeline")
		return
	}

	// 只允許回滾成功的 Run
	if sourceRun.Status != models.PipelineRunStatusSuccess {
		response.BadRequest(c, fmt.Sprintf("can only rollback successful runs (current status: %s)", sourceRun.Status))
		return
	}

	// 確定回滾目標叢集與 Namespace
	clusterID := sourceRun.ClusterID
	if req.ClusterID != nil {
		clusterID = *req.ClusterID
	}
	namespace := sourceRun.Namespace
	if req.Namespace != "" {
		namespace = req.Namespace
	}

	userID := c.GetUint("user_id")

	newRun := &models.PipelineRun{
		PipelineID:       sourceRun.PipelineID,
		SnapshotID:       sourceRun.SnapshotID, // 使用原始 Run 的不可變版本快照
		ClusterID:        clusterID,
		Namespace:        namespace,
		TriggerType:      models.TriggerTypeRollback,
		TriggeredByUser:  userID,
		ConcurrencyGroup: sourceRun.ConcurrencyGroup,
		RollbackOfRunID:  &sourceRun.ID,
	}

	if err := h.scheduler.EnqueueRun(c.Request.Context(), newRun); err != nil {
		h.logRunAudit(c, constants.ActionRollback, fmt.Sprintf("pipeline:%d/from-run:%d", pipelineID, runID), "failed")
		if ae, ok := apierrors.As(err); ok {
			c.JSON(ae.HTTPStatus, gin.H{"error": ae.Message})
			return
		}
		response.InternalError(c, "failed to rollback pipeline: "+err.Error())
		return
	}

	logger.Info("pipeline rollback triggered",
		"pipeline_id", pipelineID,
		"source_run_id", runID,
		"new_run_id", newRun.ID,
		"user_id", userID,
		"cluster_id", clusterID,
		"namespace", namespace,
	)
	h.logRunAudit(c, constants.ActionRollback, fmt.Sprintf("pipeline:%d/from-run:%d/new-run:%d", pipelineID, runID, newRun.ID), "success")

	c.JSON(http.StatusAccepted, gin.H{
		"run_id":           newRun.ID,
		"rollback_of_run":  sourceRun.ID,
		"snapshot_id":      newRun.SnapshotID,
		"status":           newRun.Status,
		"message":          "pipeline rollback triggered",
	})
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

// ---------------------------------------------------------------------------
// TriggerRunInEnvironment — env-based trigger（H2）
// ---------------------------------------------------------------------------

// TriggerRunInEnvironmentRequest env-based 手動觸發請求。
type TriggerRunInEnvironmentRequest struct {
	VersionID *uint `json:"version_id"` // 指定版本（可選，預設用 current_version_id）
}

// TriggerRunInEnvironment 以指定 Environment 為目標觸發 Pipeline Run。
// POST /pipelines/:pipelineID/environments/:envID/runs
func (h *PipelineRunHandler) TriggerRunInEnvironment(c *gin.Context) {
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

	if h.envSvc == nil {
		response.InternalError(c, "environment service not configured")
		return
	}

	var req TriggerRunInEnvironmentRequest
	_ = c.ShouldBindJSON(&req) // VersionID is optional

	userID := c.GetUint("user_id")

	run, err := h.scheduler.EnqueueRunInEnvironment(c.Request.Context(), h.envSvc, services.TriggerRunInput{
		PipelineID:  pipelineID,
		EnvID:       envID,
		VersionID:   req.VersionID,
		UserID:      userID,
		TriggerType: models.TriggerTypeManual,
	})
	if err != nil {
		h.logRunAudit(c, constants.ActionTrigger, fmt.Sprintf("pipeline:%d/env:%d", pipelineID, envID), "failed")
		response.InternalError(c, "failed to trigger run: "+err.Error())
		return
	}

	logger.Info("pipeline run triggered in environment",
		"pipeline_id", pipelineID,
		"env_id", envID,
		"run_id", run.ID,
		"user_id", userID,
	)
	h.logRunAudit(c, constants.ActionTrigger, fmt.Sprintf("pipeline:%d/env:%d/run:%d", pipelineID, envID, run.ID), "success")

	c.JSON(http.StatusAccepted, gin.H{
		"run_id":  run.ID,
		"status":  run.Status,
		"message": "pipeline triggered in environment",
	})
}

// ---------------------------------------------------------------------------
// PromoteRun — 促進到下一個 Environment（H3）
// ---------------------------------------------------------------------------

// PromoteRun 以相同 SnapshotID 在下一個 Environment 建立新 Run。
// POST /pipelines/:pipelineID/runs/:runID/promote
func (h *PipelineRunHandler) PromoteRun(c *gin.Context) {
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

	if h.envSvc == nil {
		response.InternalError(c, "environment service not configured")
		return
	}

	// 取得原始 Run
	origRun, err := h.pipelineSvc.GetPipelineRun(c.Request.Context(), runID)
	if err != nil {
		response.NotFound(c, "pipeline run not found")
		return
	}

	if origRun.PipelineID != pipelineID {
		response.NotFound(c, "run does not belong to this pipeline")
		return
	}

	if origRun.Status != models.PipelineRunStatusSuccess {
		response.BadRequest(c, "can only promote successful runs")
		return
	}

	// 查找原始 Run 所在 Environment（透過 ClusterID+Namespace 反查）
	// 取得 Pipeline 的所有 Environments，找到 ClusterID+Namespace 匹配者
	envs, err := h.envSvc.ListEnvironments(c.Request.Context(), pipelineID)
	if err != nil {
		response.InternalError(c, "failed to list environments: "+err.Error())
		return
	}

	var currentOrderIndex int
	found := false
	for _, env := range envs {
		if env.ClusterID == origRun.ClusterID && env.Namespace == origRun.Namespace {
			currentOrderIndex = env.OrderIndex
			found = true
			break
		}
	}
	if !found {
		response.BadRequest(c, "cannot determine promotion target: run environment not found in pipeline")
		return
	}

	// 取得下一個 Environment
	nextEnv, err := h.envSvc.GetNextEnvironment(c.Request.Context(), pipelineID, currentOrderIndex)
	if err != nil {
		response.InternalError(c, "failed to find next environment: "+err.Error())
		return
	}
	if nextEnv == nil {
		response.BadRequest(c, "no next environment: already at the last environment")
		return
	}

	// 建立新 Run（使用相同 SnapshotID）
	userID := c.GetUint("user_id")
	snapshotID := origRun.SnapshotID

	newRun, err := h.scheduler.EnqueueRunInEnvironment(c.Request.Context(), h.envSvc, services.TriggerRunInput{
		PipelineID:  pipelineID,
		EnvID:       nextEnv.ID,
		VersionID:   &snapshotID,
		UserID:      userID,
		TriggerType: models.TriggerTypeManual,
		Payload:     fmt.Sprintf("promoted_from_run:%d", runID),
	})
	if err != nil {
		response.InternalError(c, "failed to promote run: "+err.Error())
		return
	}

	logger.Info("pipeline run promoted to next environment",
		"original_run_id", runID,
		"new_run_id", newRun.ID,
		"pipeline_id", pipelineID,
		"next_env_id", nextEnv.ID,
		"next_env_name", nextEnv.Name,
	)

	response.OK(c, gin.H{
		"run_id":    newRun.ID,
		"env_id":    nextEnv.ID,
		"env_name":  nextEnv.Name,
		"status":    newRun.Status,
		"message":   "promoted to next environment",
	})
}

// GetScanResult 取得 Pipeline Run 的安全掃描結果。
// GET /pipelines/:pipelineID/runs/:runID/scan-result
func (h *PipelineRunHandler) GetScanResult(c *gin.Context) {
	runID, err := parseUintParam(c, "runID")
	if err != nil {
		response.BadRequest(c, "invalid run ID")
		return
	}

	var scan models.ImageScanResult
	if err := h.pipelineSvc.DB().WithContext(c.Request.Context()).
		Where("pipeline_run_id = ?", runID).
		Order("id DESC").
		First(&scan).Error; err != nil {
		response.NotFound(c, "no scan result found for this run")
		return
	}

	response.OK(c, scan)
}
