package services

import (
	"time"

	"github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// LogRetentionWorker 定期清理超過保留期限的操作日誌
type LogRetentionWorker struct {
	db        *gorm.DB
	retention time.Duration  // 保留時長（預設 90 天）
	metrics   *metrics.WorkerMetrics
}

// SetMetrics attaches Prometheus worker metrics.
func (w *LogRetentionWorker) SetMetrics(m *metrics.WorkerMetrics) { w.metrics = m }

// NewLogRetentionWorker 建立保留策略工作器；retention=0 時使用預設 90 天
func NewLogRetentionWorker(db *gorm.DB, retention time.Duration) *LogRetentionWorker {
	if retention <= 0 {
		retention = 90 * 24 * time.Hour
	}
	return &LogRetentionWorker{db: db, retention: retention}
}

// Start 啟動後臺清理 Goroutine（每天 00:05 UTC 執行一次）
func (w *LogRetentionWorker) Start() {
	go func() {
		// 首次啟動時延遲至下個 00:05 UTC
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		w.cleanup()
		for range ticker.C {
			w.cleanup()
		}
	}()
}

func (w *LogRetentionWorker) cleanup() {
	var run *metrics.WorkerRun
	if w.metrics != nil {
		run = w.metrics.Start("log_retention")
	}

	cutoff := time.Now().Add(-w.retention)
	result := w.db.Where("created_at < ?", cutoff).Delete(&models.OperationLog{})

	if run != nil {
		run.Done(result.Error)
	}
	if result.Error != nil {
		logger.Error("日誌保留清理失敗", "error", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logger.Info("日誌保留清理完成", "deleted", result.RowsAffected, "cutoff", cutoff.Format(time.RFC3339))
	}
}
