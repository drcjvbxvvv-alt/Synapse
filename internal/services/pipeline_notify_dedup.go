package services

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Pipeline Notification Dedup — 通知風暴防護
//
// 設計原則（CICD_ARCHITECTURE §15.4）：
//   - 同一事件 5 分鐘內同 channel 不重複發送
//   - 失敗重試的 Run 不發「started」通知
//   - Concurrency group 取消的 Run 不發通知
//   - In-memory map + 定期清理，適用單實例部署
// ---------------------------------------------------------------------------

const (
	// DefaultDedupWindow 預設去重視窗
	DefaultDedupWindow = 5 * time.Minute
	// maxDedupEntries 防止記憶體無限增長
	maxDedupEntries = 10000
)

// NotifyDedup 通知去重服務。
type NotifyDedup struct {
	mu      sync.Mutex
	seen    map[string]time.Time // key → last sent time
	window  time.Duration
	stopCh  chan struct{}
}

// NewNotifyDedup 建立通知去重服務。
func NewNotifyDedup(window time.Duration) *NotifyDedup {
	if window <= 0 {
		window = DefaultDedupWindow
	}
	d := &NotifyDedup{
		seen:   make(map[string]time.Time),
		window: window,
		stopCh: make(chan struct{}),
	}
	go d.cleanupLoop()
	return d
}

// ShouldNotify 判斷此通知是否應發送。
// 回傳 true 表示允許發送（未重複），false 表示被抑制（去重視窗內重複）。
//
// 參數：
//   - pipelineID: Pipeline ID
//   - event: 事件類型（如 "run_failed", "run_success"）
//   - channel: 通知通道識別（如 webhook URL 或 channel ID）
//   - runID: Pipeline Run ID（用於生成唯一 key）
func (d *NotifyDedup) ShouldNotify(pipelineID uint, event string, channel string, runID uint) bool {
	key := dedupKey(pipelineID, event, channel)

	d.mu.Lock()
	defer d.mu.Unlock()

	if lastSent, ok := d.seen[key]; ok {
		if time.Since(lastSent) < d.window {
			return false // 抑制：去重視窗內重複
		}
	}

	d.seen[key] = time.Now()

	// 防止記憶體洩漏
	if len(d.seen) > maxDedupEntries {
		d.evictOldest()
	}

	return true
}

// IsRetryRun 判斷是否為重試 Run（重試 Run 不發 started 通知）。
func IsRetryRun(retryCount int) bool {
	return retryCount > 0
}

// IsCancellationFromConcurrencyGroup 判斷是否因 concurrency group 被取消。
// 這類取消不應發通知（非使用者主動取消）。
func IsCancellationFromConcurrencyGroup(cancelReason string) bool {
	return cancelReason == "superseded_by_concurrency_group"
}

// Stop 停止清理 goroutine。
func (d *NotifyDedup) Stop() {
	close(d.stopCh)
}

// Stats 回傳目前的去重統計。
func (d *NotifyDedup) Stats() map[string]interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[string]interface{}{
		"active_entries": len(d.seen),
		"window":         d.window.String(),
		"max_entries":    maxDedupEntries,
	}
}

// cleanupLoop 定期清理過期的去重記錄。
func (d *NotifyDedup) cleanupLoop() {
	ticker := time.NewTicker(d.window)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.cleanup()
		}
	}
}

func (d *NotifyDedup) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for key, lastSent := range d.seen {
		if now.Sub(lastSent) >= d.window {
			delete(d.seen, key)
		}
	}
}

func (d *NotifyDedup) evictOldest() {
	// 找出最舊的 20% 並刪除
	evictCount := len(d.seen) / 5
	if evictCount == 0 {
		evictCount = 1
	}
	type entry struct {
		key  string
		time time.Time
	}
	entries := make([]entry, 0, len(d.seen))
	for k, t := range d.seen {
		entries = append(entries, entry{k, t})
	}
	// 簡單排序找最舊的
	for i := 0; i < evictCount && i < len(entries); i++ {
		oldest := i
		for j := i + 1; j < len(entries); j++ {
			if entries[j].time.Before(entries[oldest].time) {
				entries[oldest], entries[j] = entries[j], entries[oldest]
				oldest = j
			}
		}
		delete(d.seen, entries[i].key)
	}
}

func dedupKey(pipelineID uint, event string, channel string) string {
	raw := fmt.Sprintf("p:%d|e:%s|c:%s", pipelineID, event, channel)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:16]) // 128-bit 足夠
}
