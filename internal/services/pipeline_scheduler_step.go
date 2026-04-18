package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Step 執行（含 retry）
// ---------------------------------------------------------------------------

// injectInsecureFlag 將 insecure: true 注入 step config JSON。
func injectInsecureFlag(configJSON string) string {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return configJSON
	}
	cfg["insecure"] = true
	out, err := json.Marshal(cfg)
	if err != nil {
		return configJSON
	}
	return string(out)
}

// executeStepWithRetry 執行單一 Step，失敗時根據 RetryPolicy 重試。
// 回傳 true 表示最終失敗。
func (s *PipelineScheduler) executeStepWithRetry(
	ctx context.Context,
	run *models.PipelineRun,
	sr *models.StepRun,
	step StepDef,
) bool {
	policy := NewRetryPolicy(sr.MaxRetries, step.RetryBackoff)

	for attempt := 0; ; attempt++ {
		// 解析 ${{ secrets.* }}
		secrets, err := s.resolveSecrets(ctx, run.PipelineID, sr.ConfigJSON)
		if err != nil {
			sr.Status = models.StepRunStatusFailed
			sr.Error = fmt.Sprintf("resolve secrets: %v", err)
			now := time.Now()
			sr.FinishedAt = &now
			if saveErr := s.db.WithContext(ctx).Save(sr).Error; saveErr != nil {
				logger.Error("failed to save failed step run", "step_run_id", sr.ID, "error", saveErr)
			}
			return true // secret 解析失敗不重試
		}

		// push-image / build-image: 自動注入 Registry 認證
		s.injectRegistryCredentials(ctx, step.Type, sr.ConfigJSON, secrets)

		// 標記 running + 提交 K8s Job
		now := time.Now()
		sr.Status = models.StepRunStatusRunning
		sr.StartedAt = &now
		sr.RetryCount = attempt
		sr.Error = ""
		if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
			logger.Error("failed to save running step run", "step_run_id", sr.ID, "error", err)
		}

		imagePullSecretName := s.resolveImagePullSecret(ctx, run, sr)

		// Resolve git repo info from Pipeline → Project → GitProvider
		gitRepoURL, gitBranch, gitToken := s.resolveGitInfo(ctx, run.PipelineID)

		// build-image: 為 Kaniko 建構 docker config.json + 自動注入 insecure flag
		var dockerConfigJSON string
		if step.Type == "build-image" {
			dockerConfigJSON = s.buildDockerConfigJSON(secrets)
			// Registry 設定 InsecureTLS → 自動在 step config 加上 insecure: true
			if secrets["REGISTRY_INSECURE"] == "true" {
				sr.ConfigJSON = injectInsecureFlag(sr.ConfigJSON)
			}
		}

		input := &BuildJobInput{
			Run:                 run,
			StepRun:             sr,
			Namespace:           run.Namespace,
			Secrets:             secrets,
			ImagePullSecretName: imagePullSecretName,
			GitRepoURL:          gitRepoURL,
			GitBranch:           gitBranch,
			GitToken:            gitToken,
			DockerConfigJSON:    dockerConfigJSON,
		}

		submitErr := s.jobBuilder.SubmitJob(ctx, s.k8sProvider.GetK8sClientByID(run.ClusterID), input)
		if submitErr != nil {
			sr.Status = models.StepRunStatusFailed
			sr.Error = fmt.Sprintf("submit k8s job: %v", submitErr)
			finishedNow := time.Now()
			sr.FinishedAt = &finishedNow
			if saveErr := s.db.WithContext(ctx).Save(sr).Error; saveErr != nil {
				logger.Error("failed to save failed step run", "step_run_id", sr.ID, "error", saveErr)
			}
			logger.Error("step job submission failed",
				"step_run_id", sr.ID,
				"step_name", sr.StepName,
				"attempt", attempt,
				"error", submitErr,
			)
			if policy.ShouldRetry(attempt) {
				delay := policy.Delay(attempt)
				logger.Info("retrying step after backoff",
					"step_run_id", sr.ID,
					"step_name", sr.StepName,
					"attempt", attempt+1,
					"max_retries", policy.MaxRetries,
					"delay", delay,
				)
				select {
				case <-ctx.Done():
					return true
				case <-s.stopCh:
					return true
				case <-time.After(delay):
				}
				continue
			}
			return true
		}

		if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
			logger.Error("failed to save step run job info", "step_run_id", sr.ID, "error", err)
		}

		logger.Info("step job submitted",
			"step_run_id", sr.ID,
			"step_name", sr.StepName,
			"job_name", sr.JobName,
			"run_id", run.ID,
			"attempt", attempt,
		)

		if err := s.waitForStep(ctx, sr); err != nil {
			return true
		}

		if sr.Status == models.StepRunStatusSuccess {
			return false
		}

		if sr.Status == models.StepRunStatusFailed && policy.ShouldRetry(attempt) {
			delay := policy.Delay(attempt)
			logger.Info("retrying failed step after backoff",
				"step_run_id", sr.ID,
				"step_name", sr.StepName,
				"attempt", attempt+1,
				"max_retries", policy.MaxRetries,
				"delay", delay,
			)
			sr.Status = models.StepRunStatusPending
			sr.FinishedAt = nil
			sr.JobName = ""
			sr.JobNamespace = ""
			sr.ExitCode = nil
			if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
				logger.Error("failed to reset step run for retry", "step_run_id", sr.ID, "error", err)
			}
			select {
			case <-ctx.Done():
				return true
			case <-s.stopCh:
				return true
			case <-time.After(delay):
			}
			continue
		}

		return sr.Status == models.StepRunStatusFailed
	}
}
