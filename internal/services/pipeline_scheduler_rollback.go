package services

import (
	"context"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Rollback helpers
// ---------------------------------------------------------------------------

// loadRollbackArtifacts 載入回滾來源 Run 的映像 Artifacts（rollback 用）。
// 回傳 map[step_name]image_reference，僅包含 kind="image" 的 artifacts。
// 若 run 不是 rollback 類型或找不到 artifacts，回傳 nil（不影響執行）。
func (s *PipelineScheduler) loadRollbackArtifacts(ctx context.Context, run *models.PipelineRun) map[string]string {
	if run.RollbackOfRunID == nil || run.TriggerType != models.TriggerTypeRollback {
		return nil
	}

	var stepRuns []models.StepRun
	if err := s.db.WithContext(ctx).
		Select("id, step_name").
		Where("pipeline_run_id = ?", *run.RollbackOfRunID).
		Find(&stepRuns).Error; err != nil {
		logger.Warn("failed to load step runs for rollback source",
			"rollback_of_run_id", *run.RollbackOfRunID, "error", err)
		return nil
	}
	stepNameByID := make(map[uint]string, len(stepRuns))
	for _, sr := range stepRuns {
		stepNameByID[sr.ID] = sr.StepName
	}

	var artifacts []models.PipelineArtifact
	if err := s.db.WithContext(ctx).
		Where("pipeline_run_id = ? AND kind = ?", *run.RollbackOfRunID, "image").
		Find(&artifacts).Error; err != nil {
		logger.Warn("failed to load artifacts for rollback source",
			"rollback_of_run_id", *run.RollbackOfRunID, "error", err)
		return nil
	}

	result := make(map[string]string, len(artifacts))
	for _, a := range artifacts {
		if stepName, ok := stepNameByID[a.StepRunID]; ok && a.Reference != "" {
			result[stepName] = a.Reference
		}
	}
	return result
}

// copyArtifactsToRollbackRun 將來源 Run 的 artifacts 複製到回滾 Run（供審計追蹤）。
func (s *PipelineScheduler) copyArtifactsToRollbackRun(ctx context.Context, rollbackRun *models.PipelineRun, stepRuns map[string]*models.StepRun) {
	if rollbackRun.RollbackOfRunID == nil {
		return
	}

	var srcArtifacts []models.PipelineArtifact
	if err := s.db.WithContext(ctx).
		Where("pipeline_run_id = ?", *rollbackRun.RollbackOfRunID).
		Find(&srcArtifacts).Error; err != nil {
		logger.Warn("failed to load source artifacts for copy",
			"rollback_of_run_id", *rollbackRun.RollbackOfRunID, "error", err)
		return
	}

	for _, src := range srcArtifacts {
		var stepRunID uint
		var origSR models.StepRun
		if err := s.db.WithContext(ctx).
			Select("step_name").First(&origSR, src.StepRunID).Error; err == nil {
			if sr, ok := stepRuns[origSR.StepName]; ok {
				stepRunID = sr.ID
			}
		}

		copied := models.PipelineArtifact{
			PipelineRunID: rollbackRun.ID,
			StepRunID:     stepRunID,
			Kind:          src.Kind,
			Name:          src.Name,
			Reference:     src.Reference,
			SizeBytes:     src.SizeBytes,
			MetadataJSON:  src.MetadataJSON,
		}
		if err := s.db.WithContext(ctx).Create(&copied).Error; err != nil {
			logger.Warn("failed to copy artifact for rollback run",
				"src_artifact_id", src.ID, "rollback_run_id", rollbackRun.ID, "error", err)
		}
	}
}
