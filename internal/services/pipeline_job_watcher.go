package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	db       *gorm.DB
	provider JobsListerProvider
	logSvc   *PipelineLogService
	cfg      JobWatcherConfig

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
			logger.Warn("job not found in cache, may be pending",
				"job_name", sr.JobName, "step_run_id", sr.ID)
			allDone = false
			continue
		}

		// 更新 StepRun 狀態
		changed := w.syncStepRunStatus(sr, job)
		if changed {
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
// Job → StepRun 狀態同步
// ---------------------------------------------------------------------------

// syncStepRunStatus 根據 K8s Job 狀態更新 StepRun，回傳是否有變更。
func (w *JobWatcher) syncStepRunStatus(sr *models.StepRun, job *batchv1.Job) bool {
	newStatus := w.jobToStepStatus(job)

	// 事件去重：狀態未變不更新
	if newStatus == sr.Status {
		return false
	}

	oldStatus := sr.Status
	sr.Status = newStatus

	if newStatus == models.StepRunStatusSuccess || newStatus == models.StepRunStatusFailed {
		now := time.Now()
		sr.FinishedAt = &now

		// 提取 exit code
		if len(job.Status.Conditions) > 0 {
			for _, c := range job.Status.Conditions {
				if c.Type == batchv1.JobFailed && c.Status == "True" {
					if sr.ExitCode == nil {
						code := 1
						sr.ExitCode = &code
					}
					sr.Error = c.Message
				}
			}
		}
		// 從 Pod status 提取更精確的 exit code
		if exitCode := w.extractExitCode(job); exitCode != nil {
			sr.ExitCode = exitCode
		}
	}

	logger.Info("step run status updated",
		"step_run_id", sr.ID,
		"step_name", sr.StepName,
		"old_status", oldStatus,
		"new_status", newStatus,
		"job_name", sr.JobName,
	)
	return true
}

func (w *JobWatcher) jobToStepStatus(job *batchv1.Job) string {
	// Job completed successfully
	if job.Status.Succeeded > 0 {
		return models.StepRunStatusSuccess
	}

	// Job failed
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == "True" {
			return models.StepRunStatusFailed
		}
	}

	// Job still active
	if job.Status.Active > 0 {
		return models.StepRunStatusRunning
	}

	// Job created but no pods yet
	return models.StepRunStatusPending
}

func (w *JobWatcher) extractExitCode(job *batchv1.Job) *int {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == "True" {
			code := 1
			if c.Reason == "BackoffLimitExceeded" {
				code = 137 // 通常是 OOMKilled
			}
			return &code
		}
	}
	return nil
}

// extractExitCodeFromPod 從 Pod containerStatuses 取得精確 exit code。
func (w *JobWatcher) extractExitCodeFromPod(ctx context.Context, clusterID uint, sr *models.StepRun) *int {
	if sr.JobName == "" || sr.JobNamespace == "" {
		return nil
	}
	k8sClient := w.provider.GetK8sClientByID(clusterID)
	if k8sClient == nil {
		return nil
	}

	podCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	podList, err := k8sClient.GetClientset().CoreV1().
		Pods(sr.JobNamespace).
		List(podCtx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", sr.JobName),
		})
	if err != nil || len(podList.Items) == 0 {
		return nil
	}

	for _, cs := range podList.Items[0].Status.ContainerStatuses {
		if cs.Name == "step" && cs.State.Terminated != nil {
			code := int(cs.State.Terminated.ExitCode)
			return &code
		}
	}
	return nil
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
	gracePeriod := w.cfg.TerminationGracePeriod

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
// Log 收集
// ---------------------------------------------------------------------------

// collectStepLogs 從 K8s Pod 收集 Step 的執行 log 並持久化。
func (w *JobWatcher) collectStepLogs(ctx context.Context, run *models.PipelineRun, sr *models.StepRun) {
	if sr.JobName == "" || sr.JobNamespace == "" {
		return
	}

	k8sClient := w.provider.GetK8sClientByID(run.ClusterID)
	if k8sClient == nil {
		logger.Warn("cannot collect logs: no k8s client",
			"cluster_id", run.ClusterID, "step_run_id", sr.ID)
		return
	}

	logCtx, logCancel := context.WithTimeout(ctx, 30*time.Second)
	defer logCancel()

	// 查找 Job 的 Pod
	podList, err := k8sClient.GetClientset().CoreV1().
		Pods(sr.JobNamespace).
		List(logCtx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", sr.JobName),
		})
	if err != nil || len(podList.Items) == 0 {
		logger.Warn("no pods found for step job, log unavailable",
			"job_name", sr.JobName, "step_run_id", sr.ID, "error", err)
		// 記錄 log unavailable
		_ = w.logSvc.AppendLog(logCtx, run.ID, sr.ID,
			"[synapse] log unavailable: pod not found or already cleaned up\n", nil)
		return
	}

	// 取第一個 Pod 的 log（Job 預設 parallelism=1）
	pod := podList.Items[0]
	logStream, err := k8sClient.GetClientset().CoreV1().
		Pods(sr.JobNamespace).
		GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: "step",
		}).Stream(logCtx)
	if err != nil {
		logger.Warn("failed to get pod logs",
			"pod_name", pod.Name, "step_run_id", sr.ID, "error", err)
		_ = w.logSvc.AppendLog(logCtx, run.ID, sr.ID,
			fmt.Sprintf("[synapse] log retrieval failed: %v\n", err), nil)
		return
	}
	defer logStream.Close()

	// 讀取全部 log 並持久化
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, logStream); err != nil {
		logger.Warn("error reading pod log stream",
			"pod_name", pod.Name, "error", err)
	}

	if buf.Len() > 0 {
		// TODO: 傳入 secrets 列表做 scrubbing（需從 PipelineSecretService 取得）
		if err := w.logSvc.AppendLog(logCtx, run.ID, sr.ID, buf.String(), nil); err != nil {
			logger.Error("failed to persist step logs",
				"step_run_id", sr.ID, "error", err)
		} else {
			logger.Info("step logs persisted",
				"step_run_id", sr.ID,
				"log_size", buf.Len(),
			)
		}
	}
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
