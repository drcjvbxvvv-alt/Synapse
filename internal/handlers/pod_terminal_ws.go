package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// HandlePodTerminal 處理Pod終端WebSocket連線
func (h *PodTerminalHandler) HandlePodTerminal(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	podName := c.Param("name")
	container := c.DefaultQuery("container", "")
	userID := c.GetUint("user_id") // 從JWT中獲取使用者ID

	// 獲取叢集資訊
	clusterIDUint, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(clusterIDUint))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	terminalType := services.TerminalTypePod
	if t, exists := c.Get("terminal_type"); exists && t == "kubectl" {
		terminalType = services.TerminalTypeKubectl
	}

	// 升級到WebSocket連線
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		// 送出 WS close frame，讓前端收到正常關閉訊號，避免代理端 ECONNRESET
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session ended"))
		time.Sleep(200 * time.Millisecond) // 等待 close frame 傳遞後再關閉底層 TCP
		_ = conn.Close()
	}()

	h.RunPodTerminalWithConn(conn, cluster, clusterID, namespace, podName, container, userID, terminalType)
}

// RunPodTerminalWithConn 在已建立的 WebSocket 上執行 Pod 終端（kubectl Pod 終端等場景先推送進度再複用此邏輯）
func (h *PodTerminalHandler) RunPodTerminalWithConn(
	conn *websocket.Conn,
	cluster *models.Cluster,
	clusterIDStr, namespace, podName, container string,
	userID uint,
	terminalType services.TerminalType,
) {
	var auditSessionID uint
	if h.auditService != nil {
		auditSession, err := h.auditService.CreateSession(&services.CreateSessionRequest{
			UserID:     userID,
			ClusterID:  cluster.ID,
			TargetType: terminalType,
			Namespace:  namespace,
			Pod:        podName,
			Container:  container,
		})
		if err != nil {
			logger.Error("建立審計會話失敗", "error", err)
		} else {
			auditSessionID = auditSession.ID
		}
	}

	sessionID := fmt.Sprintf("%s-%s-%s-%d", clusterIDStr, namespace, podName, time.Now().Unix())
	ctx, cancel := context.WithCancel(context.Background())

	session := &PodTerminalSession{
		ID:             sessionID,
		AuditSessionID: auditSessionID,
		ClusterID:      clusterIDStr,
		Namespace:      namespace,
		PodName:        podName,
		Container:      container,
		Conn:           conn,
		Context:        ctx,
		Cancel:         cancel,
	}

	h.sessionsMutex.Lock()
	h.sessions[sessionID] = session
	h.sessionsMutex.Unlock()

	defer func() {
		h.sessionsMutex.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMutex.Unlock()
		cancel()
		h.closeSession(session)
		if h.auditService != nil && auditSessionID > 0 {
			_ = h.auditService.CloseSession(auditSessionID, "closed")
		}
	}()

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		time.Sleep(300 * time.Millisecond)
		return
	}
	client := k8sClient.GetClientset()
	k8sConfig := k8sClient.GetRestConfig()

	shell, err := h.findAvailableShell(client, k8sConfig, session)
	if err != nil {
		h.sendMessage(conn, "error", "此容器為 distroless 映像或不含標準 shell（/bin/sh, /bin/bash 等），無法開啟互動終端。")
		time.Sleep(300 * time.Millisecond)
		return
	}

	if err := h.startPodTerminal(client, k8sConfig, session, shell); err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("啟動Pod終端失敗: %v", err))
		time.Sleep(300 * time.Millisecond)
		return
	}

	containerInfo := ""
	if container != "" {
		containerInfo = fmt.Sprintf(" (container: %s)", container)
	}
	h.sendMessage(conn, "connected", fmt.Sprintf("Connected to pod %s/%s%s using %s", namespace, podName, containerInfo, shell))

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		var msg PodTerminalMessage
		if err := json.Unmarshal(data, &msg); err == nil && msg.Type != "" {
			switch msg.Type {
			case "input":
				h.handleInput(session, msg.Data)
			case "resize":
				h.handleResize(session, msg.Cols, msg.Rows)
			}
			continue
		}

		h.handleInput(session, string(data))
	}
}
