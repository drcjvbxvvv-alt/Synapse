package handlers

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// HandleAggregateLogStream 處理聚合日誌流 WebSocket
func (h *LogCenterHandler) HandleAggregateLogStream(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 升級WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket升級失敗", "error", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 讀取客戶端配置訊息
	var config models.LogStreamConfig
	if err := conn.ReadJSON(&config); err != nil {
		_ = conn.WriteJSON(gin.H{"type": "error", "message": "無效的配置: " + err.Error()})
		return
	}

	if len(config.Targets) == 0 {
		_ = conn.WriteJSON(gin.H{"type": "error", "message": "至少需要一個日誌目標"})
		return
	}

	// 傳送連線成功訊息
	_ = conn.WriteJSON(gin.H{
		"type":    "connected",
		"message": fmt.Sprintf("已連線到 %d 個日誌源", len(config.Targets)),
	})

	// 啟動聚合日誌流
	opts := &models.LogStreamOptions{
		TailLines:     config.TailLines,
		SinceSeconds:  config.SinceSeconds,
		ShowTimestamp: config.ShowTimestamp,
	}

	logCh, err := h.aggregator.AggregateStream(ctx, cluster, config.Targets, opts)
	if err != nil {
		_ = conn.WriteJSON(gin.H{"type": "error", "message": err.Error()})
		return
	}

	// 監聽客戶端斷開
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// 傳送日誌開始訊息
	_ = conn.WriteJSON(gin.H{
		"type":    "start",
		"message": "開始接收日誌流",
	})

	// 轉發日誌
	for entry := range logCh {
		msg := gin.H{
			"type":      "log",
			"id":        entry.ID,
			"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
			"namespace": entry.Namespace,
			"pod_name":  entry.PodName,
			"container": entry.Container,
			"level":     entry.Level,
			"message":   entry.Message,
		}

		if err := conn.WriteJSON(msg); err != nil {
			logger.Info("傳送日誌失敗，客戶端可能已斷開", "error", err)
			return
		}
	}

	_ = conn.WriteJSON(gin.H{
		"type":    "end",
		"message": "日誌流已結束",
	})
}

// HandleSinglePodLogStream 處理單個Pod日誌流 WebSocket
func (h *LogCenterHandler) HandleSinglePodLogStream(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Param("namespace")
	podName := c.Param("name")
	container := c.Query("container")
	previous := c.Query("previous") == "true"
	tailLines, _ := strconv.ParseInt(c.DefaultQuery("tailLines", "100"), 10, 64)
	sinceSeconds, _ := strconv.ParseInt(c.DefaultQuery("sinceSeconds", "0"), 10, 64)

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 升級WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket升級失敗", "error", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// 傳送連線成功訊息
	_ = conn.WriteJSON(gin.H{
		"type":    "connected",
		"message": "WebSocket連線已建立",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		_ = conn.WriteJSON(gin.H{"type": "error", "message": "獲取K8s客戶端失敗: " + err.Error()})
		return
	}

	// 構建日誌選項
	podLogOpts := &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
		Previous:   previous,
	}

	if container != "" {
		podLogOpts.Container = container
	}

	if tailLines > 0 {
		podLogOpts.TailLines = &tailLines
	}

	if sinceSeconds > 0 {
		podLogOpts.SinceSeconds = &sinceSeconds
	}

	// 獲取日誌流
	stream, err := k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(podName, podLogOpts).Stream(ctx)
	if err != nil {
		_ = conn.WriteJSON(gin.H{"type": "error", "message": "獲取日誌流失敗: " + err.Error()})
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	// 監聽客戶端斷開
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				_ = stream.Close()
				return
			}
		}
	}()

	// 傳送日誌開始訊息
	_ = conn.WriteJSON(gin.H{
		"type":    "start",
		"message": "開始接收日誌流",
	})

	// 讀取並轉發日誌
	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteJSON(gin.H{"type": "closed", "message": "日誌流已關閉"})
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "canceled") {
					return
				}
				_ = conn.WriteJSON(gin.H{"type": "end", "message": "日誌流已結束"})
				return
			}

			// 解析日誌級別
			level := "info"
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "fail") {
				level = "error"
			} else if strings.Contains(lowerLine, "warn") {
				level = "warn"
			}

			msg := gin.H{
				"type":      "log",
				"id":        uuid.New().String(),
				"timestamp": time.Now().Format(time.RFC3339Nano),
				"namespace": namespace,
				"pod":       podName,
				"container": container,
				"level":     level,
				"message":   strings.TrimSpace(line),
			}

			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}
