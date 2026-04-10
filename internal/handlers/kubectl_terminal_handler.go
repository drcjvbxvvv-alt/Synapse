package handlers

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
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
