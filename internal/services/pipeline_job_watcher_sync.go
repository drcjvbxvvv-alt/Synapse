package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

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
// Rollout 狀態補充
// ---------------------------------------------------------------------------

// enrichRolloutFields 針對 deploy-rollout / rollout-status Step，查詢 Argo Rollout CRD
// 並寫入 StepRun.RolloutStatus 和 StepRun.RolloutWeight。
func (w *JobWatcher) enrichRolloutFields(ctx context.Context, sr *models.StepRun, clusterID uint) {
	if w.rolloutSvc == nil {
		return
	}

	// 從 ConfigJSON 解析 rollout_name + namespace
	rolloutName, namespace := w.parseRolloutTarget(sr)
	if rolloutName == "" || namespace == "" {
		return
	}

	k8sClient := w.provider.GetK8sClientByID(clusterID)
	if k8sClient == nil {
		return
	}

	rolloutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dynClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		logger.Warn("failed to create dynamic client for rollout enrichment",
			"step_run_id", sr.ID, "error", err)
		return
	}

	info, err := w.rolloutSvc.GetRollout(rolloutCtx, dynClient, namespace, rolloutName)
	if err != nil {
		logger.Warn("failed to enrich rollout fields",
			"step_run_id", sr.ID,
			"rollout", rolloutName,
			"namespace", namespace,
			"error", err,
		)
		return
	}

	sr.RolloutStatus = info.Status
	weight := int(info.CanaryWeight)
	sr.RolloutWeight = &weight

	logger.Info("rollout fields enriched",
		"step_run_id", sr.ID,
		"rollout_status", info.Status,
		"rollout_weight", weight,
	)
}

// parseRolloutTarget 從 StepRun.ConfigJSON 解析 rollout 的 name 和 namespace。
// 支援 deploy-rollout 和 rollout-status Step 類型。
func (w *JobWatcher) parseRolloutTarget(sr *models.StepRun) (name, namespace string) {
	if sr.ConfigJSON == "" {
		return "", ""
	}

	switch sr.StepType {
	case "deploy-rollout":
		var cfg DeployRolloutConfig
		if err := json.Unmarshal([]byte(sr.ConfigJSON), &cfg); err == nil {
			return cfg.RolloutName, cfg.Namespace
		}
	case "rollout-status":
		var cfg RolloutStatusConfig
		if err := json.Unmarshal([]byte(sr.ConfigJSON), &cfg); err == nil {
			return cfg.RolloutName, cfg.Namespace
		}
	}
	return "", ""
}
