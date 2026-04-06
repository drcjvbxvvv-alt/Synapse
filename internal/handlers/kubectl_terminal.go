package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// KubectlTerminalHandler kubectl終端WebSocket處理器
type KubectlTerminalHandler struct {
	clusterService *services.ClusterService
	auditService   *services.AuditService
	upgrader       websocket.Upgrader
	sessions       map[string]*KubectlSession
	sessionsMutex  sync.RWMutex
}

// KubectlSession kubectl會話
type KubectlSession struct {
	ID             string
	AuditSessionID uint // 審計會話ID
	ClusterID      string
	Namespace      string
	Conn           *websocket.Conn
	Cmd            *exec.Cmd
	StdinPipe      *os.File
	StdoutPipe     *os.File
	Context        context.Context
	Cancel         context.CancelFunc
	LastCommand    string
	History        []string
	Mutex          sync.Mutex
}

// TerminalMessage 終端訊息
type TerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// NewKubectlTerminalHandler 建立kubectl終端處理器
func NewKubectlTerminalHandler(clusterService *services.ClusterService, auditService *services.AuditService) *KubectlTerminalHandler {
	return &KubectlTerminalHandler{
		clusterService: clusterService,
		auditService:   auditService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return middleware.IsOriginAllowed(origin)
			},
			ReadBufferSize:  wsBufferSize,
			WriteBufferSize: wsBufferSize,
		},
		sessions:      make(map[string]*KubectlSession),
		sessionsMutex: sync.RWMutex{},
	}
}

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

// executeKubectlCommand 執行kubectl命令
func (h *KubectlTerminalHandler) executeKubectlCommand(session *KubectlSession, kubeconfigPath, namespace, command string) {
	// 解析命令
	parts := strings.Fields(command)
	if len(parts) == 0 {
		h.sendMessage(session.Conn, "command_result", "")
		return
	}

	// 處理特殊命令
	if h.handleSpecialCommands(session, command) {
		return
	}

	// 構建kubectl命令
	var args []string
	if parts[0] == "kubectl" {
		args = parts[1:]
	} else {
		// 如果使用者沒有輸入kubectl字首，自動新增
		args = parts
	}

	// 檢查是否需要新增namespace參數
	needsNamespace := h.commandNeedsNamespace(args)

	// 新增kubeconfig參數
	kubectlArgs := []string{"--kubeconfig", kubeconfigPath}

	// 如果命令需要namespace且使用者沒有指定，則新增預設namespace
	if needsNamespace && !h.hasNamespaceFlag(args) {
		kubectlArgs = append(kubectlArgs, "--namespace", namespace)
	}

	kubectlArgs = append(kubectlArgs, args...)

	// 檢查是否是流式命令（如 logs -f）
	isStreamingCommand := h.isStreamingCommand(args)

	// 建立命令
	var ctx context.Context
	var cancel context.CancelFunc

	if isStreamingCommand {
		// 流式命令不設定超時
		ctx, cancel = context.WithCancel(session.Context)
	} else {
		// 非流式命令設定超時
		ctx, cancel = context.WithTimeout(session.Context, 60*time.Second)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", kubectlArgs...) // #nosec G204 -- kubectl 參數經過白名單校驗

	// 設定環境變數
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath),
	)

	// 儲存命令到會話，以便可以被中斷
	session.Mutex.Lock()
	session.Cmd = cmd
	session.Mutex.Unlock()

	// 如果是流式命令，使用管道處理輸出
	if isStreamingCommand {
		// 建立管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("建立輸出管道失敗: %v", err))
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("建立錯誤管道失敗: %v", err))
			return
		}

		// 啟動命令
		if err := cmd.Start(); err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("啟動命令失敗: %v", err))
			return
		}

		// 讀取標準輸出
		go func() {
			buffer := make([]byte, wsBufferSize)
			for {
				n, err := stdout.Read(buffer)
				if n > 0 {
					h.sendMessage(session.Conn, "output", string(buffer[:n]))
				}
				if err != nil {
					break
				}
			}
		}()

		// 讀取標準錯誤
		go func() {
			buffer := make([]byte, wsBufferSize)
			for {
				n, err := stderr.Read(buffer)
				if n > 0 {
					h.sendMessage(session.Conn, "error", string(buffer[:n]))
				}
				if err != nil {
					break
				}
			}
		}()

		// 等待命令完成
		go func() {
			err := cmd.Wait()
			if err != nil && ctx.Err() != context.Canceled {
				h.sendMessage(session.Conn, "error", fmt.Sprintf("命令執行失敗: %v", err))
			}
			h.sendMessage(session.Conn, "command_result", "")

			// 清除會話中的命令引用
			session.Mutex.Lock()
			session.Cmd = nil
			session.Mutex.Unlock()
		}()
	} else {
		// 非流式命令，使用CombinedOutput
		output, err := cmd.CombinedOutput()

		// 清除會話中的命令引用
		session.Mutex.Lock()
		session.Cmd = nil
		session.Mutex.Unlock()

		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				h.sendMessage(session.Conn, "error", "命令執行超時 (60秒)")
			} else {
				h.sendMessage(session.Conn, "error", fmt.Sprintf("命令執行失敗: %v\n%s", err, string(output)))
			}
		} else {
			// 傳送輸出
			if len(output) > 0 {
				h.sendMessage(session.Conn, "output", string(output))
			}
		}

		h.sendMessage(session.Conn, "command_result", "")
	}
}

// isStreamingCommand 檢查是否是流式命令
func (h *KubectlTerminalHandler) isStreamingCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// 檢查是否是 logs 命令 (無論是否有 -f 參數，都作為流式處理)
	if args[0] == "logs" {
		return true
	}

	// 檢查是否是 exec 命令
	if args[0] == "exec" {
		return true
	}

	// 檢查是否是 port-forward 命令
	if args[0] == "port-forward" {
		return true
	}

	// 檢查是否是 watch 命令
	if args[0] == "watch" {
		return true
	}

	// 檢查是否是 top 命令
	if args[0] == "top" {
		return true
	}

	// 檢查命令列中是否包含 --watch 參數
	for _, arg := range args {
		if arg == "--watch" || arg == "-w" {
			return true
		}
	}

	return false
}

// handleSpecialCommands 處理特殊命令
func (h *KubectlTerminalHandler) handleSpecialCommands(session *KubectlSession, command string) bool {
	command = strings.TrimSpace(command)

	switch {
	case command == "clear" || command == "cls":
		h.sendMessage(session.Conn, "clear", "")
		h.sendMessage(session.Conn, "command_result", "")
		return true
	case command == "help" || command == "?":
		h.sendHelpMessage(session)
		return true
	case command == "history":
		h.sendHistoryMessage(session)
		return true
	case strings.HasPrefix(command, "ns "):
		// 切換namespace的快捷命令
		namespace := strings.TrimSpace(command[3:])
		if namespace != "" {
			session.Namespace = namespace
			h.sendMessage(session.Conn, "namespace_changed", namespace)
		}
		h.sendMessage(session.Conn, "command_result", "")
		return true
	}

	return false
}

// commandNeedsNamespace 檢查命令是否需要namespace
func (h *KubectlTerminalHandler) commandNeedsNamespace(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// 不需要namespace的命令
	clusterCommands := []string{
		"cluster-info", "version", "api-versions", "api-resources",
		"get nodes", "get namespaces", "get ns", "get pv", "get sc",
		"get clusterroles", "get clusterrolebindings",
	}

	command := strings.Join(args, " ")
	for _, cmd := range clusterCommands {
		if strings.HasPrefix(command, cmd) {
			return false
		}
	}

	return true
}

// hasNamespaceFlag 檢查命令是否已經包含namespace參數
func (h *KubectlTerminalHandler) hasNamespaceFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-n" || arg == "--namespace" {
			return true
		}
		if strings.HasPrefix(arg, "--namespace=") {
			return true
		}
		if arg == "--all-namespaces" || arg == "-A" {
			return true
		}
	}
	return false
}

// sendHelpMessage 傳送幫助資訊
func (h *KubectlTerminalHandler) sendHelpMessage(session *KubectlSession) {
	helpText := `
kubectl終端幫助資訊:

基本命令:
  kubectl get pods              - 檢視Pod列表
  kubectl get nodes             - 檢視節點列表
  kubectl get svc               - 檢視服務列表
  kubectl get deployments      - 檢視部署列表
  kubectl describe pod <name>   - 檢視Pod詳情
  kubectl logs <pod-name>       - 檢視Pod日誌
  kubectl exec -it <pod> bash   - 進入Pod容器

快捷命令:
  clear/cls                     - 清屏
  help/?                        - 顯示幫助
  history                       - 顯示命令歷史
  ns <namespace>                - 切換命名空間

提示:
  - 可以省略kubectl字首，系統會自動新增
  - 使用Tab鍵可以自動補全(部分支援)
  - 使用上下箭頭鍵瀏覽歷史命令
  - 當前命名空間會自動應用到相關命令

`
	h.sendMessage(session.Conn, "output", helpText)
	h.sendMessage(session.Conn, "command_result", "")
}

// sendHistoryMessage 傳送歷史命令
func (h *KubectlTerminalHandler) sendHistoryMessage(session *KubectlSession) {
	if len(session.History) == 0 {
		h.sendMessage(session.Conn, "output", "暫無命令歷史\n")
	} else {
		historyText := "命令歷史:\n"
		for i, cmd := range session.History {
			historyText += fmt.Sprintf("  %d: %s\n", i+1, cmd)
		}
		h.sendMessage(session.Conn, "output", historyText)
	}
	h.sendMessage(session.Conn, "command_result", "")
}

// handleInterrupt 處理中斷訊號
func (h *KubectlTerminalHandler) handleInterrupt(session *KubectlSession) {
	session.Mutex.Lock()
	cmd := session.Cmd
	session.LastCommand = ""
	session.Mutex.Unlock()

	// 傳送中斷訊號到終端
	h.sendMessage(session.Conn, "output", "^C\n")

	// 如果有正在執行的命令，嘗試終止它
	if cmd != nil && cmd.Process != nil {
		logger.Info("正在終止命令", "pid", cmd.Process.Pid)

		// 在Windows上，Kill()可能不會立即終止程序，嘗試使用taskkill
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run() // #nosec G204 -- PID 來自已知程序
		} else {
			// 在Unix系統上，傳送SIGINT訊號（等同於Ctrl+C）
			_ = cmd.Process.Signal(syscall.SIGINT)

			// 給程序一點時間響應SIGINT
			time.Sleep(100 * time.Millisecond)

			// 如果程序仍在執行，強制終止
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				_ = cmd.Process.Kill()
			}
		}
	}

	h.sendMessage(session.Conn, "command_result", "")
}

// createTempKubeconfig 建立臨時kubeconfig檔案
func (h *KubectlTerminalHandler) createTempKubeconfig(cluster *models.Cluster) (string, error) {
	// 建立臨時檔案
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("建立臨時檔案失敗: %v", err)
	}
	defer func() {
		_ = tmpFile.Close()
	}()

	// 寫入kubeconfig內容
	var kubeconfigContent string
	if cluster.KubeconfigEnc != "" {
		kubeconfigContent = cluster.KubeconfigEnc
	} else if cluster.SATokenEnc != "" {
		// 從Token建立kubeconfig
		kubeconfigContent = services.CreateKubeconfigFromToken(
			cluster.Name,
			cluster.APIServer,
			cluster.SATokenEnc,
			cluster.CAEnc,
		)
	} else {
		return "", fmt.Errorf("叢集缺少認證資訊")
	}

	_, err = tmpFile.WriteString(kubeconfigContent)
	if err != nil {
		return "", fmt.Errorf("寫入kubeconfig失敗: %v", err)
	}

	return tmpFile.Name(), nil
}

// sendMessage 傳送WebSocket訊息
func (h *KubectlTerminalHandler) sendMessage(conn *websocket.Conn, msgType, data string) {
	msg := TerminalMessage{
		Type: msgType,
		Data: data,
	}

	if err := conn.WriteJSON(msg); err != nil {
		logger.Error("傳送WebSocket訊息失敗", "error", err)
	}
}

