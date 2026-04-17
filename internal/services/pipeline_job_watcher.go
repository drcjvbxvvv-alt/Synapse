package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	batchv1listers "k8s.io/client-go/listers/batch/v1"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// JobWatcher — 跨叢集 K8s Job 狀態追蹤
//
// 設計（CICD_ARCHITECTURE §7.8, §17）：
//   - 使用 Informer lister（已在 ClusterInformerManager 註冊）輪詢 Job 狀態
//   - Label selector: synapse.io/pipeline-run-id=<runID>
//   - 狀態機：pending → running → success/failed
//   - cancelling → 刪除 K8s Job（propagationPolicy: Background）→ cancelled
//   - 事件去重：以 StepRun.Status 為真值來源，避免重複更新
// ---------------------------------------------------------------------------

// JobWatcherConfig 可調參數。
type JobWatcherConfig struct {
	PollInterval            time.Duration // Job 狀態輪詢間隔
	TerminationGracePeriod  int64         // SIGTERM 後等待秒數
}

// DefaultJobWatcherConfig 傳回預設配置。
func DefaultJobWatcherConfig() JobWatcherConfig {
	return JobWatcherConfig{
		PollInterval:           3 * time.Second,
		TerminationGracePeriod: 30,
	}
}

// JobsListerProvider 提供 K8s Job lister 和 live client 的介面。
// 由 ClusterInformerManager 實作。
type JobsListerProvider interface {
	JobsLister(clusterID uint) batchv1listers.JobLister
	GetK8sClientByID(clusterID uint) *K8sClient
}

// JobWatcher 追蹤 Pipeline Run 的 K8s Job 狀態。
type JobWatcher struct {
	db         *gorm.DB
	provider   JobsListerProvider
	logSvc     *PipelineLogService
	rolloutSvc *RolloutService
	cfg        JobWatcherConfig

	// 正在追蹤的 Run（防止重複 watch）
	mu       sync.Mutex
	watching map[uint]context.CancelFunc // runID → cancel
}

// NewJobWatcher 建立 JobWatcher。
func NewJobWatcher(db *gorm.DB, provider JobsListerProvider, cfg JobWatcherConfig) *JobWatcher {
	return &JobWatcher{
		db:       db,
		provider: provider,
		cfg:      cfg,
		watching: make(map[uint]context.CancelFunc),
	}
}

// SetLogService 設定 Log 服務（可選）。設定後 Watcher 會在 Step 完成時收集 Pod log。
func (w *JobWatcher) SetLogService(logSvc *PipelineLogService) {
	w.logSvc = logSvc
}

// SetRolloutService 設定 Rollout 服務（可選）。設定後 Watcher 會在 rollout Step 狀態變更時更新 rollout_status / rollout_weight。
func (w *JobWatcher) SetRolloutService(rolloutSvc *RolloutService) {
	w.rolloutSvc = rolloutSvc
}

// WatchRun 開始追蹤指定 Run 的所有 StepRun Job 狀態。
// 冪等：同一 Run 不會重複追蹤。
func (w *JobWatcher) WatchRun(run *models.PipelineRun) {
	w.mu.Lock()
	if _, exists := w.watching[run.ID]; exists {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.watching[run.ID] = cancel
	w.mu.Unlock()

	go w.watchLoop(ctx, run)

	logger.Info("job watcher attached",
		"run_id", run.ID,
		"cluster_id", run.ClusterID,
	)
}

// StopWatchRun 停止追蹤指定 Run。
func (w *JobWatcher) StopWatchRun(runID uint) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if cancel, ok := w.watching[runID]; ok {
		cancel()
		delete(w.watching, runID)
	}
}

// Stop 實作 Stoppable 介面，停止所有追蹤（優雅關閉用）。
func (w *JobWatcher) Stop() {
	w.StopAll()
}

// StopAll 停止所有追蹤。
func (w *JobWatcher) StopAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, cancel := range w.watching {
		cancel()
		delete(w.watching, id)
	}
	logger.Info("job watcher stopped all watches")
}

// ---------------------------------------------------------------------------
// Watch loop
// ---------------------------------------------------------------------------

func (w *JobWatcher) watchLoop(ctx context.Context, run *models.PipelineRun) {
	defer w.StopWatchRun(run.ID)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			done, err := w.pollRunStatus(ctx, run)
			if err != nil {
				logger.Error("job watcher poll error",
					"run_id", run.ID, "error", err)
				continue
			}
			if done {
				logger.Info("job watcher completed",
					"run_id", run.ID)
				return
			}
		}
	}
}

// pollRunStatus 檢查 Run 的所有 StepRun 狀態，回傳 true 表示 Run 已結束。
func (w *JobWatcher) pollRunStatus(ctx context.Context, run *models.PipelineRun) (bool, error) {
	// 重新載入 Run 狀態
	var currentRun models.PipelineRun
	if err := w.db.WithContext(ctx).First(&currentRun, run.ID).Error; err != nil {
		return false, fmt.Errorf("reload run %d: %w", run.ID, err)
	}

	// Run 已被取消 → 清理 K8s Jobs
	if currentRun.Status == models.PipelineRunStatusCancelling {
		return w.handleCancellation(ctx, &currentRun)
	}

	// Run 已結束
	if isTerminalRunStatus(currentRun.Status) {
		return true, nil
	}

	// 載入所有 StepRun
	var stepRuns []models.StepRun
	if err := w.db.WithContext(ctx).
		Where("pipeline_run_id = ?", run.ID).
		Find(&stepRuns).Error; err != nil {
		return false, fmt.Errorf("load step runs: %w", err)
	}

	// 取得叢集的 Job lister
	lister := w.provider.JobsLister(currentRun.ClusterID)
	if lister == nil {
		return false, fmt.Errorf("no job lister for cluster %d", currentRun.ClusterID)
	}

	allDone := true
	anyFailed := false

	for i := range stepRuns {
		sr := &stepRuns[i]

		// 跳過已結束的 Step
		if isTerminalStepStatus(sr.Status) {
			if sr.Status == models.StepRunStatusFailed {
				anyFailed = true
			}
			continue
		}

		// 還沒提交 Job 的 Step 不在此處理
		if sr.JobName == "" {
			allDone = false
			continue
		}

		// 從 Informer cache 查找 Job
		job, err := w.findJob(lister, sr)
		if err != nil {
			// Job 已從叢集消失（被回收或手動刪除）→ 嘗試 live API 確認
			k8sClient := w.provider.GetK8sClientByID(currentRun.ClusterID)
			if k8sClient != nil && sr.JobNamespace != "" {
				liveCtx, liveCancel := context.WithTimeout(ctx, 5*time.Second)
				_, liveErr := k8sClient.GetClientset().BatchV1().
					Jobs(sr.JobNamespace).Get(liveCtx, sr.JobName, metav1.GetOptions{})
				liveCancel()
				if liveErr != nil {
					// Job 確實不存在 → 標記 step 為 failed
					logger.Warn("job disappeared from cluster, marking step as failed",
						"job_name", sr.JobName, "step_run_id", sr.ID)
					sr.Status = models.StepRunStatusFailed
					now := time.Now()
					sr.FinishedAt = &now
					sr.Error = "job not found in cluster (deleted or garbage collected)"
					if err := w.db.WithContext(ctx).Save(sr).Error; err != nil {
						logger.Error("failed to save orphaned step run",
							"step_run_id", sr.ID, "error", err)
					}
					anyFailed = true
					continue
				}
			}
			logger.Warn("job not found in cache, may be pending",
				"job_name", sr.JobName, "step_run_id", sr.ID)
			allDone = false
			continue
		}

		// 更新 StepRun 狀態
		changed := w.syncStepRunStatus(sr, job)
		if changed {
			// Rollout Step 額外更新 rollout_status / rollout_weight
			w.enrichRolloutFields(ctx, sr, currentRun.ClusterID)

			if err := w.db.WithContext(ctx).Save(sr).Error; err != nil {
				logger.Error("failed to save step run status",
					"step_run_id", sr.ID, "error", err)
			}

			// Step 完成時收集 Pod log 並持久化
			if isTerminalStepStatus(sr.Status) && w.logSvc != nil {
				w.collectStepLogs(ctx, &currentRun, sr)
			}
		}

		if !isTerminalStepStatus(sr.Status) {
			allDone = false
		}
		if sr.Status == models.StepRunStatusFailed {
			anyFailed = true
		}
	}

	// 所有 Step 完成 → 更新 Run 狀態
	if allDone {
		return true, w.finalizeRun(ctx, &currentRun, anyFailed)
	}

	return false, nil
}

// ---------------------------------------------------------------------------
// Cancellation 處理
// ---------------------------------------------------------------------------

func (w *JobWatcher) handleCancellation(ctx context.Context, run *models.PipelineRun) (bool, error) {
	k8sClient := w.provider.GetK8sClientByID(run.ClusterID)
	if k8sClient == nil {
		return false, fmt.Errorf("no k8s client for cluster %d", run.ClusterID)
	}

	var stepRuns []models.StepRun
	if err := w.db.WithContext(ctx).
		Where("pipeline_run_id = ? AND job_name != '' AND status IN ?",
			run.ID,
			[]string{models.StepRunStatusRunning, models.StepRunStatusPending},
		).Find(&stepRuns).Error; err != nil {
		return false, fmt.Errorf("load active step runs: %w", err)
	}

	propagation := metav1.DeletePropagationBackground
	var gracePeriod int64 // 0 = immediate termination on cancel

	for i := range stepRuns {
		sr := &stepRuns[i]
		deleteCtx, deleteCancel := context.WithTimeout(ctx, 30*time.Second)

		err := k8sClient.GetClientset().BatchV1().
			Jobs(sr.JobNamespace).
			Delete(deleteCtx, sr.JobName, metav1.DeleteOptions{
				PropagationPolicy:  &propagation,
				GracePeriodSeconds: &gracePeriod,
			})
		deleteCancel()

		if err != nil {
			logger.Warn("failed to delete k8s job during cancellation",
				"job_name", sr.JobName, "error", err)
		}

		sr.Status = models.StepRunStatusCancelled
		now := time.Now()
		sr.FinishedAt = &now
		w.db.WithContext(ctx).Save(sr)

		logger.Info("step job deleted for cancellation",
			"step_run_id", sr.ID,
			"job_name", sr.JobName,
		)
	}

	// 標記 Run 為 cancelled
	now := time.Now()
	run.Status = models.PipelineRunStatusCancelled
	run.FinishedAt = &now
	if err := w.db.WithContext(ctx).Save(run).Error; err != nil {
		return false, fmt.Errorf("finalize cancelled run: %w", err)
	}

	logger.Info("pipeline run cancelled", "run_id", run.ID)
	return true, nil
}

// ---------------------------------------------------------------------------
// Run 完結
// ---------------------------------------------------------------------------

func (w *JobWatcher) finalizeRun(ctx context.Context, run *models.PipelineRun, anyFailed bool) error {
	now := time.Now()
	run.FinishedAt = &now

	if anyFailed {
		run.Status = models.PipelineRunStatusFailed
	} else {
		run.Status = models.PipelineRunStatusSuccess
	}

	if err := w.db.WithContext(ctx).Save(run).Error; err != nil {
		return fmt.Errorf("finalize run %d: %w", run.ID, err)
	}

	logger.Info("pipeline run finalized",
		"run_id", run.ID,
		"status", run.Status,
	)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (w *JobWatcher) findJob(lister batchv1listers.JobLister, sr *models.StepRun) (*batchv1.Job, error) {
	if sr.JobNamespace != "" {
		return lister.Jobs(sr.JobNamespace).Get(sr.JobName)
	}
	// Fallback: search by label across all namespaces
	selector := labels.Set{
		"synapse.io/step-run-id": fmt.Sprintf("%d", sr.ID),
	}.AsSelector()
	jobs, err := lister.List(selector)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("job not found for step run %d", sr.ID)
	}
	return jobs[0], nil
}

func isTerminalRunStatus(status string) bool {
	switch status {
	case models.PipelineRunStatusSuccess,
		models.PipelineRunStatusFailed,
		models.PipelineRunStatusCancelled,
		models.PipelineRunStatusRejected:
		return true
	}
	return false
}

func isTerminalStepStatus(status string) bool {
	switch status {
	case models.StepRunStatusSuccess,
		models.StepRunStatusFailed,
		models.StepRunStatusCancelled,
		models.StepRunStatusSkipped:
		return true
	}
	return false
}
