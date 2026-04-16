package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

const (
	// defaultTerminalMaxGlobal 全局並行 terminal 上限（pod + kubectl 合計）
	defaultTerminalMaxGlobal = 200
	// defaultTerminalMaxPerUser 每用戶並行 terminal 上限
	defaultTerminalMaxPerUser = 5
)

// PodTerminalHandler Pod終端WebSocket處理器
type PodTerminalHandler struct {
	clusterService *services.ClusterService
	auditService   *services.AuditService
	k8sMgr         *k8s.ClusterInformerManager
	upgrader       websocket.Upgrader
	sessions       map[string]*PodTerminalSession
	sessionsMutex  sync.RWMutex
	sem            chan struct{} // 全局並行 semaphore
	maxPerUser     int          // 每用戶上限
}

// PodTerminalSession Pod終端會話
type PodTerminalSession struct {
	ID             string
	UserID         uint      // 建立此 session 的用戶 ID
	AuditSessionID uint      // 審計會話ID
	lastActivityAt time.Time // 最後 stdin/stdout 活動時間（閒置超時用）
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

// terminalIdleTimeout 閒置超過此時間的 session 會被自動關閉
const terminalIdleTimeout = 15 * time.Minute

// NewPodTerminalHandler 建立Pod終端處理器
func NewPodTerminalHandler(clusterService *services.ClusterService, auditService *services.AuditService, k8sMgr *k8s.ClusterInformerManager) *PodTerminalHandler {
	h := &PodTerminalHandler{
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
		sem:           make(chan struct{}, defaultTerminalMaxGlobal),
		maxPerUser:    defaultTerminalMaxPerUser,
	}
	go h.idleCleanup()
	return h
}

// idleCleanup 定期掃描並關閉閒置超過 terminalIdleTimeout 的 session。
func (h *PodTerminalHandler) idleCleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		h.sessionsMutex.Lock()
		for id, s := range h.sessions {
			if !s.lastActivityAt.IsZero() && time.Since(s.lastActivityAt) > terminalIdleTimeout {
				s.Cancel()
				delete(h.sessions, id)
				logger.Info("pod terminal idle timeout: session closed",
					"session_id", id,
					"user_id", s.UserID,
				)
			}
		}
		h.sessionsMutex.Unlock()
	}
}
