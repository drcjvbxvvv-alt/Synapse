package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"kubepolaris/internal/models"
	"kubepolaris/internal/services"
	"kubepolaris/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// PodTerminalHandler Pod终端WebSocket处理器
type PodTerminalHandler struct {
	clusterService *services.ClusterService
	upgrader       websocket.Upgrader
	sessions       map[string]*PodTerminalSession
	sessionsMutex  sync.RWMutex
}

// PodTerminalSession Pod终端会话
type PodTerminalSession struct {
	ID        string
	ClusterID string
	Namespace string
	PodName   string
	Container string
	Conn      *websocket.Conn
	Context   context.Context
	Cancel    context.CancelFunc
	Mutex     sync.Mutex

	// Kubernetes连接相关
	stdinReader  io.ReadCloser
	stdinWriter  io.WriteCloser
	stdoutReader io.ReadCloser
	stdoutWriter io.WriteCloser
	winSizeChan  chan *remotecommand.TerminalSize
	done         chan struct{}
}

// PodTerminalMessage Pod终端消息
type PodTerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// NewPodTerminalHandler 创建Pod终端处理器
func NewPodTerminalHandler(clusterService *services.ClusterService) *PodTerminalHandler {
	return &PodTerminalHandler{
		clusterService: clusterService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 在生产环境中应该检查Origin
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		sessions:      make(map[string]*PodTerminalSession),
		sessionsMutex: sync.RWMutex{},
	}
}

// HandlePodTerminal 处理Pod终端WebSocket连接
func (h *PodTerminalHandler) HandlePodTerminal(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	podName := c.Param("name")
	container := c.DefaultQuery("container", "")

	// 获取集群信息
	clusterIDUint, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的集群ID"})
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(clusterIDUint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}

	// 升级到WebSocket连接
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 创建会话
	sessionID := fmt.Sprintf("%s-%s-%s-%d", clusterID, namespace, podName, time.Now().Unix())
	ctx, cancel := context.WithCancel(context.Background())

	session := &PodTerminalSession{
		ID:        sessionID,
		ClusterID: clusterID,
		Namespace: namespace,
		PodName:   podName,
		Container: container,
		Conn:      conn,
		Context:   ctx,
		Cancel:    cancel,
	}

	// 注册会话
	h.sessionsMutex.Lock()
	h.sessions[sessionID] = session
	h.sessionsMutex.Unlock()

	// 清理会话
	defer func() {
		h.sessionsMutex.Lock()
		delete(h.sessions, sessionID)
		h.sessionsMutex.Unlock()
		cancel()
		h.closeSession(session)
	}()

	// 创建Kubernetes配置
	k8sConfig, err := h.createK8sConfig(cluster)
	if err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("创建Kubernetes配置失败: %v", err))
		return
	}

	// 创建Kubernetes客户端
	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("创建Kubernetes客户端失败: %v", err))
		return
	}

	// 查找可用的shell
	shell, err := h.findAvailableShell(client, k8sConfig, session)
	if err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("未找到可用的shell: %v", err))
		return
	}

	// 启动Pod终端连接
	if err := h.startPodTerminal(client, k8sConfig, session, shell); err != nil {
		h.sendMessage(conn, "error", fmt.Sprintf("启动Pod终端失败: %v", err))
		return
	}

	// 发送连接成功消息
	containerInfo := ""
	if container != "" {
		containerInfo = fmt.Sprintf(" (container: %s)", container)
	}
	h.sendMessage(conn, "connected", fmt.Sprintf("Connected to pod %s/%s%s using %s", namespace, podName, containerInfo, shell))

	// 处理WebSocket消息
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		// 优先尝试按JSON解析
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

		// 兼容纯文本：直接作为输入
		h.handleInput(session, string(data))
	}
}

// findAvailableShell 查找可用的shell
func (h *PodTerminalHandler) findAvailableShell(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession) (string, error) {
	shells := []string{"bash", "sh", "ash", "dash", "zsh", "ksh"}

	for _, shell := range shells {
		if h.hasShellInContainer(client, k8sConfig, session, shell) {
			return shell, nil
		}
	}

	return "", fmt.Errorf("未找到任何可用的shell")
}

// hasShellInContainer 检查容器中是否有指定的shell
func (h *PodTerminalHandler) hasShellInContainer(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession, shell string) bool {
	testScript := fmt.Sprintf("command -v %s", shell)
	command := []string{"sh", "-c", testScript}

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(session.PodName).
		Namespace(session.Namespace).SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: session.Container,
		Command:   command,
		Stdout:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k8sConfig, "POST", req.URL())
	if err != nil {
		return false
	}

	var buf bytes.Buffer
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &buf,
		Tty:    false,
	})
	if err != nil {
		return false
	}

	result := strings.TrimSpace(buf.String())
	return strings.HasSuffix(result, shell)
}

// startPodTerminal 启动Pod终端连接
func (h *PodTerminalHandler) startPodTerminal(client *kubernetes.Clientset, k8sConfig *rest.Config, session *PodTerminalSession, shell string) error {
	// 创建管道
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	session.stdinReader = stdinReader
	session.stdinWriter = stdinWriter
	session.stdoutReader = stdoutReader
	session.stdoutWriter = stdoutWriter
	session.winSizeChan = make(chan *remotecommand.TerminalSize, 10)
	session.done = make(chan struct{})

	// 设置默认终端大小
	session.winSizeChan <- &remotecommand.TerminalSize{
		Width:  120,
		Height: 30,
	}

	// 启动输出读取协程
	go h.readOutput(session)

	// 启动Kubernetes exec
	go func() {
		defer func() {
			select {
			case <-session.done:
			default:
				close(session.done)
			}
			h.sendMessage(session.Conn, "disconnected", "Pod终端连接已断开")
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
			h.sendMessage(session.Conn, "error", fmt.Sprintf("创建执行器失败: %v", err))
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
			h.sendMessage(session.Conn, "error", fmt.Sprintf("执行失败: %v", err))
		}
	}()

	return nil
}

// handleInput 处理用户输入
func (h *PodTerminalHandler) handleInput(session *PodTerminalSession, input string) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if session.stdinWriter != nil {
		_, err := session.stdinWriter.Write([]byte(input))
		if err != nil {
			h.sendMessage(session.Conn, "error", "写入输入失败")
		}
	}
}

// handleResize 处理终端大小调整
func (h *PodTerminalHandler) handleResize(session *PodTerminalSession, cols, rows int) {
	if session.winSizeChan != nil {
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

// readOutput 读取命令输出
func (h *PodTerminalHandler) readOutput(session *PodTerminalSession) {
	buffer := make([]byte, 1024)
	for {
		n, err := session.stdoutReader.Read(buffer)
		if err != nil {
			break
		}

		if n > 0 {
			h.sendMessage(session.Conn, "data", string(buffer[:n]))
		}
	}
}

// closeSession 关闭会话
func (h *PodTerminalHandler) closeSession(session *PodTerminalSession) {
	if session.stdinWriter != nil {
		session.stdinWriter.Close()
	}
	if session.stdoutReader != nil {
		session.stdoutReader.Close()
	}
	if session.done != nil {
		select {
		case <-session.done:
		default:
			close(session.done)
		}
	}
}

// createK8sConfig 创建Kubernetes配置
func (h *PodTerminalHandler) createK8sConfig(cluster *models.Cluster) (*rest.Config, error) {
	config := &rest.Config{
		Host: cluster.APIServer,
	}

	if cluster.SATokenEnc != "" {
		config.BearerToken = cluster.SATokenEnc
	}

	if cluster.CAEnc != "" {
		config.CAData = []byte(cluster.CAEnc)
	} else {
		config.Insecure = true
	}

	return config, nil
}

// sendMessage 发送WebSocket消息
func (h *PodTerminalHandler) sendMessage(conn *websocket.Conn, msgType, data string) {
	msg := PodTerminalMessage{
		Type: msgType,
		Data: data,
	}

	if err := conn.WriteJSON(msg); err != nil {
		logger.Error("发送WebSocket消息失败", "error", err)
	}
}

// terminalStream 实现io.Reader和io.Writer接口
type terminalStream struct {
	session *PodTerminalSession
}

func (t *terminalStream) Read(p []byte) (int, error) {
	return t.session.stdinReader.Read(p)
}

func (t *terminalStream) Write(p []byte) (int, error) {
	return len(p), nil // 不需要写入
}

// terminalSizeQueue 实现remotecommand.TerminalSizeQueue接口
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
