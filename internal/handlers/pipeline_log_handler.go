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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/models"
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
	k8sMgr      services.JobsListerProvider
}

// NewPipelineLogHandler 建立 Log handler。
func NewPipelineLogHandler(
	logSvc *services.PipelineLogService,
	pipelineSvc *services.PipelineService,
	k8sMgr services.JobsListerProvider,
) *PipelineLogHandler {
	return &PipelineLogHandler{
		logSvc:      logSvc,
		pipelineSvc: pipelineSvc,
		k8sMgr:      k8sMgr,
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
// 策略：step 運行中 → 直接串流 K8s Pod logs；step 已完成 → 從 DB 讀取歷史。
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

	// 取得 StepRun 資訊
	sr, err := h.pipelineSvc.GetStepRun(ctx, stepRunID)
	if err != nil {
		writeSSEEvent(c.Writer, "error", fmt.Sprintf(`{"error":"step run not found: %s"}`, err.Error()))
		flusher.Flush()
		return
	}

	// Step 還在運行中且有 Job → 直接串流 K8s Pod logs
	if (sr.Status == "running" || sr.Status == "pending") && sr.JobName != "" && h.k8sMgr != nil {
		h.streamFromK8s(c, ctx, sr, flusher)
		return
	}

	// Step 已完成 → 從 DB 讀取歷史 log
	h.streamFromDB(c, ctx, stepRunID, flusher)
}

// streamFromK8s 直接從 K8s Pod 串流即時日誌。
func (h *PipelineLogHandler) streamFromK8s(c *gin.Context, ctx context.Context, sr *models.StepRun, flusher http.Flusher) {
	// 查詢 PipelineRun 取得 ClusterID
	clusterID, err := h.pipelineSvc.GetRunClusterID(ctx, sr.PipelineRunID)
	if err != nil {
		writeSSEEvent(c.Writer, "error", fmt.Sprintf(`{"error":"run not found: %s"}`, err.Error()))
		flusher.Flush()
		return
	}

	k8sClient := h.k8sMgr.GetK8sClientByID(clusterID)
	if k8sClient == nil {
		writeSSEEvent(c.Writer, "error", `{"error":"cluster client unavailable"}`)
		flusher.Flush()
		return
	}

	// 串流 Pod logs（follow=true）
	follow := true
	logOpts := &corev1.PodLogOptions{
		Container: "step",
		Follow:    follow,
	}

	// 透過 Job 找到 Pod
	pods, err := k8sClient.GetClientset().CoreV1().
		Pods(sr.JobNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", sr.JobName),
		})
	if err != nil || len(pods.Items) == 0 {
		// Pod 還沒建立，等待後重試
		h.waitForPodAndStream(c, ctx, sr, k8sClient, flusher)
		return
	}

	podName := pods.Items[0].Name
	stream, err := k8sClient.GetClientset().CoreV1().
		Pods(sr.JobNamespace).
		GetLogs(podName, logOpts).Stream(ctx)
	if err != nil {
		logger.Warn("failed to stream pod logs, falling back to DB",
			"pod", podName, "error", err)
		h.streamFromDB(c, ctx, sr.ID, flusher)
		return
	}
	defer stream.Close()

	buf := make([]byte, 4096)
	for {
		n, readErr := stream.Read(buf)
		if n > 0 {
			lines := strings.Split(strings.TrimRight(string(buf[:n]), "\n"), "\n")
			for _, line := range lines {
				if line != "" {
					writeSSEEvent(c.Writer, "log", line)
				}
			}
			flusher.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF {
				logger.Warn("pod log stream error", "error", readErr)
			}
			break
		}
	}

	writeSSEEvent(c.Writer, "done", `{"status":"completed"}`)
	flusher.Flush()
}

// waitForPodAndStream 等待 Pod 建立後開始串流。
func (h *PipelineLogHandler) waitForPodAndStream(c *gin.Context, ctx context.Context, sr *models.StepRun, k8sClient *services.K8sClient, flusher http.Flusher) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 30; i++ { // 最多等 60 秒
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := k8sClient.GetClientset().CoreV1().
				Pods(sr.JobNamespace).
				List(ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("job-name=%s", sr.JobName),
				})
			if err == nil && len(pods.Items) > 0 {
				// Pod 就緒，開始串流
				podName := pods.Items[0].Name
				follow := true
				stream, err := k8sClient.GetClientset().CoreV1().
					Pods(sr.JobNamespace).
					GetLogs(podName, &corev1.PodLogOptions{
						Container: "step",
						Follow:    follow,
					}).Stream(ctx)
				if err != nil {
					continue // Pod 可能還沒 ready
				}
				defer stream.Close()

				buf := make([]byte, 4096)
				for {
					n, readErr := stream.Read(buf)
					if n > 0 {
						lines := strings.Split(strings.TrimRight(string(buf[:n]), "\n"), "\n")
						for _, line := range lines {
							if line != "" {
								writeSSEEvent(c.Writer, "log", line)
							}
						}
						flusher.Flush()
					}
					if readErr != nil {
						break
					}
				}

				writeSSEEvent(c.Writer, "done", `{"status":"completed"}`)
				flusher.Flush()
				return
			}
		}
	}

	writeSSEEvent(c.Writer, "error", `{"error":"pod not ready within timeout"}`)
	flusher.Flush()
}

// streamFromDB 從 DB 輪詢歷史日誌（step 已完成時使用）。
func (h *PipelineLogHandler) streamFromDB(c *gin.Context, ctx context.Context, stepRunID uint, flusher http.Flusher) {
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
				// 日誌可能是多行，需逐行發送 SSE event
				for _, line := range strings.Split(log.Content, "\n") {
					if line != "" {
						writeSSEEvent(c.Writer, "log", line)
					}
				}
				if log.ChunkSeq > lastSeq {
					lastSeq = log.ChunkSeq
				}
			}

			if len(logs) > 0 {
				flusher.Flush()
			}

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
