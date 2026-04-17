package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// StepDef — Step 定義（從 PipelineVersion.StepsJSON 解析）
// ---------------------------------------------------------------------------

// StepDef 從 PipelineVersion.StepsJSON 解析的 Step 定義。
type StepDef struct {
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	Image        string              `json:"image"`
	Command      string              `json:"command"`
	DependsOn    []string            `json:"depends_on"`
	Config       json.RawMessage     `json:"config"`        // raw JSON for StepConfig (object or string)
	MaxRetries   int                 `json:"max_retries"`   // 0 = no retry (default)
	RetryBackoff string              `json:"retry_backoff"` // "fixed" or "exponential" (default: "exponential")
	Matrix       map[string][]string `json:"matrix"`        // 矩陣展開
}

// ConfigString returns Config as a JSON string for DB storage.
func (s *StepDef) ConfigString() string {
	return string(s.Config)
}

// ---------------------------------------------------------------------------
// DAG 執行（非同步）
// ---------------------------------------------------------------------------

// executeRunAsync 非同步執行 Pipeline Run 的 Steps DAG。
func (s *PipelineScheduler) executeRunAsync(run *models.PipelineRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	var version models.PipelineVersion
	if err := s.db.WithContext(ctx).First(&version, run.SnapshotID).Error; err != nil {
		s.failRun(ctx, run, fmt.Sprintf("load version snapshot: %v", err))
		return
	}

	var steps []StepDef
	if err := json.Unmarshal([]byte(version.StepsJSON), &steps); err != nil {
		s.failRun(ctx, run, fmt.Sprintf("parse steps JSON: %v", err))
		return
	}

	for i := range steps {
		if err := ValidateStepDef(&steps[i]); err != nil {
			s.failRun(ctx, run, fmt.Sprintf("validate step: %v", err))
			return
		}
		steps[i].Image = ResolveImage(&steps[i])
	}

	sorted, err := topoSortSteps(steps)
	if err != nil {
		s.failRun(ctx, run, fmt.Sprintf("topological sort: %v", err))
		return
	}

	originalStepResults := s.loadOriginalStepResults(ctx, run)
	rollbackArtifacts := s.loadRollbackArtifacts(ctx, run)

	// 建立 StepRun 記錄（Transaction 保證原子性）
	stepRuns := make(map[string]*models.StepRun, len(sorted))
	if txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, step := range sorted {
			dependsOnJSON, _ := json.Marshal(step.DependsOn)

			initialStatus := models.StepRunStatusPending
			if originalStepResults != nil {
				if origStatus, ok := originalStepResults[step.Name]; ok && origStatus == models.StepRunStatusSuccess {
					if run.RerunFromStep == "" || s.isBeforeStep(sorted, step.Name, run.RerunFromStep) {
						initialStatus = models.StepRunStatusSkipped
					}
				}
			}

			if rollbackArtifacts != nil && initialStatus == models.StepRunStatusPending {
				if !IsDeployStepType(step.Type) {
					initialStatus = models.StepRunStatusSkipped
				}
			}

			imageRef := step.Image
			if rollbackArtifacts != nil {
				if oldImage, ok := rollbackArtifacts[step.Name]; ok && oldImage != "" {
					imageRef = oldImage
				}
			}

			sr := &models.StepRun{
				PipelineRunID: run.ID,
				StepName:      step.Name,
				StepType:      step.Type,
				StepIndex:     i,
				Status:        initialStatus,
				Image:         imageRef,
				Command:       step.Command,
				ConfigJSON:    step.ConfigString(),
				DependsOn:     string(dependsOnJSON),
				MaxRetries:    step.MaxRetries,
			}
			if err := tx.Create(sr).Error; err != nil {
				return fmt.Errorf("create step run %s: %w", step.Name, err)
			}
			stepRuns[step.Name] = sr
		}
		return nil
	}); txErr != nil {
		s.failRun(ctx, run, fmt.Sprintf("create step runs: %v", txErr))
		return
	}

	logger.Info("pipeline run DAG initialized",
		"run_id", run.ID,
		"step_count", len(sorted),
	)

	if s.k8sProvider.GetK8sClientByID(run.ClusterID) == nil {
		s.failRun(ctx, run, fmt.Sprintf("cluster %d: k8s client not available", run.ClusterID))
		return
	}

	anyFailed := false
	for _, step := range sorted {
		sr := stepRuns[step.Name]

		if sr.Status == models.StepRunStatusSkipped {
			continue
		}

		if err := s.db.WithContext(ctx).First(run, run.ID).Error; err != nil {
			return
		}
		if run.Status == models.PipelineRunStatusCancelling || run.Status == models.PipelineRunStatusCancelled {
			s.cancelRemainingSteps(ctx, stepRuns)
			return
		}

		if !s.allDependenciesMet(stepRuns, step.DependsOn) {
			sr.Status = models.StepRunStatusSkipped
			if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
				logger.Error("failed to save skipped step run", "step_run_id", sr.ID, "error", err)
			}
			continue
		}

		if step.Type == "approval" {
			if failed := s.executeApprovalStep(ctx, run, sr); failed {
				anyFailed = true
			}
			continue
		}

		if IsMatrixStep(step) {
			if failed := s.executeMatrixStep(ctx, run, step, sr); failed {
				anyFailed = true
			}
			continue
		}

		if failed := s.executeStepWithRetry(ctx, run, sr, step); failed {
			anyFailed = true
		}
	}

	if s.watcher != nil {
		s.watcher.WatchRun(run)
	}

	finishNow := time.Now()
	run.FinishedAt = &finishNow
	if anyFailed {
		run.Status = models.PipelineRunStatusFailed
	} else {
		run.Status = models.PipelineRunStatusSuccess
	}
	if err := s.db.WithContext(ctx).Save(run).Error; err != nil {
		logger.Error("failed to save completed run", "run_id", run.ID, "error", err)
	}

	logger.Info("pipeline run completed",
		"run_id", run.ID,
		"status", run.Status,
		"trigger_type", run.TriggerType,
	)

	if run.TriggerType == models.TriggerTypeRollback && run.Status == models.PipelineRunStatusSuccess {
		s.copyArtifactsToRollbackRun(ctx, run, stepRuns)
	}

	s.notifyRunCompletion(ctx, run)
}

// ---------------------------------------------------------------------------
// DAG helpers
// ---------------------------------------------------------------------------

func (s *PipelineScheduler) allDependenciesMet(stepRuns map[string]*models.StepRun, deps []string) bool {
	for _, dep := range deps {
		sr, ok := stepRuns[dep]
		if !ok {
			return false
		}
		if sr.Status != models.StepRunStatusSuccess && sr.Status != models.StepRunStatusSkipped {
			return false
		}
	}
	return true
}

// waitForStep 輪詢等待 Step 完成（由 Watcher 更新 DB 狀態）。
func (s *PipelineScheduler) waitForStep(ctx context.Context, sr *models.StepRun) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return fmt.Errorf("scheduler stopped")
		case <-ticker.C:
			if err := s.db.WithContext(ctx).First(sr, sr.ID).Error; err != nil {
				return fmt.Errorf("reload step run %d: %w", sr.ID, err)
			}
			if isTerminalStepStatus(sr.Status) {
				return nil
			}
		}
	}
}

func (s *PipelineScheduler) failRun(ctx context.Context, run *models.PipelineRun, errMsg string) {
	now := time.Now()
	run.Status = models.PipelineRunStatusFailed
	run.Error = errMsg
	run.FinishedAt = &now
	if err := s.db.WithContext(ctx).Save(run).Error; err != nil {
		logger.Error("failed to update run status to failed",
			"run_id", run.ID, "error", err)
	}
	logger.Error("pipeline run failed", "run_id", run.ID, "error", errMsg)
}

func (s *PipelineScheduler) cancelRemainingSteps(ctx context.Context, stepRuns map[string]*models.StepRun) {
	for _, sr := range stepRuns {
		if sr.Status == models.StepRunStatusPending || sr.Status == models.StepRunStatusRunning {
			sr.Status = models.StepRunStatusCancelled
			s.db.WithContext(ctx).Save(sr)
		}
	}
}

// ---------------------------------------------------------------------------
// 通知
// ---------------------------------------------------------------------------

// notifyRunCompletion 在 Run 完成後發送通知到配置的 channels。
func (s *PipelineScheduler) notifyRunCompletion(ctx context.Context, run *models.PipelineRun) {
	if s.notifier == nil {
		return
	}

	var eventType string
	switch run.Status {
	case models.PipelineRunStatusSuccess:
		eventType = "run_success"
	case models.PipelineRunStatusFailed:
		eventType = "run_failed"
	default:
		return
	}

	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).Select("id, name").First(&pipeline, run.PipelineID).Error; err != nil {
		logger.Warn("notify: failed to load pipeline name", "pipeline_id", run.PipelineID, "error", err)
		return
	}

	var duration time.Duration
	if run.StartedAt != nil && run.FinishedAt != nil {
		duration = run.FinishedAt.Sub(*run.StartedAt)
	}

	event := &PipelineEvent{
		Type:         eventType,
		PipelineName: pipeline.Name,
		PipelineID:   run.PipelineID,
		RunID:        run.ID,
		Namespace:    run.Namespace,
		TriggerType:  run.TriggerType,
		Error:        run.Error,
		Duration:     duration,
	}

	s.notifier.Notify(ctx, event)
}
