package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	// 啟動 Watcher：必須在第一個 Job 提交前啟動，
	// 否則 waitForStep 等不到 DB 狀態更新
	if s.watcher != nil {
		s.watcher.WatchRun(run)
	}

	// 檢查 Pipeline 開關
	pipelineFlags := s.getPipelineFlags(ctx, run.PipelineID)

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

		// 部署審核：deploy 類 step 前自動插入 approval gate
		if pipelineFlags.ApprovalEnabled && IsDeployStepType(step.Type) {
			approvalSR := s.createAutoApprovalStep(ctx, run, sr)
			if approvalSR != nil {
				if failed := s.executeApprovalStep(ctx, run, approvalSR); failed {
					anyFailed = true
					// 審核被拒 → 跳過 deploy step
					sr.Status = models.StepRunStatusSkipped
					sr.Error = "deploy skipped: approval rejected"
					now := time.Now()
					sr.FinishedAt = &now
					s.db.WithContext(ctx).Save(sr)
					continue
				}
			}
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

		// 安全掃描：build-image 成功後自動插入 trivy-scan（不阻斷流程）
		if pipelineFlags.ScanEnabled && step.Type == "build-image" {
			// 重新從 DB 載入最新狀態（waitForStep 可能已更新）
			s.db.WithContext(ctx).First(sr, sr.ID)
			logger.Info("scan check after build-image",
				"step_status", sr.Status, "step_name", sr.StepName, "run_id", run.ID)
			if sr.Status == models.StepRunStatusSuccess {
				s.runAutoScan(ctx, run, sr, step)
			}
		}
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

// pipelineFlags 持有 Pipeline 級別的開關。
type pipelineFlags struct {
	ApprovalEnabled bool
	ScanEnabled     bool
}

// getPipelineFlags 讀取 Pipeline 的開關設定。
func (s *PipelineScheduler) getPipelineFlags(ctx context.Context, pipelineID uint) pipelineFlags {
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).
		Select("approval_enabled, scan_enabled").
		First(&pipeline, pipelineID).Error; err != nil {
		return pipelineFlags{}
	}
	return pipelineFlags{
		ApprovalEnabled: pipeline.ApprovalEnabled,
		ScanEnabled:     pipeline.ScanEnabled,
	}
}

// runAutoScan 在 build-image 成功後自動執行 trivy-scan（不阻斷流程）。
func (s *PipelineScheduler) runAutoScan(ctx context.Context, run *models.PipelineRun, buildSR *models.StepRun, buildStep StepDef) {
	// 從 build-image config 取得 destination 作為掃描目標
	var buildCfg BuildImageConfig
	if err := parseJSON(buildStep.Config, &buildCfg); err != nil || buildCfg.Destination == "" {
		logger.Warn("auto scan skipped: cannot parse build config",
			"step_run_id", buildSR.ID)
		return
	}

	scanSR := &models.StepRun{
		PipelineRunID: run.ID,
		StepName:      fmt.Sprintf("scan-%s", buildSR.StepName),
		StepType:      "trivy-scan",
		StepIndex:     buildSR.StepIndex,
		Status:        models.StepRunStatusPending,
		Image:         "aquasec/trivy:0.58.0",
		ConfigJSON:    fmt.Sprintf(`{"image":%q,"exit_code":0,"registry":%q,"format":"json"}`, buildCfg.Destination, buildCfg.Registry),
		DependsOn:     "[]",
	}
	if err := s.db.WithContext(ctx).Create(scanSR).Error; err != nil {
		logger.Error("failed to create auto scan step",
			"run_id", run.ID, "error", err)
		return
	}

	logger.Info("auto trivy scan started after build",
		"scan_step_id", scanSR.ID,
		"target_image", buildCfg.Destination,
		"run_id", run.ID,
	)

	scanStep := StepDef{
		Name:   scanSR.StepName,
		Type:   "trivy-scan",
		Image:  scanSR.Image,
		Config: json.RawMessage(scanSR.ConfigJSON),
	}

	// 確保 run 狀態是 running（watcher 可能已提前 finalize）
	run.Status = models.PipelineRunStatusRunning
	run.FinishedAt = nil
	s.db.WithContext(ctx).Save(run)

	// 重新啟動 watcher 追蹤掃描 step 的 Job 狀態
	if s.watcher != nil {
		s.watcher.WatchRun(run)
	}

	// 執行掃描（exit_code=0 → 不會因漏洞而失敗）
	s.executeStepWithRetry(ctx, run, scanSR, scanStep)

	// 等待 watcher 收集日誌後再解析掃描結果
	for i := 0; i < 10; i++ {
		var logCount int64
		s.db.WithContext(ctx).Model(&models.PipelineLog{}).
			Where("step_run_id = ?", scanSR.ID).Count(&logCount)
		if logCount > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	s.parseScanResults(ctx, run, scanSR, buildCfg.Destination)

	// 掃描完成 → 重新 finalize run（掃描結果不影響成敗）
	now := time.Now()
	run.FinishedAt = &now
	run.Status = models.PipelineRunStatusSuccess
	s.db.WithContext(ctx).Save(run)
	logger.Info("pipeline run finalized after scan", "run_id", run.ID)
}

// parseScanResults 解析 Trivy JSON 輸出並寫入 image_scan_results 表。
func (s *PipelineScheduler) parseScanResults(ctx context.Context, run *models.PipelineRun, scanSR *models.StepRun, image string) {
	// 從 DB 讀取掃描日誌
	var logs []models.PipelineLog
	s.db.WithContext(ctx).
		Where("step_run_id = ?", scanSR.ID).
		Order("chunk_seq ASC").
		Find(&logs)

	if len(logs) == 0 {
		return
	}

	// 合併所有 chunk
	var fullLog string
	for _, l := range logs {
		fullLog += l.Content
	}

	// 解析 Trivy JSON 輸出
	critical, high, medium, low, unknown := parseTrivyCounts(fullLog)

	now := time.Now()
	scanResult := &models.ImageScanResult{
		ClusterID:     run.ClusterID,
		Namespace:     run.Namespace,
		Image:         image,
		Status:        "completed",
		Critical:      critical,
		High:          high,
		Medium:        medium,
		Low:           low,
		Unknown:       unknown,
		ScannedAt:     &now,
		ScanSource:    "pipeline",
		PipelineRunID: &run.ID,
		StepRunID:     &scanSR.ID,
	}

	if err := s.db.WithContext(ctx).Create(scanResult).Error; err != nil {
		logger.Error("failed to save scan result", "run_id", run.ID, "error", err)
		return
	}

	// 回寫 scan_result_id 到 step_run
	scanSR.ScanResultID = &scanResult.ID
	s.db.WithContext(ctx).Save(scanSR)

	logger.Info("scan results saved",
		"run_id", run.ID,
		"scan_result_id", scanResult.ID,
		"critical", critical, "high", high, "medium", medium, "low", low,
	)
}

// parseTrivyCounts 從 Trivy JSON 輸出提取漏洞計數。
func parseTrivyCounts(trivyOutput string) (critical, high, medium, low, unknown int) {
	// Trivy JSON 格式: { "Results": [{ "Vulnerabilities": [{ "Severity": "CRITICAL" }, ...] }] }
	var report struct {
		Results []struct {
			Vulnerabilities []struct {
				Severity string `json:"Severity"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	// Trivy 輸出可能夾雜進度條和 INFO 日誌，提取 JSON 部分
	jsonStart := strings.Index(trivyOutput, "{")
	jsonEnd := strings.LastIndex(trivyOutput, "}")
	jsonContent := trivyOutput
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonContent = trivyOutput[jsonStart : jsonEnd+1]
	}

	if err := json.Unmarshal([]byte(jsonContent), &report); err != nil {
		// 可能是 table 格式或非 JSON，嘗試從文字計數
		critical = strings.Count(trivyOutput, "CRITICAL")
		high = strings.Count(trivyOutput, "HIGH")
		medium = strings.Count(trivyOutput, "MEDIUM")
		low = strings.Count(strings.ToUpper(trivyOutput), "LOW")
		return
	}

	for _, result := range report.Results {
		for _, vuln := range result.Vulnerabilities {
			switch strings.ToUpper(vuln.Severity) {
			case "CRITICAL":
				critical++
			case "HIGH":
				high++
			case "MEDIUM":
				medium++
			case "LOW":
				low++
			default:
				unknown++
			}
		}
	}
	return
}

// createAutoApprovalStep 為 deploy step 自動建立一個 approval StepRun。
func (s *PipelineScheduler) createAutoApprovalStep(ctx context.Context, run *models.PipelineRun, deploySR *models.StepRun) *models.StepRun {
	depsOn := deploySR.DependsOn
	if depsOn == "" {
		depsOn = "[]"
	}
	approvalSR := &models.StepRun{
		PipelineRunID: run.ID,
		StepName:      fmt.Sprintf("approve-%s", deploySR.StepName),
		StepType:      "approval",
		StepIndex:     deploySR.StepIndex,
		Status:        models.StepRunStatusPending,
		DependsOn:     depsOn,
	}
	if err := s.db.WithContext(ctx).Create(approvalSR).Error; err != nil {
		logger.Error("failed to create auto approval step",
			"run_id", run.ID, "deploy_step", deploySR.StepName, "error", err)
		return nil
	}
	logger.Info("auto approval step created before deploy",
		"approval_step_id", approvalSR.ID,
		"deploy_step", deploySR.StepName,
		"run_id", run.ID,
	)
	return approvalSR
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
