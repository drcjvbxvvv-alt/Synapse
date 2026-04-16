package handlers

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// KubectlTerminalHandler kubectl終端WebSocket處理器
type KubectlTerminalHandler struct {
	clusterService *services.ClusterService
	auditService   *services.AuditService
	upgrader       websocket.Upgrader
	sessions       map[string]*KubectlSession
	sessionsMutex  sync.RWMutex
	sem            chan struct{} // 全局並行 semaphore（與 PodTerminalHandler 共享上限空間）
	maxPerUser     int          // 每用戶上限
}

// KubectlSession kubectl會話
type KubectlSession struct {
	ID             string
	UserID         uint      // 建立此 session 的用戶 ID
	AuditSessionID uint      // 審計會話ID
	lastActivityAt time.Time // 最後 stdin 活動時間（閒置超時用）
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
	h := &KubectlTerminalHandler{
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
		sem:           make(chan struct{}, defaultTerminalMaxGlobal),
		maxPerUser:    defaultTerminalMaxPerUser,
	}
	go h.idleCleanup()
	return h
}

// idleCleanup 定期掃描並關閉閒置超過 terminalIdleTimeout 的 kubectl session。
func (h *KubectlTerminalHandler) idleCleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		h.sessionsMutex.Lock()
		for id, s := range h.sessions {
			if !s.lastActivityAt.IsZero() && time.Since(s.lastActivityAt) > terminalIdleTimeout {
				s.Cancel()
				if s.Cmd != nil && s.Cmd.Process != nil {
					_ = s.Cmd.Process.Kill()
				}
				delete(h.sessions, id)
				logger.Info("kubectl terminal idle timeout: session closed",
					"session_id", id,
					"user_id", s.UserID,
				)
			}
		}
		h.sessionsMutex.Unlock()
	}
}
