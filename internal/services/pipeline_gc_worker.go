package services

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PipelineGCWorker — 清理 K8s Job 孤兒 + PipelineRun 過期 + PipelineLog 過期
//
// 設計原則（CICD_ARCHITECTURE §7.12）：
//   - K8s Job: TTL controller 負責主要清理（10min），GCWorker 補掃孤兒
//   - PipelineRun: 預設保留 90 天，soft delete（deleted_at）
//   - PipelineLog: 預設保留 30 天，hard delete（Log chunk 無需 soft delete）
// ---------------------------------------------------------------------------

// PipelineGCConfig GC Worker 設定。
type PipelineGCConfig struct {
	// K8s Job 孤兒掃描間隔
	OrphanJobScanInterval time.Duration
	// K8s Job 完成後超過此時間視為孤兒（TTL controller 未清理）
	OrphanJobMaxAge time.Duration

	// PipelineRun 保留天數
	RunRetentionDays int
	// PipelineRun 清理掃描間隔
	RunCleanupInterval time.Duration

	// PipelineLog 保留天數
	LogRetentionDays int
	// PipelineLog 清理掃描間隔
	LogCleanupInterval time.Duration
}

// DefaultPipelineGCConfig 預設 GC 設定。
func DefaultPipelineGCConfig() PipelineGCConfig {
	return PipelineGCConfig{
		OrphanJobScanInterval: 10 * time.Minute,
		OrphanJobMaxAge:       1 * time.Hour,
		RunRetentionDays:      90,
		RunCleanupInterval:    24 * time.Hour,
		LogRetentionDays:      30,
		LogCleanupInterval:    24 * time.Hour,
	}
}

// PipelineGCWorker 負責清理 Pipeline 相關的過期資源。
type PipelineGCWorker struct {
	db          *gorm.DB
	k8sProvider PipelineK8sProvider
	cfg         PipelineGCConfig
	stopCh      chan struct{}
}

// NewPipelineGCWorker 建立 GC Worker。
func NewPipelineGCWorker(db *gorm.DB, k8sProvider PipelineK8sProvider, cfg PipelineGCConfig) *PipelineGCWorker {
	return &PipelineGCWorker{
		db:          db,
		k8sProvider: k8sProvider,
		cfg:         cfg,
		stopCh:      make(chan struct{}),
	}
}

// Start 啟動所有 GC goroutine。
func (w *PipelineGCWorker) Start() {
	go w.orphanJobLoop()
	go w.runRetentionLoop()
	go w.logRetentionLoop()
	logger.Info("pipeline GC worker started",
		"orphan_job_interval", w.cfg.OrphanJobScanInterval,
		"run_retention_days", w.cfg.RunRetentionDays,
		"log_retention_days", w.cfg.LogRetentionDays,
	)
}

// Stop 停止所有 GC goroutine。
func (w *PipelineGCWorker) Stop() {
	close(w.stopCh)
}

// ---------------------------------------------------------------------------
// 1. K8s Job 孤兒清理
// ---------------------------------------------------------------------------

func (w *PipelineGCWorker) orphanJobLoop() {
	ticker := time.NewTicker(w.cfg.OrphanJobScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.cleanupOrphanJobs()
		}
	}
}

// cleanupOrphanJobs 掃描所有叢集，刪除已完成但超過 maxAge 的 Pipeline Job。
func (w *PipelineGCWorker) cleanupOrphanJobs() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 查詢所有使用中的 clusterID
	var clusterIDs []uint
	if err := w.db.WithContext(ctx).
		Model(&models.PipelineRun{}).
		Select("DISTINCT cluster_id").
		Where("status IN ?", []string{
			models.PipelineRunStatusRunning,
			models.PipelineRunStatusQueued,
		}).
		Scan(&clusterIDs).Error; err != nil {
		logger.Error("gc: failed to query active cluster IDs", "error", err)
		return
	}

	// 也掃描最近有完成 Run 的叢集
	var recentClusterIDs []uint
	cutoff := time.Now().Add(-24 * time.Hour)
	if err := w.db.WithContext(ctx).
		Model(&models.PipelineRun{}).
		Select("DISTINCT cluster_id").
		Where("finished_at > ?", cutoff).
		Scan(&recentClusterIDs).Error; err != nil {
		logger.Warn("gc: failed to query recent cluster IDs", "error", err)
	}

	// 合併去重
	clusterSet := make(map[uint]bool, len(clusterIDs)+len(recentClusterIDs))
	for _, id := range clusterIDs {
		clusterSet[id] = true
	}
	for _, id := range recentClusterIDs {
		clusterSet[id] = true
	}

	totalCleaned := 0
	for clusterID := range clusterSet {
		cleaned := w.cleanupClusterOrphanJobs(ctx, clusterID)
		totalCleaned += cleaned
	}

	if totalCleaned > 0 {
		logger.Info("gc: orphan jobs cleaned", "total", totalCleaned)
	}
}

func (w *PipelineGCWorker) cleanupClusterOrphanJobs(ctx context.Context, clusterID uint) int {
	k8sClient := w.k8sProvider.GetK8sClientByID(clusterID)
	if k8sClient == nil {
		return 0
	}

	// 查詢帶有 synapse.io/pipeline-step=true label 的 Jobs
	selector := labels.Set{"synapse.io/pipeline-step": "true"}.AsSelector().String()

	// 列出所有 namespace 的 pipeline Jobs
	jobList, err := k8sClient.GetClientset().BatchV1().
		Jobs("").
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		logger.Warn("gc: failed to list pipeline jobs",
			"cluster_id", clusterID,
			"error", err,
		)
		return 0
	}

	cleaned := 0
	now := time.Now()
	propagation := metav1.DeletePropagationBackground

	for i := range jobList.Items {
		job := &jobList.Items[i]

		// 只清理已完成（Succeeded 或 Failed）的 Job
		if job.Status.Succeeded == 0 && !isJobFailed(job) {
			continue
		}

		// 判斷完成時間
		completionTime := job.Status.CompletionTime
		if completionTime == nil {
			// 沒有完成時間，用最後一個 condition 時間
			for _, cond := range job.Status.Conditions {
				if cond.LastTransitionTime.Time.After(time.Time{}) {
					completionTime = &cond.LastTransitionTime
				}
			}
		}
		if completionTime == nil {
			continue
		}

		// 超過 maxAge 才清理
		if now.Sub(completionTime.Time) < w.cfg.OrphanJobMaxAge {
			continue
		}

		// 刪除 Job（propagation: Background 會同時清理 Pod）
		if err := k8sClient.GetClientset().BatchV1().
			Jobs(job.Namespace).
			Delete(ctx, job.Name, metav1.DeleteOptions{
				PropagationPolicy: &propagation,
			}); err != nil {
			logger.Warn("gc: failed to delete orphan job",
				"cluster_id", clusterID,
				"job_name", job.Name,
				"namespace", job.Namespace,
				"error", err,
			)
			continue
		}
		cleaned++
	}

	return cleaned
}

// isJobFailed 判斷 batchv1.Job 是否已 Failed。
func isJobFailed(job *batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == "True" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 2. PipelineRun 保留策略清理
// ---------------------------------------------------------------------------

func (w *PipelineGCWorker) runRetentionLoop() {
	// 首次延遲至 01:00 UTC
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	select {
	case <-w.stopCh:
		return
	case <-time.After(time.Until(next)):
	}

	w.cleanupExpiredRuns()

	ticker := time.NewTicker(w.cfg.RunCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.cleanupExpiredRuns()
		}
	}
}

// cleanupExpiredRuns 軟刪除超過保留期限的已完成 PipelineRun。
func (w *PipelineGCWorker) cleanupExpiredRuns() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cutoff := time.Now().AddDate(0, 0, -w.cfg.RunRetentionDays)
	terminalStatuses := []string{
		models.PipelineRunStatusSuccess,
		models.PipelineRunStatusFailed,
		models.PipelineRunStatusCancelled,
		models.PipelineRunStatusRejected,
	}

	// Soft delete 已完成 + 超過保留期限的 Run
	result := w.db.WithContext(ctx).
		Where("status IN ? AND finished_at < ? AND deleted_at IS NULL",
			terminalStatuses, cutoff).
		Delete(&models.PipelineRun{})

	if result.Error != nil {
		logger.Error("gc: failed to cleanup expired runs", "error", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		logger.Info("gc: expired pipeline runs cleaned",
			"deleted", result.RowsAffected,
			"cutoff", cutoff.Format("2006-01-02"),
			"retention_days", w.cfg.RunRetentionDays,
		)
	}

	// 同步清理這些 Run 對應的 StepRun（hard delete，因為 StepRun 沒有 DeletedAt）
	w.cleanupOrphanedStepRuns(ctx, cutoff, terminalStatuses)
}

// cleanupOrphanedStepRuns 清理已 soft-deleted Run 的 StepRun 記錄。
func (w *PipelineGCWorker) cleanupOrphanedStepRuns(ctx context.Context, cutoff time.Time, terminalStatuses []string) {
	// 找出已 soft-deleted 的 Run ID
	var deletedRunIDs []uint
	if err := w.db.WithContext(ctx).Unscoped().
		Model(&models.PipelineRun{}).
		Select("id").
		Where("deleted_at IS NOT NULL AND finished_at < ?", cutoff).
		Scan(&deletedRunIDs).Error; err != nil {
		logger.Error("gc: failed to query deleted run IDs", "error", err)
		return
	}

	if len(deletedRunIDs) == 0 {
		return
	}

	// 分批刪除 StepRun（每批 100）
	batchSize := 100
	for i := 0; i < len(deletedRunIDs); i += batchSize {
		end := i + batchSize
		if end > len(deletedRunIDs) {
			end = len(deletedRunIDs)
		}
		batch := deletedRunIDs[i:end]

		result := w.db.WithContext(ctx).
			Where("pipeline_run_id IN ?", batch).
			Delete(&models.StepRun{})
		if result.Error != nil {
			logger.Error("gc: failed to cleanup orphaned step runs",
				"batch_start", i,
				"error", result.Error,
			)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. PipelineLog 保留策略清理
// ---------------------------------------------------------------------------

func (w *PipelineGCWorker) logRetentionLoop() {
	// 首次延遲至 02:00 UTC
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	select {
	case <-w.stopCh:
		return
	case <-time.After(time.Until(next)):
	}

	w.cleanupExpiredLogs()

	ticker := time.NewTicker(w.cfg.LogCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.cleanupExpiredLogs()
		}
	}
}

// cleanupExpiredLogs hard-delete 超過保留期限的 PipelineLog chunks。
func (w *PipelineGCWorker) cleanupExpiredLogs() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cutoff := time.Now().AddDate(0, 0, -w.cfg.LogRetentionDays)

	result := w.db.WithContext(ctx).
		Where("stored_at < ?", cutoff).
		Delete(&models.PipelineLog{})

	if result.Error != nil {
		logger.Error("gc: failed to cleanup expired pipeline logs", "error", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		logger.Info("gc: expired pipeline logs cleaned",
			"deleted", result.RowsAffected,
			"cutoff", cutoff.Format("2006-01-02"),
			"retention_days", w.cfg.LogRetentionDays,
		)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// GCStats 回傳 GC 相關統計（供 API 或除錯用）。
func (w *PipelineGCWorker) GCStats(ctx context.Context) (map[string]interface{}, error) {
	var totalRuns, deletedRuns, totalLogs int64

	if err := w.db.WithContext(ctx).Model(&models.PipelineRun{}).Count(&totalRuns).Error; err != nil {
		return nil, fmt.Errorf("count runs: %w", err)
	}
	if err := w.db.WithContext(ctx).Unscoped().Model(&models.PipelineRun{}).
		Where("deleted_at IS NOT NULL").Count(&deletedRuns).Error; err != nil {
		return nil, fmt.Errorf("count deleted runs: %w", err)
	}
	if err := w.db.WithContext(ctx).Model(&models.PipelineLog{}).Count(&totalLogs).Error; err != nil {
		return nil, fmt.Errorf("count logs: %w", err)
	}

	runCutoff := time.Now().AddDate(0, 0, -w.cfg.RunRetentionDays)
	logCutoff := time.Now().AddDate(0, 0, -w.cfg.LogRetentionDays)

	return map[string]interface{}{
		"total_runs":         totalRuns,
		"deleted_runs":       deletedRuns,
		"total_log_chunks":   totalLogs,
		"run_retention_days": w.cfg.RunRetentionDays,
		"log_retention_days": w.cfg.LogRetentionDays,
		"run_cutoff":         runCutoff.Format("2006-01-02"),
		"log_cutoff":         logCutoff.Format("2006-01-02"),
	}, nil
}
