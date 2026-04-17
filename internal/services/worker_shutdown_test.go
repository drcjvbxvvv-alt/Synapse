package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── CertExpiryWorker ──────────────────────────────────────────────────────────

func TestCertExpiryWorker_Stop_DoesNotHang(t *testing.T) {
	w := NewCertExpiryWorker(nil)
	w.Start()

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ✅ stopped cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("CertExpiryWorker.Stop() did not return within 2 seconds")
	}
}

func TestCertExpiryWorker_Stop_BeforeStart_DoesNotHang(t *testing.T) {
	// Stop() called before Start() — should not panic.
	w := NewCertExpiryWorker(nil)

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CertExpiryWorker.Stop() (pre-Start) did not return within 2 seconds")
	}
}

// ── LogRetentionWorker ───────────────────────────────────────────────────────

func TestLogRetentionWorker_Stop_DoesNotHang(t *testing.T) {
	w := NewLogRetentionWorker(nil, 90*24*time.Hour)
	w.Start()

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ✅ stopped cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("LogRetentionWorker.Stop() did not return within 2 seconds")
	}
}

func TestLogRetentionWorker_Stop_BeforeStart_DoesNotHang(t *testing.T) {
	w := NewLogRetentionWorker(nil, 0) // 0 → uses default 90 days

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("LogRetentionWorker.Stop() (pre-Start) did not return within 2 seconds")
	}
}

// ── JobWatcher ───────────────────────────────────────────────────────────────

func TestJobWatcher_Stop_DelegatesToStopAll(t *testing.T) {
	w := NewJobWatcher(nil, nil, DefaultJobWatcherConfig())

	// Manually register a fake in-progress watch entry.
	cancelled := false
	w.mu.Lock()
	w.watching[99] = func() { cancelled = true }
	w.mu.Unlock()

	w.Stop() // must call StopAll internally

	w.mu.Lock()
	remaining := len(w.watching)
	w.mu.Unlock()

	assert.True(t, cancelled, "Stop() must cancel in-flight watches")
	assert.Equal(t, 0, remaining, "watching map must be empty after Stop()")
}

// ── Stoppable interface compliance ───────────────────────────────────────────

// Compile-time assertions: all worker types must satisfy router.Stoppable
// (which is interface{ Stop() }).  We verify this via local interface.
type stoppable interface{ Stop() }

var _ stoppable = (*CertExpiryWorker)(nil)
var _ stoppable = (*LogRetentionWorker)(nil)
var _ stoppable = (*JobWatcher)(nil)
var _ stoppable = (*EventAlertWorker)(nil)
var _ stoppable = (*CostWorker)(nil)
var _ stoppable = (*ImageIndexWorker)(nil)
var _ stoppable = (*PipelineScheduler)(nil)
var _ stoppable = (*NotifyDedup)(nil)
