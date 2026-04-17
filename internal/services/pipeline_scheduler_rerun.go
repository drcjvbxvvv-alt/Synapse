package services

import (
	"context"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Rerun-from-failed helpers
// ---------------------------------------------------------------------------

// loadOriginalStepResults 載入 rerun 原始 Run 的 Step 狀態對照表。
// 僅在 RerunFromID 有值且 TriggerType=rerun 時載入。
func (s *PipelineScheduler) loadOriginalStepResults(ctx context.Context, run *models.PipelineRun) map[string]string {
	if run.RerunFromID == nil || run.TriggerType != models.TriggerTypeRerun {
		return nil
	}

	var origSteps []models.StepRun
	if err := s.db.WithContext(ctx).
		Select("step_name, status").
		Where("pipeline_run_id = ?", *run.RerunFromID).
		Find(&origSteps).Error; err != nil {
		logger.Warn("failed to load original run step results for rerun",
			"rerun_from_id", *run.RerunFromID, "error", err)
		return nil
	}

	results := make(map[string]string, len(origSteps))
	for _, sr := range origSteps {
		results[sr.StepName] = sr.Status
	}
	return results
}

// isBeforeStep 判斷 stepName 在拓撲排序中是否在 targetStep 之前。
func (s *PipelineScheduler) isBeforeStep(sorted []StepDef, stepName, targetStep string) bool {
	for _, step := range sorted {
		if step.Name == targetStep {
			return true
		}
		if step.Name == stepName {
			return true
		}
	}
	return false
}
