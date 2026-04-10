package handlers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// HandleKubectlTerminal 處理kubectl終端WebSocket連線
func (h *KubectlTerminalHandler) HandleKubectlTerminal(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.DefaultQuery("namespace", "default")
	userID := c.GetUint("user_id") // 從JWT中獲取使用者ID

	// 獲取叢集資訊
	cid, parseErr := strconv.ParseUint(clusterID, 10, 32)
	if parseErr != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(uint(cid))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 建立審計會話
	var auditSessionID uint
	if h.auditService != nil {
		auditSession, err := h.auditService.CreateSession(&services.CreateSessionRequest{
			UserID:     userID,
			ClusterID:  cluster.ID,
			TargetType: services.TerminalTypeKubectl,
			Namespace:  namespace,
		})
		if err != nil {
			logger.Error("建立審計會話失敗", "error", err)
		} else {
			auditSessionID = auditSession.ID
		}
	}

	// 升級到WebSocket連線
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("升級WebSocket連線失敗", "error", err)
		// 關閉審計會話
		if h.auditService != nil && auditSessionID > 0 {
			_ = h.auditService.CloseSession(auditSessionID, "error")
		}
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// 建立會話
	sessionID := fmt.Sprintf("%s-%d", clusterID, time.Now().Unix())
	ctx, cancel := context.WithCancel(context.Background())

	session := &KubectlSession{
		ID:             sessionID,
		AuditSessionID: auditSessionID,
		ClusterID:      clusterID,
		Namespace:      namespace,
		Conn:           conn,
		Context:        ctx,
		Cancel:         cancel,
		History:        make([]string, 0),
	}

	// 註冊會話
	h.sessionsMutex.Lock()
	h.sessions[sessionID] = session
	h.sessionsMutex.Unlock()

	// 清理會話
	defer func() {
		h.sessionsMutex.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMutex.Unlock()
		cancel()
		if session.Cmd != nil && session.Cmd.Process != nil {
			_ = session.Cmd.Process.Kill()
		}
		// 關閉審計會話
		if h.auditService != nil && auditSessionID > 0 {
			_ = h.auditService.CloseSession(auditSessionID, "closed")
		}
	}()

	// 建立臨時kubeconfig檔案
	kubeconfigPath, err := h.createTempKubeconfig(cluster)
	if err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("建立kubeconfig失敗: %v", err))
		return
	}
	defer func() {
		_ = os.Remove(kubeconfigPath)
	}()

	// 傳送歡迎訊息
	h.sendMessage(conn, "output", fmt.Sprintf("Connected to cluster: %s\n", cluster.Name))
	h.sendMessage(conn, "output", fmt.Sprintf("Default namespace: %s\n", namespace))
	h.sendMessage(conn, "command_result", "")

	// 處理WebSocket訊息
	for {
		var msg TerminalMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			logger.Error("讀取WebSocket訊息失敗", "error", err)
			break
		}

		switch msg.Type {
		case "input":
			h.handleInput(session, msg.Data)
		case "command":
			h.handleCommand(session, kubeconfigPath, namespace)
		case "interrupt":
			h.handleInterrupt(session)
		case "change_namespace":
			namespace = msg.Data
			session.Namespace = namespace
			h.sendMessage(conn, "namespace_changed", namespace)
		case "quick_command":
			h.handleQuickCommand(session, kubeconfigPath, namespace, msg.Data)
		}
	}
}

// handleInput 處理使用者輸入
func (h *KubectlTerminalHandler) handleInput(session *KubectlSession, input string) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if input == "\u007f" { // 退格鍵
		if len(session.LastCommand) > 0 {
			session.LastCommand = session.LastCommand[:len(session.LastCommand)-1]
			h.sendMessage(session.Conn, "output", "\b \b")
		}
	} else {
		session.LastCommand += input
		h.sendMessage(session.Conn, "output", input)
	}
}

// handleCommand 處理命令執行
func (h *KubectlTerminalHandler) handleCommand(session *KubectlSession, kubeconfigPath, namespace string) {
	session.Mutex.Lock()
	command := strings.TrimSpace(session.LastCommand)
	session.LastCommand = ""
	// 使用會話中的命名空間，而不是傳入的參數
	currentNamespace := session.Namespace
	session.Mutex.Unlock()

	if command == "" {
		h.sendMessage(session.Conn, "command_result", "")
		return
	}

	// 新增到歷史記錄
	session.History = append(session.History, command)
	if len(session.History) > 100 {
		session.History = session.History[1:]
	}

	// 記錄命令到審計資料庫（非同步）
	if h.auditService != nil && session.AuditSessionID > 0 {
		h.auditService.RecordCommandAsync(session.AuditSessionID, command, command, nil)
	}

	// 執行kubectl命令，使用會話中的命名空間
	h.executeKubectlCommand(session, kubeconfigPath, currentNamespace, command)
}

// handleQuickCommand 處理快捷命令
func (h *KubectlTerminalHandler) handleQuickCommand(session *KubectlSession, kubeconfigPath, namespace, command string) {
	h.sendMessage(session.Conn, "output", fmt.Sprintf("\n%s\n", command))

	// 記錄快捷命令到審計資料庫（非同步）
	if h.auditService != nil && session.AuditSessionID > 0 {
		h.auditService.RecordCommandAsync(session.AuditSessionID, command, command, nil)
	}

	// 使用會話中的命名空間，而不是傳入的參數
	h.executeKubectlCommand(session, kubeconfigPath, session.Namespace, command)
}
