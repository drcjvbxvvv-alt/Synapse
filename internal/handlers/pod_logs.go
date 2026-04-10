package handlers

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
)

// GetPodLogs 獲取Pod日誌
func (h *PodHandler) GetPodLogs(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	container := c.Query("container")
	follow := c.Query("follow") == "true"
	previous := c.Query("previous") == "true"
	tailLines := c.Query("tailLines")
	sinceSeconds := c.Query("sinceSeconds")

	logger.Info("獲取Pod日誌: %s/%s/%s, container=%s", clusterId, namespace, name, container)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logOptions := &corev1.PodLogOptions{
		Follow:   follow,
		Previous: previous,
	}

	if container != "" {
		logOptions.Container = container
	}

	if tailLines != "" {
		if lines, err := strconv.ParseInt(tailLines, 10, 64); err == nil {
			logOptions.TailLines = &lines
		}
	}

	if sinceSeconds != "" {
		if seconds, err := strconv.ParseInt(sinceSeconds, 10, 64); err == nil {
			logOptions.SinceSeconds = &seconds
		}
	}

	req := k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(name, logOptions)
	logs, err := req.Stream(ctx)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "previous") || strings.Contains(errMsg, "not waiting to start") {
			response.BadRequest(c, "沒有上一個容器的日誌（Pod 尚未重啟過）")
			return
		}
		response.InternalError(c, "獲取日誌失敗: "+errMsg)
		return
	}
	defer func() {
		_ = logs.Close()
	}()

	if follow {
		response.BadRequest(c, "流式日誌請使用WebSocket連線: /ws/clusters/:clusterID/pods/:namespace/:name/logs")
		return
	}

	buf := make([]byte, 4096)
	var logContent string
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			logContent += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	response.OK(c, gin.H{
		"logs": logContent,
	})
}

// StreamPodLogs WebSocket流式傳輸Pod日誌
func (h *PodHandler) StreamPodLogs(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	container := c.Query("container")
	previous := c.Query("previous") == "true"
	tailLines := c.Query("tailLines")
	sinceSeconds := c.Query("sinceSeconds")

	logger.Info("WebSocket流式獲取Pod日誌: %s/%s/%s, container=%s", clusterId, namespace, name, container)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("升級WebSocket連線失敗", "error", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	err = conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"message": "WebSocket連線已建立",
	})
	if err != nil {
		logger.Error("傳送連線訊息失敗", "error", err)
		return
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		_ = conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"message": "獲取K8s客戶端失敗: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logOptions := &corev1.PodLogOptions{
		Follow:   true,
		Previous: previous,
	}

	if container != "" {
		logOptions.Container = container
	}

	if tailLines != "" {
		if lines, err := strconv.ParseInt(tailLines, 10, 64); err == nil {
			logOptions.TailLines = &lines
		}
	}

	if sinceSeconds != "" {
		if seconds, err := strconv.ParseInt(sinceSeconds, 10, 64); err == nil {
			logOptions.SinceSeconds = &seconds
		}
	}

	req := k8sClient.GetClientset().CoreV1().Pods(namespace).GetLogs(name, logOptions)
	logStream, err := req.Stream(context.Background())
	if err != nil {
		_ = conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"message": "獲取日誌流失敗: " + err.Error(),
		})
		return
	}
	defer func() {
		_ = logStream.Close()
	}()

	reader := bufio.NewReader(logStream)

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				logger.Info("WebSocket連線關閉", "error", err)
				cancel()
				_ = logStream.Close()
				return
			}
		}
	}()

	err = conn.WriteJSON(map[string]interface{}{
		"type":    "start",
		"message": "開始接收日誌流",
	})
	if err != nil {
		logger.Error("傳送開始訊息失敗", "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteJSON(map[string]interface{}{
				"type":    "closed",
				"message": "日誌流已關閉",
			})
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "end",
						"message": "日誌流已結束",
					})
					return
				}

				errStr := err.Error()
				if strings.Contains(errStr, "closed") ||
					strings.Contains(errStr, "canceled") ||
					strings.Contains(errStr, "cancel") {
					logger.Info("日誌流停止: 連線已關閉或取消")
					return
				}

				if ctx.Err() != nil {
					logger.Info("日誌流停止: context取消")
					return
				}

				logger.Error("讀取日誌失敗", "error", err)
				_ = conn.WriteJSON(map[string]interface{}{
					"type":    "error",
					"message": "讀取日誌失敗: " + err.Error(),
				})
				return
			}

			err = conn.WriteJSON(map[string]interface{}{
				"type": "log",
				"data": line,
			})
			if err != nil {
				logger.Info("傳送日誌失敗，客戶端可能已斷開", "error", err)
				return
			}
		}
	}
}
