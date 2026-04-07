package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// PodTerminalHandler Pod終端WebSocket處理器
type PodTerminalHandler struct {
	clusterService *services.ClusterService
	auditService   *services.AuditService
	k8sMgr         *k8s.ClusterInformerManager
	upgrader       websocket.Upgrader
	sessions       map[string]*PodTerminalSession
	sessionsMutex  sync.RWMutex
}

// PodTerminalSession Pod終端會話
type PodTerminalSession struct {
	ID             string
	AuditSessionID uint // 審計會話ID
	ClusterID      string
	Namespace      string
	PodName        string
	Container      string
	Conn           *websocket.Conn
	Context        context.Context
	Cancel         context.CancelFunc
	Mutex          sync.Mutex

	// 命令捕獲（從終端輸出中提取完整命令，包括Tab補全結果）
	currentLine      strings.Builder // 當前行的輸出內容
	lastCompleteLine string          // 上一個完整行（用於提取命令）
	pendingEnter     bool            // 是否有待處理的回車鍵

	// Kubernetes連線相關
	stdinReader  io.ReadCloser
	stdinWriter  io.WriteCloser
	stdoutReader io.ReadCloser
	stdoutWriter io.WriteCloser
	winSizeChan  chan *remotecommand.TerminalSize
	done         chan struct{}
}

// PodTerminalMessage Pod終端訊息
type PodTerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// NewPodTerminalHandler 建立Pod終端處理器
func NewPodTerminalHandler(clusterService *services.ClusterService, auditService *services.AuditService, k8sMgr *k8s.ClusterInformerManager) *PodTerminalHandler {
	return &PodTerminalHandler{
		clusterService: clusterService,
		auditService:   auditService,
		k8sMgr:         k8sMgr,
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
		sessions:      make(map[string]*PodTerminalSession),
		sessionsMutex: sync.RWMutex{},
	}
}

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
		return
	}
	client := k8sClient.GetClientset()
	k8sConfig := k8sClient.GetRestConfig()

	shell, err := h.findAvailableShell(client, k8sConfig, session)
	if err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("未找到可用的shell: %v", err))
		return
	}

	if err := h.startPodTerminal(client, k8sConfig, session, shell); err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("啟動Pod終端失敗: %v", err))
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

// findAvailableShell 查詢可用的shell
// 策略：直接以絕對路徑執行各 shell，不依賴 sh 包裝（極簡容器可能連 sh 都沒有）
func (h *PodTerminalHandler) findAvailableShell(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession) (string, error) {
	// 按常見程度排序，優先嘗試完整路徑，再嘗試裸名（PATH 查找）
	candidates := []string{
		"/bin/bash", "/usr/bin/bash",
		"/bin/sh", "/usr/bin/sh",
		"/bin/ash", "/usr/bin/ash",
		"/bin/dash", "/usr/bin/dash",
		"/bin/zsh", "/usr/bin/zsh",
		"/bin/ksh", "/usr/bin/ksh",
	}

	for _, shell := range candidates {
		if h.tryExecShell(client, k8sConfig, session, shell) {
			return shell, nil
		}
	}

	return "", fmt.Errorf("未找到任何可用的shell")
}

// tryExecShell 直接執行指定 shell 路徑，成功回傳 true
// 不使用 sh -c 包裝，避免容器內無 sh 時全部誤判為不存在
func (h *PodTerminalHandler) tryExecShell(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession, shell string) bool {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(session.PodName).
		Namespace(session.Namespace).SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: session.Container,
		Command:   []string{shell, "-c", "echo ok"},
		Stdout:    true,
		Stderr:    false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k8sConfig, "POST", req.URL())
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var buf bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &buf, Tty: false})
	return err == nil && strings.TrimSpace(buf.String()) == "ok"
}

// startPodTerminal 啟動Pod終端連線
func (h *PodTerminalHandler) startPodTerminal(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession, shell string) error {
	// 建立管道
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	session.stdinReader = stdinReader
	session.stdinWriter = stdinWriter
	session.stdoutReader = stdoutReader
	session.stdoutWriter = stdoutWriter
	session.winSizeChan = make(chan *remotecommand.TerminalSize, 10)
	session.done = make(chan struct{})

	// 設定預設終端大小
	session.winSizeChan <- &remotecommand.TerminalSize{
		Width:  120,
		Height: 30,
	}

	// 啟動輸出讀取協程
	go h.readOutput(session)

	// 啟動Kubernetes exec
	go func() {
		defer func() {
			select {
			case <-session.done:
			default:
				close(session.done)
			}
			h.sendMessage(session.Conn, "disconnected", "Pod終端連線已斷開")
		}()

		req := client.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(session.PodName).
			Namespace(session.Namespace).
			SubResource("exec")

		req.VersionedParams(&v1.PodExecOptions{
			Container: session.Container,
			Command:   []string{shell},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(k8sConfig, "POST", req.URL())
		if err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("建立執行器失敗: %v", err))
			return
		}

		streamOption := remotecommand.StreamOptions{
			Stdin:             &terminalStream{session: session},
			Stdout:            session.stdoutWriter,
			Stderr:            session.stdoutWriter,
			TerminalSizeQueue: &terminalSizeQueue{session: session},
			Tty:               true,
		}

		if err := exec.StreamWithContext(session.Context, streamOption); err != nil {
			h.sendMessage(session.Conn, "error", fmt.Sprintf("執行失敗: %v", err))
		}
	}()

	return nil
}

// handleInput 處理使用者輸入
func (h *PodTerminalHandler) handleInput(session *PodTerminalSession, input string) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if session.stdinWriter != nil {
		_, err := session.stdinWriter.Write([]byte(input))
		if err != nil {
			h.sendMessage(session.Conn, "error", "寫入輸入失敗")
			return
		}
	}

	// 檢測回車鍵，標記待處理（命令將從輸出中提取）
	if h.auditService != nil && session.AuditSessionID > 0 {
		if strings.Contains(input, "\r") || strings.Contains(input, "\n") {
			session.pendingEnter = true
		} else if input == "\x03" {
			// Ctrl+C 清空當前行
			session.currentLine.Reset()
		}
	}
}

// handleResize 處理終端大小調整
func (h *PodTerminalHandler) handleResize(session *PodTerminalSession, cols, rows int) {
	if session.winSizeChan != nil {
		if cols < 0 || cols > math.MaxUint16 {
			cols = 80
		}
		if rows < 0 || rows > math.MaxUint16 {
			rows = 24
		}
		size := &remotecommand.TerminalSize{
			Width:  uint16(cols),
			Height: uint16(rows),
		}
		select {
		case session.winSizeChan <- size:
		case <-session.done:
		}
	}
}

// readOutput 讀取命令輸出
func (h *PodTerminalHandler) readOutput(session *PodTerminalSession) {
	buffer := make([]byte, wsBufferSize)
	for {
		n, err := session.stdoutReader.Read(buffer)
		if err != nil {
			break
		}

		if n > 0 {
			output := string(buffer[:n])
			h.sendMessage(session.Conn, "data", output)

			// 追蹤終端輸出，用於提取完整命令（包括Tab補全結果）
			if h.auditService != nil && session.AuditSessionID > 0 {
				h.trackOutputForCommand(session, output)
			}
		}
	}
}

// trackOutputForCommand 追蹤輸出以提取命令
func (h *PodTerminalHandler) trackOutputForCommand(session *PodTerminalSession, output string) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	for _, c := range output {
		switch c {
		case '\n':
			// 遇到換行，儲存當前行並檢查是否需要記錄命令
			currentContent := session.currentLine.String()
			session.currentLine.Reset()

			if session.pendingEnter && currentContent != "" {
				// 使用者按了回車，提取命令
				cmd := h.extractCommandFromLine(currentContent)
				if cmd != "" {
					h.auditService.RecordCommandAsync(session.AuditSessionID, cmd, cmd, nil)
				}
				session.pendingEnter = false
			}
			session.lastCompleteLine = currentContent

		case '\r':
			// 回車符，可能是行首返回，暫時忽略
			continue

		case '\x1b':
			// ESC 字元，可能是 ANSI 轉義序列的開始，忽略
			continue

		case '\x07':
			// Bell 字元，忽略
			continue

		default:
			// 過濾掉不可列印字元和ANSI序列中的字元
			if c >= 32 && c < 127 {
				session.currentLine.WriteRune(c)
			}
		}
	}
}

// extractCommandFromLine 從行內容中提取命令（去掉shell提示符）
func (h *PodTerminalHandler) extractCommandFromLine(line string) string {
	// 去掉 ANSI 轉義序列
	line = h.stripANSI(line)
	line = strings.TrimSpace(line)

	if line == "" {
		return ""
	}

	// 嘗試識別並去掉常見的 shell 提示符
	// 格式如: "bash-4.4#", "root@hostname:~#", "$ ", "# ", "[user@host ~]$ "
	promptPatterns := []string{
		"# ", // root 提示符
		"$ ", // 普通使用者提示符
		"] ", // 方括號結尾的提示符
		"> ", // 其他提示符
	}

	for _, pattern := range promptPatterns {
		if idx := strings.LastIndex(line, pattern); idx != -1 {
			cmd := strings.TrimSpace(line[idx+len(pattern):])
			if cmd != "" {
				return cmd
			}
		}
	}

	// 如果沒有找到提示符模式，檢查是否看起來像命令
	// 如果行以常見命令開頭，可能就是命令本身
	commonCommands := []string{"ls", "cd", "cat", "grep", "kubectl", "find", "pwd", "echo", "ps", "top", "vi", "vim", "nano", "apt", "yum", "dnf", "pip", "npm", "go", "python", "java", "curl", "wget", "tar", "cp", "mv", "rm", "mkdir", "chmod", "chown", "df", "du", "free", "whoami", "id", "date", "tail", "head", "less", "more", "sort", "uniq", "wc", "awk", "sed", "cut", "tr", "diff", "patch", "git", "docker", "helm", "make", "sh", "bash", "exit", "clear", "history"}

	lineLower := strings.ToLower(line)
	for _, cmd := range commonCommands {
		if strings.HasPrefix(lineLower, cmd+" ") || lineLower == cmd {
			return line
		}
	}

	return ""
}

// stripANSI 去掉ANSI轉義序列
func (h *PodTerminalHandler) stripANSI(s string) string {
	// 簡單的ANSI轉義序列過濾
	result := strings.Builder{}
	inEscape := false
	for _, c := range s {
		if c == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			// ANSI序列通常以字母結尾
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(c)
	}
	return result.String()
}

// closeSession 關閉會話
func (h *PodTerminalHandler) closeSession(session *PodTerminalSession) {
	if session.stdinWriter != nil {
		_ = session.stdinWriter.Close()
	}
	if session.stdoutReader != nil {
		_ = session.stdoutReader.Close()
	}
	if session.done != nil {
		select {
		case <-session.done:
		default:
			close(session.done)
		}
	}
}

// sendMessage 傳送WebSocket訊息
func (h *PodTerminalHandler) sendMessage(conn *websocket.Conn, msgType, data string) {
	msg := PodTerminalMessage{
		Type: msgType,
		Data: data,
	}

	if err := conn.WriteJSON(msg); err != nil {
		logger.Error("傳送WebSocket訊息失敗", "error", err)
	}
}

// terminalStream 實現io.Reader和io.Writer介面
type terminalStream struct {
	session *PodTerminalSession
}

func (t *terminalStream) Read(p []byte) (int, error) {
	return t.session.stdinReader.Read(p)
}

func (t *terminalStream) Write(p []byte) (int, error) {
	return len(p), nil // 不需要寫入
}

// terminalSizeQueue 實現remotecommand.TerminalSizeQueue介面
type terminalSizeQueue struct {
	session *PodTerminalSession
}

func (t *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.session.winSizeChan:
		return size
	case <-t.session.done:
		return nil
	}
}
