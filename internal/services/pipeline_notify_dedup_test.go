package services

import (
	"testing"
	"time"
)

func TestNotifyDedup_FirstCallAllowed(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	if !d.ShouldNotify(1, "run_failed", "slack-channel", 100) {
		t.Error("expected first notification to be allowed")
	}
}

func TestNotifyDedup_DuplicateSuppressed(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack-channel", 100)

	if d.ShouldNotify(1, "run_failed", "slack-channel", 101) {
		t.Error("expected duplicate notification to be suppressed")
	}
}

func TestNotifyDedup_DifferentEventAllowed(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack-channel", 100)

	if !d.ShouldNotify(1, "run_success", "slack-channel", 101) {
		t.Error("expected different event to be allowed")
	}
}

func TestNotifyDedup_DifferentChannelAllowed(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack-channel", 100)

	if !d.ShouldNotify(1, "run_failed", "email", 101) {
		t.Error("expected different channel to be allowed")
	}
}

func TestNotifyDedup_DifferentPipelineAllowed(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack", 100)

	if !d.ShouldNotify(2, "run_failed", "slack", 200) {
		t.Error("expected different pipeline to be allowed")
	}
}

func TestNotifyDedup_ExpiredWindowAllowed(t *testing.T) {
	d := NewNotifyDedup(50 * time.Millisecond) // very short window for test
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack", 100)
	time.Sleep(60 * time.Millisecond)

	if !d.ShouldNotify(1, "run_failed", "slack", 101) {
		t.Error("expected notification after window expires to be allowed")
	}
}

func TestNotifyDedup_Cleanup(t *testing.T) {
	d := NewNotifyDedup(50 * time.Millisecond)
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack", 100)
	d.ShouldNotify(2, "run_failed", "slack", 200)

	time.Sleep(60 * time.Millisecond)
	d.cleanup()

	stats := d.Stats()
	if stats["active_entries"].(int) != 0 {
		t.Errorf("expected 0 active entries after cleanup, got %d", stats["active_entries"].(int))
	}
}

func TestNotifyDedup_Stats(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	d.ShouldNotify(1, "run_failed", "slack", 100)
	d.ShouldNotify(2, "run_success", "email", 200)

	stats := d.Stats()
	if stats["active_entries"].(int) != 2 {
		t.Errorf("expected 2 active entries, got %d", stats["active_entries"].(int))
	}
	if stats["window"].(string) != "5m0s" {
		t.Errorf("expected 5m0s window, got %s", stats["window"].(string))
	}
}

func TestIsRetryRun(t *testing.T) {
	if IsRetryRun(0) {
		t.Error("expected non-retry for count=0")
	}
	if !IsRetryRun(1) {
		t.Error("expected retry for count=1")
	}
	if !IsRetryRun(3) {
		t.Error("expected retry for count=3")
	}
}

func TestIsCancellationFromConcurrencyGroup(t *testing.T) {
	if !IsCancellationFromConcurrencyGroup("superseded_by_concurrency_group") {
		t.Error("expected true for concurrency group cancellation")
	}
	if IsCancellationFromConcurrencyGroup("user_cancelled") {
		t.Error("expected false for user cancellation")
	}
	if IsCancellationFromConcurrencyGroup("") {
		t.Error("expected false for empty reason")
	}
}

func TestNotifyDedup_EvictOldest(t *testing.T) {
	d := NewNotifyDedup(5 * time.Minute)
	defer d.Stop()

	// Fill beyond max (simulate by setting internal state)
	d.mu.Lock()
	for i := 0; i < maxDedupEntries+1; i++ {
		d.seen[dedupKey(uint(i), "event", "channel")] = time.Now()
	}
	d.mu.Unlock()

	// Next call should trigger eviction
	d.ShouldNotify(99999, "run_failed", "slack", 1)

	d.mu.Lock()
	count := len(d.seen)
	d.mu.Unlock()

	if count > maxDedupEntries {
		t.Errorf("expected entries to be within limit, got %d", count)
	}
}
