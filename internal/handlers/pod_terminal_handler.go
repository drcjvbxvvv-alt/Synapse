package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
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
