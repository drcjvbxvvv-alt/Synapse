package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineLogHandler — SSE Log 串流 + 歷史查詢
//
// 設計（CICD_ARCHITECTURE §7.11, §8.3）：
//   - follow=true  → SSE 串流（輪詢 DB 新 chunk，直到 step 完成）
//   - follow=false → 回傳完整歷史 log
// ---------------------------------------------------------------------------

// PipelineLogHandler 處理 Pipeline Step Log 的查詢與串流。
type PipelineLogHandler struct {
	logSvc      *services.PipelineLogService
	pipelineSvc *services.PipelineService
}

// NewPipelineLogHandler 建立 Log handler。
func NewPipelineLogHandler(
	logSvc *services.PipelineLogService,
	pipelineSvc *services.PipelineService,
) *PipelineLogHandler {
	return &PipelineLogHandler{
		logSvc:      logSvc,
		pipelineSvc: pipelineSvc,
	}
}

// GetStepLogs 查詢 Step 執行 Log。
// GET /clusters/:clusterID/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/logs
//
// Query params:
//   - follow=true|false (default: false)
func (h *PipelineLogHandler) GetStepLogs(c *gin.Context) {
	stepRunID, err := parseUintParam(c, "stepRunID")
	if err != nil {
		response.BadRequest(c, "invalid stepRunID")
		return
	}

	follow := c.DefaultQuery("follow", "false") == "true"

	if follow {
		h.streamLogsSSE(c, stepRunID)
		return
	}

	// 歷史查詢模式：回傳全部 log
	ctx := c.Request.Context()
	content, err := h.logSvc.GetLogContent(ctx, stepRunID)
	if err != nil {
		logger.Error("failed to get step logs", "step_run_id", stepRunID, "error", err)
		response.InternalError(c, "failed to get logs: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"step_run_id": stepRunID,
		"content":     content,
	})
}

// streamLogsSSE 透過 SSE 串流 Log 到客戶端。
// 輪詢 DB 直到 step 完成且無新 chunk。
func (h *PipelineLogHandler) streamLogsSSE(c *gin.Context, stepRunID uint) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // nginx proxy buffering 關閉

	ctx := c.Request.Context()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "streaming not supported")
		return
	}

	lastSeq := -1
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logs, err := h.logSvc.GetLogsSince(ctx, stepRunID, lastSeq)
			if err != nil {
				writeSSEEvent(c.Writer, "error", fmt.Sprintf(`{"error":"%s"}`, err.Error()))
				flusher.Flush()
				return
			}

			for _, log := range logs {
				writeSSEEvent(c.Writer, "log", log.Content)
				if log.ChunkSeq > lastSeq {
					lastSeq = log.ChunkSeq
				}
			}

			if len(logs) > 0 {
				flusher.Flush()
			}

			// 檢查 step 是否已完成
			if h.isStepFinished(ctx, stepRunID) && len(logs) == 0 {
				writeSSEEvent(c.Writer, "done", `{"status":"completed"}`)
				flusher.Flush()
				return
			}
		}
	}
}

// isStepFinished 檢查 StepRun 是否已結束（非 pending/running）。
func (h *PipelineLogHandler) isStepFinished(ctx context.Context, stepRunID uint) bool {
	stepRun, err := h.pipelineSvc.GetStepRun(ctx, stepRunID)
	if err != nil {
		return true // 找不到視為完成
	}
	return stepRun.Status != "pending" && stepRun.Status != "running"
}

// writeSSEEvent 寫入一個 SSE 事件。
func writeSSEEvent(w io.Writer, event, data string) {
	// 分行處理多行 data
	lines := strings.Split(data, "\n")
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range lines {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n")
}

// parseUintParam 已在 pipeline_handler.go 定義，此處重用。
// 若將來需要避免 redeclare，可提取到 handler_helpers.go。

// parseStepRunID 解析 stepRunID 參數（自定義名稱，避免重複）。
func parseStepRunID(c *gin.Context) (uint, error) {
	idStr := c.Param("stepRunID")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
