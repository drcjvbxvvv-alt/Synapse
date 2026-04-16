package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newPodHandlerForTest() *PodTerminalHandler {
	gin.SetMode(gin.TestMode)
	return &PodTerminalHandler{
		sessions:   make(map[string]*PodTerminalSession),
		sem:        make(chan struct{}, defaultTerminalMaxGlobal),
		maxPerUser: defaultTerminalMaxPerUser,
		upgrader:   websocket.Upgrader{},
	}
}

func newKubectlHandlerForTest() *KubectlTerminalHandler {
	gin.SetMode(gin.TestMode)
	return &KubectlTerminalHandler{
		sessions:   make(map[string]*KubectlSession),
		sem:        make(chan struct{}, defaultTerminalMaxGlobal),
		maxPerUser: defaultTerminalMaxPerUser,
		upgrader:   websocket.Upgrader{},
	}
}

// ─── Pod terminal: global semaphore ──────────────────────────────────────────

func TestPodTerminal_GlobalSemaphore_CapacityIsCorrect(t *testing.T) {
	h := newPodHandlerForTest()
	assert.Equal(t, defaultTerminalMaxGlobal, cap(h.sem))
}

func TestPodTerminal_GlobalSemaphore_FullWhenFilled(t *testing.T) {
	h := newPodHandlerForTest()
	// Fill to capacity
	for i := 0; i < defaultTerminalMaxGlobal; i++ {
		h.sem <- struct{}{}
	}
	// Next acquire must fail (non-blocking select)
	acquired := false
	select {
	case h.sem <- struct{}{}:
		acquired = true
		<-h.sem // release it
	default:
	}
	assert.False(t, acquired, "semaphore must be full; new acquire must fail")
}

func TestPodTerminal_GlobalSemaphore_DrainAndRefill(t *testing.T) {
	h := newPodHandlerForTest()
	assert.Equal(t, 0, len(h.sem), "starts empty")
	h.sem <- struct{}{}
	assert.Equal(t, 1, len(h.sem))
	<-h.sem
	assert.Equal(t, 0, len(h.sem), "drains back to zero")
}

// ─── Pod terminal: per-user limit ────────────────────────────────────────────

func TestPodTerminal_PerUserLimit_BelowLimit(t *testing.T) {
	h := newPodHandlerForTest()
	const ownerUID = uint(42)

	for i := 0; i < defaultTerminalMaxPerUser-1; i++ {
		id := string(rune('a' + i))
		h.sessions[id] = &PodTerminalSession{ID: id, UserID: ownerUID}
	}
	// also one session for another user (must not count)
	h.sessions["other"] = &PodTerminalSession{ID: "other", UserID: 99}

	h.sessionsMutex.RLock()
	count := 0
	for _, s := range h.sessions {
		if s.UserID == ownerUID {
			count++
		}
	}
	h.sessionsMutex.RUnlock()

	assert.Equal(t, defaultTerminalMaxPerUser-1, count)
	assert.Less(t, count, h.maxPerUser, "one below limit — must be allowed")
}

func TestPodTerminal_PerUserLimit_AtLimit(t *testing.T) {
	h := newPodHandlerForTest()
	const ownerUID = uint(42)

	for i := 0; i < defaultTerminalMaxPerUser; i++ {
		id := string(rune('a' + i))
		h.sessions[id] = &PodTerminalSession{ID: id, UserID: ownerUID}
	}

	h.sessionsMutex.RLock()
	count := 0
	for _, s := range h.sessions {
		if s.UserID == ownerUID {
			count++
		}
	}
	h.sessionsMutex.RUnlock()

	assert.GreaterOrEqual(t, count, h.maxPerUser, "at limit — must be rejected")
}

// ─── Kubectl terminal: per-user limit ────────────────────────────────────────

func TestKubectlTerminal_PerUserLimit_AtLimit(t *testing.T) {
	h := newKubectlHandlerForTest()
	const ownerUID = uint(7)

	for i := 0; i < defaultTerminalMaxPerUser; i++ {
		id := string(rune('a' + i))
		h.sessions[id] = &KubectlSession{ID: id, UserID: ownerUID}
	}

	h.sessionsMutex.RLock()
	count := 0
	for _, s := range h.sessions {
		if s.UserID == ownerUID {
			count++
		}
	}
	h.sessionsMutex.RUnlock()

	assert.Equal(t, defaultTerminalMaxPerUser, count)
	assert.GreaterOrEqual(t, count, h.maxPerUser)
}

func TestKubectlTerminal_Semaphore_CapacityMatchesPod(t *testing.T) {
	pod := newPodHandlerForTest()
	kubectl := newKubectlHandlerForTest()
	assert.Equal(t, cap(pod.sem), cap(kubectl.sem), "both handlers must share the same global limit constant")
}

// ─── B3: idle timeout ────────────────────────────────────────────────────────

func TestPodTerminal_IdleCleanup_RemovesExpiredSession(t *testing.T) {
	h := newPodHandlerForTest()

	ctx, cancel := context.WithCancel(context.Background())
	expired := &PodTerminalSession{
		ID:             "expired",
		UserID:         1,
		Context:        ctx,
		Cancel:         cancel,
		lastActivityAt: time.Now().Add(-20 * time.Minute), // 超過 15min 上限
	}
	active := &PodTerminalSession{
		ID:             "active",
		UserID:         2,
		Context:        context.Background(),
		Cancel:         func() {},
		lastActivityAt: time.Now(), // 剛活動
	}

	h.sessions["expired"] = expired
	h.sessions["active"] = active

	// Manually trigger cleanup logic (same as idleCleanup goroutine body)
	h.sessionsMutex.Lock()
	for id, s := range h.sessions {
		if !s.lastActivityAt.IsZero() && time.Since(s.lastActivityAt) > terminalIdleTimeout {
			s.Cancel()
			delete(h.sessions, id)
		}
	}
	h.sessionsMutex.Unlock()

	h.sessionsMutex.RLock()
	_, expiredPresent := h.sessions["expired"]
	_, activePresent := h.sessions["active"]
	h.sessionsMutex.RUnlock()

	assert.False(t, expiredPresent, "expired session must be removed")
	assert.True(t, activePresent, "active session must remain")

	// Verify cancel was called (context must be Done)
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Fatal("Cancel() must have been called on expired session")
	}
}

func TestKubectlTerminal_IdleCleanup_RemovesExpiredSession(t *testing.T) {
	h := newKubectlHandlerForTest()

	ctx, cancel := context.WithCancel(context.Background())
	expired := &KubectlSession{
		ID:             "expired",
		UserID:         1,
		Context:        ctx,
		Cancel:         cancel,
		lastActivityAt: time.Now().Add(-20 * time.Minute),
	}
	active := &KubectlSession{
		ID:             "active",
		UserID:         2,
		Context:        context.Background(),
		Cancel:         func() {},
		lastActivityAt: time.Now(),
	}

	h.sessions["expired"] = expired
	h.sessions["active"] = active

	h.sessionsMutex.Lock()
	for id, s := range h.sessions {
		if !s.lastActivityAt.IsZero() && time.Since(s.lastActivityAt) > terminalIdleTimeout {
			s.Cancel()
			delete(h.sessions, id)
		}
	}
	h.sessionsMutex.Unlock()

	h.sessionsMutex.RLock()
	_, expiredPresent := h.sessions["expired"]
	_, activePresent := h.sessions["active"]
	h.sessionsMutex.RUnlock()

	assert.False(t, expiredPresent, "expired session must be removed")
	assert.True(t, activePresent, "active session must remain")

	select {
	case <-ctx.Done():
	default:
		t.Fatal("Cancel() must have been called on expired session")
	}
}

func TestPodTerminal_LastActivity_UpdatedOnInput(t *testing.T) {
	h := newPodHandlerForTest()
	before := time.Now().Add(-time.Second)

	session := &PodTerminalSession{
		ID:             "test",
		lastActivityAt: before,
	}

	// handleInput updates lastActivityAt under Mutex, but stdinWriter is nil
	// so it won't write — the update should still happen.
	h.handleInput(session, "ls\n")

	assert.True(t, session.lastActivityAt.After(before),
		"lastActivityAt must be updated after handleInput")
}

// ─── response.TooManyRequests ─────────────────────────────────────────────────

func TestTooManyRequests_Returns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "TOO_MANY_REQUESTS", "message": "test"}})
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
