package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// PipelineScheduler — 排程迴圈 + 三級並發控制
//
// 設計（CICD_ARCHITECTURE §7.8–§7.9）：
//   - 系統級 / 叢集級 / Pipeline 級三層並發上限
//   - Concurrency Group 策略：cancel_previous / queue / reject
//   - 佇列深度 > 系統上限 × 3 → reject 新 Run（飢餓預防）
//   - 單一活躍實例模式（M13a），不做分散式鎖
// ---------------------------------------------------------------------------

// SchedulerConfig 排程器可調參數。
type SchedulerConfig struct {
	TickInterval       time.Duration // 排程迴圈間隔，預設 1s
	SystemMaxRuns      int           // 系統級並發上限，預設 20
	ClusterMaxRuns     int           // 叢集級並發上限，預設 10
	QueueOverflowRatio int           // 佇列滿拒絕倍率，預設 3
}

// DefaultSchedulerConfig 傳回預設配置。
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		TickInterval:       1 * time.Second,
		SystemMaxRuns:      20,
		ClusterMaxRuns:     10,
		QueueOverflowRatio: 3,
	}
}

// PipelineK8sProvider 提供 Pipeline 排程所需的 K8s client（由 ClusterInformerManager 實作）。
type PipelineK8sProvider interface {
	GetK8sClientByID(clusterID uint) *K8sClient
}

// PipelineScheduler 管理 Pipeline Run 的排程與並發控制。
type PipelineScheduler struct {
	db            *gorm.DB
	jobBuilder    *JobBuilder
	secretSvc     *PipelineSecretService
	k8sProvider   PipelineK8sProvider
	watcher       *JobWatcher
	cfg           SchedulerConfig

	stopCh chan struct{}
	once   sync.Once
}

// NewPipelineScheduler 建立排程器。
func NewPipelineScheduler(
	db *gorm.DB,
	jobBuilder *JobBuilder,
	secretSvc *PipelineSecretService,
	k8sProvider PipelineK8sProvider,
	watcher *JobWatcher,
	cfg SchedulerConfig,
) *PipelineScheduler {
	return &PipelineScheduler{
		db:          db,
		jobBuilder:  jobBuilder,
		secretSvc:   secretSvc,
		k8sProvider: k8sProvider,
		watcher:     watcher,
		cfg:         cfg,
		stopCh:      make(chan struct{}),
	}
}

// Start 啟動排程迴圈（background goroutine）。
func (s *PipelineScheduler) Start() {
	go s.loop()
	logger.Info("pipeline scheduler started",
		"tick_interval", s.cfg.TickInterval,
		"system_max_runs", s.cfg.SystemMaxRuns,
		"cluster_max_runs", s.cfg.ClusterMaxRuns,
	)
}

// Stop 停止排程迴圈。
func (s *PipelineScheduler) Stop() {
	s.once.Do(func() {
		close(s.stopCh)
		logger.Info("pipeline scheduler stopped")
	})
}

// ---------------------------------------------------------------------------
// EnqueueRun — 建立新 Run 並處理 Concurrency Group
// ---------------------------------------------------------------------------

// EnqueueRun 建立 PipelineRun 並根據並發策略處理 Concurrency Group。
func (s *PipelineScheduler) EnqueueRun(ctx context.Context, run *models.PipelineRun) error {
	// 設定初始狀態
	run.Status = models.PipelineRunStatusQueued
	run.QueuedAt = time.Now()

	// 佇列滿檢查（飢餓預防）
	var queuedCount int64
	if err := s.db.WithContext(ctx).Model(&models.PipelineRun{}).
		Where("status = ?", models.PipelineRunStatusQueued).
		Count(&queuedCount).Error; err != nil {
		return fmt.Errorf("count queued runs: %w", err)
	}
	if int(queuedCount) >= s.cfg.SystemMaxRuns*s.cfg.QueueOverflowRatio {
		run.Status = models.PipelineRunStatusRejected
		run.Error = "queue overflow: too many queued runs, retry later"
		if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
			return fmt.Errorf("create rejected run: %w", err)
		}
		logger.Warn("pipeline run rejected due to queue overflow",
			"pipeline_id", run.PipelineID,
			"queued_count", queuedCount,
		)
		return nil
	}

	// Concurrency Group 策略
	if run.ConcurrencyGroup != "" {
		if err := s.applyConcurrencyPolicy(ctx, run); err != nil {
			return err
		}
		// reject 策略可能已把 status 改為 rejected
		if run.Status == models.PipelineRunStatusRejected {
			if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
				return fmt.Errorf("create rejected run: %w", err)
			}
			return nil
		}
	}

	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("create pipeline run: %w", err)
	}

	logger.Info("pipeline run enqueued",
		"run_id", run.ID,
		"pipeline_id", run.PipelineID,
		"concurrency_group", run.ConcurrencyGroup,
		"status", run.Status,
	)
	return nil
}

// ---------------------------------------------------------------------------
// CancelRun — 取消執行中或排隊中的 Run
// ---------------------------------------------------------------------------

// CancelRun 將 Run 標記為 cancelling，Watcher 負責清理 K8s Job。
func (s *PipelineScheduler) CancelRun(ctx context.Context, runID uint) error {
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("pipeline run %d not found", runID)
		}
		return fmt.Errorf("get pipeline run %d: %w", runID, err)
	}

	switch run.Status {
	case models.PipelineRunStatusQueued:
		// 排隊中直接取消
		now := time.Now()
		run.Status = models.PipelineRunStatusCancelled
		run.FinishedAt = &now
		if err := s.db.WithContext(ctx).Save(&run).Error; err != nil {
			return fmt.Errorf("cancel queued run %d: %w", runID, err)
		}
	case models.PipelineRunStatusRunning:
		// 執行中標記為 cancelling，Watcher 會處理
		run.Status = models.PipelineRunStatusCancelling
		if err := s.db.WithContext(ctx).Save(&run).Error; err != nil {
			return fmt.Errorf("cancel running run %d: %w", runID, err)
		}
	default:
		return fmt.Errorf("cannot cancel run %d in status %s", runID, run.Status)
	}

	logger.Info("pipeline run cancel requested",
		"run_id", runID,
		"previous_status", run.Status,
	)
	return nil
}

// ---------------------------------------------------------------------------
// Scheduler Loop
// ---------------------------------------------------------------------------

func (s *PipelineScheduler) loop() {
	ticker := time.NewTicker(s.cfg.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.tick(); err != nil {
				logger.Error("scheduler tick error", "error", err)
			}
		}
	}
}

func (s *PipelineScheduler) tick() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 取出所有 queued Runs，按 queued_at ASC（FIFO 公平排程）
	var queuedRuns []models.PipelineRun
	if err := s.db.WithContext(ctx).
		Where("status = ?", models.PipelineRunStatusQueued).
		Order("queued_at ASC").
		Find(&queuedRuns).Error; err != nil {
		return fmt.Errorf("fetch queued runs: %w", err)
	}

	if len(queuedRuns) == 0 {
		return nil
	}

	// 載入當前並發計數
	counts, err := s.loadConcurrencyCounts(ctx)
	if err != nil {
		return err
	}

	for i := range queuedRuns {
		run := &queuedRuns[i]

		// 三級並發檢查
		if !s.canSchedule(ctx, run, counts) {
			continue
		}

		// 可排程 → status: running
		now := time.Now()
		run.Status = models.PipelineRunStatusRunning
		run.StartedAt = &now
		if err := s.db.WithContext(ctx).Save(run).Error; err != nil {
			logger.Error("failed to start run", "run_id", run.ID, "error", err)
			continue
		}

		// 更新計數
		counts.system++
		counts.cluster[run.ClusterID]++
		counts.pipeline[run.PipelineID]++

		logger.Info("pipeline run started",
			"run_id", run.ID,
			"pipeline_id", run.PipelineID,
			"cluster_id", run.ClusterID,
		)

		// 非同步啟動 Steps DAG 執行（P0-5 JobWatcher 會接管）
		go s.executeRunAsync(run)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Concurrency counting
// ---------------------------------------------------------------------------

type concurrencyCounts struct {
	system   int
	cluster  map[uint]int
	pipeline map[uint]int
}

func (s *PipelineScheduler) loadConcurrencyCounts(ctx context.Context) (*concurrencyCounts, error) {
	counts := &concurrencyCounts{
		cluster:  make(map[uint]int),
		pipeline: make(map[uint]int),
	}

	// 統計所有 running 的 Runs
	var runningRuns []struct {
		PipelineID uint
		ClusterID  uint
	}
	if err := s.db.WithContext(ctx).
		Model(&models.PipelineRun{}).
		Select("pipeline_id, cluster_id").
		Where("status = ?", models.PipelineRunStatusRunning).
		Find(&runningRuns).Error; err != nil {
		return nil, fmt.Errorf("count running runs: %w", err)
	}

	counts.system = len(runningRuns)
	for _, r := range runningRuns {
		counts.cluster[r.ClusterID]++
		counts.pipeline[r.PipelineID]++
	}
	return counts, nil
}

func (s *PipelineScheduler) canSchedule(ctx context.Context, run *models.PipelineRun, counts *concurrencyCounts) bool {
	// 系統級
	if counts.system >= s.cfg.SystemMaxRuns {
		return false
	}
	// 叢集級
	if counts.cluster[run.ClusterID] >= s.cfg.ClusterMaxRuns {
		return false
	}
	// Pipeline 級（從 Pipeline 定義取 max_concurrent_runs）
	pipelineMax := s.getPipelineMaxConcurrent(ctx, run.PipelineID)
	if counts.pipeline[run.PipelineID] >= pipelineMax {
		return false
	}
	return true
}

func (s *PipelineScheduler) getPipelineMaxConcurrent(ctx context.Context, pipelineID uint) int {
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).Select("max_concurrent_runs").First(&pipeline, pipelineID).Error; err != nil {
		return 1 // fallback
	}
	if pipeline.MaxConcurrentRuns <= 0 {
		return 1
	}
	return pipeline.MaxConcurrentRuns
}

// ---------------------------------------------------------------------------
// Concurrency Group 策略
// ---------------------------------------------------------------------------

func (s *PipelineScheduler) applyConcurrencyPolicy(ctx context.Context, newRun *models.PipelineRun) error {
	// 查找同 group 中正在執行或排隊的 Runs
	var activeRuns []models.PipelineRun
	if err := s.db.WithContext(ctx).
		Where("concurrency_group = ? AND status IN ?",
			newRun.ConcurrencyGroup,
			[]string{models.PipelineRunStatusRunning, models.PipelineRunStatusQueued},
		).
		Order("queued_at ASC").
		Find(&activeRuns).Error; err != nil {
		return fmt.Errorf("find active runs in group %s: %w", newRun.ConcurrencyGroup, err)
	}

	if len(activeRuns) == 0 {
		return nil
	}

	// 取得 Pipeline 的 concurrency_policy
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).
		Select("concurrency_policy").
		First(&pipeline, newRun.PipelineID).Error; err != nil {
		return fmt.Errorf("get pipeline concurrency policy: %w", err)
	}

	switch pipeline.ConcurrencyPolicy {
	case models.ConcurrencyPolicyCancelPrevious:
		// 取消所有舊的 active Runs
		for i := range activeRuns {
			old := &activeRuns[i]
			if old.Status == models.PipelineRunStatusRunning {
				old.Status = models.PipelineRunStatusCancelling
			} else {
				now := time.Now()
				old.Status = models.PipelineRunStatusCancelled
				old.FinishedAt = &now
			}
			if err := s.db.WithContext(ctx).Save(old).Error; err != nil {
				logger.Error("failed to cancel previous run",
					"run_id", old.ID, "error", err)
			}
			logger.Info("previous run cancelled by concurrency policy",
				"cancelled_run_id", old.ID,
				"new_run_pipeline_id", newRun.PipelineID,
				"group", newRun.ConcurrencyGroup,
			)
		}

	case models.ConcurrencyPolicyQueue:
		// 不做任何處理，讓新 Run 排隊等待

	case models.ConcurrencyPolicyReject:
		// 拒絕新 Run
		newRun.Status = models.PipelineRunStatusRejected
		newRun.Error = fmt.Sprintf("rejected: concurrency group %q already has active runs", newRun.ConcurrencyGroup)
		logger.Info("pipeline run rejected by concurrency policy",
			"pipeline_id", newRun.PipelineID,
			"group", newRun.ConcurrencyGroup,
		)

	default:
		// 未知策略，fallback 為 cancel_previous
		logger.Warn("unknown concurrency policy, falling back to cancel_previous",
			"policy", pipeline.ConcurrencyPolicy)
	}

	return nil
}

// ---------------------------------------------------------------------------
// DAG 執行（非同步）
// ---------------------------------------------------------------------------

// StepDef 從 PipelineVersion.StepsJSON 解析的 Step 定義。
type StepDef struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Image     string   `json:"image"`
	Command   string   `json:"command"`
	DependsOn []string `json:"depends_on"`
	Config    string   `json:"config"` // raw JSON for StepConfig
}

// executeRunAsync 非同步執行 Pipeline Run 的 Steps DAG。
// 建立 StepRun 記錄、拓撲排序、依層提交 Job。
func (s *PipelineScheduler) executeRunAsync(run *models.PipelineRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	// 載入版本快照
	var version models.PipelineVersion
	if err := s.db.WithContext(ctx).First(&version, run.SnapshotID).Error; err != nil {
		s.failRun(ctx, run, fmt.Sprintf("load version snapshot: %v", err))
		return
	}

	// 解析 Steps DAG
	var steps []StepDef
	if err := json.Unmarshal([]byte(version.StepsJSON), &steps); err != nil {
		s.failRun(ctx, run, fmt.Sprintf("parse steps JSON: %v", err))
		return
	}

	// 拓撲排序
	sorted, err := topoSortSteps(steps)
	if err != nil {
		s.failRun(ctx, run, fmt.Sprintf("topological sort: %v", err))
		return
	}

	// 建立 StepRun 記錄
	stepRuns := make(map[string]*models.StepRun, len(sorted))
	for i, step := range sorted {
		dependsOnJSON, _ := json.Marshal(step.DependsOn)
		sr := &models.StepRun{
			PipelineRunID: run.ID,
			StepName:      step.Name,
			StepType:      step.Type,
			StepIndex:     i,
			Status:        models.StepRunStatusPending,
			Image:         step.Image,
			Command:       step.Command,
			ConfigJSON:    step.Config,
			DependsOn:     string(dependsOnJSON),
		}
		if err := s.db.WithContext(ctx).Create(sr).Error; err != nil {
			s.failRun(ctx, run, fmt.Sprintf("create step run %s: %v", step.Name, err))
			return
		}
		stepRuns[step.Name] = sr
	}

	logger.Info("pipeline run DAG initialized",
		"run_id", run.ID,
		"step_count", len(sorted),
	)

	// 取得 K8s client
	k8sClient := s.k8sProvider.GetK8sClientByID(run.ClusterID)
	if k8sClient == nil {
		s.failRun(ctx, run, fmt.Sprintf("cluster %d: k8s client not available", run.ClusterID))
		return
	}

	// 依層執行（M13a 同步逐步執行，後續 Milestone 可改為同層並行）
	anyFailed := false
	for _, step := range sorted {
		sr := stepRuns[step.Name]

		// 檢查 Run 是否被取消
		if err := s.db.WithContext(ctx).First(run, run.ID).Error; err != nil {
			return
		}
		if run.Status == models.PipelineRunStatusCancelling || run.Status == models.PipelineRunStatusCancelled {
			s.cancelRemainingSteps(ctx, stepRuns)
			return
		}

		// 檢查依賴是否全部成功
		if !s.allDependenciesMet(stepRuns, step.DependsOn) {
			sr.Status = models.StepRunStatusSkipped
			if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
				logger.Error("failed to save skipped step run", "step_run_id", sr.ID, "error", err)
			}
			continue
		}

		// 解析 ${{ secrets.* }} → 查詢 PipelineSecretService
		secrets, err := s.resolveSecrets(ctx, run.PipelineID, sr.ConfigJSON)
		if err != nil {
			sr.Status = models.StepRunStatusFailed
			sr.Error = fmt.Sprintf("resolve secrets: %v", err)
			now := time.Now()
			sr.FinishedAt = &now
			if saveErr := s.db.WithContext(ctx).Save(sr).Error; saveErr != nil {
				logger.Error("failed to save failed step run", "step_run_id", sr.ID, "error", saveErr)
			}
			anyFailed = true
			continue
		}

		// 標記 running + 提交 K8s Job
		now := time.Now()
		sr.Status = models.StepRunStatusRunning
		sr.StartedAt = &now
		if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
			logger.Error("failed to save running step run", "step_run_id", sr.ID, "error", err)
		}

		input := &BuildJobInput{
			Run:       run,
			StepRun:   sr,
			Namespace: run.Namespace,
			Secrets:   secrets,
		}

		if err := s.jobBuilder.SubmitJob(ctx, k8sClient, input); err != nil {
			sr.Status = models.StepRunStatusFailed
			sr.Error = fmt.Sprintf("submit k8s job: %v", err)
			finishedNow := time.Now()
			sr.FinishedAt = &finishedNow
			if saveErr := s.db.WithContext(ctx).Save(sr).Error; saveErr != nil {
				logger.Error("failed to save failed step run", "step_run_id", sr.ID, "error", saveErr)
			}
			anyFailed = true
			logger.Error("step job submission failed",
				"step_run_id", sr.ID,
				"step_name", sr.StepName,
				"error", err,
			)
			continue
		}

		// 回寫 Job 資訊
		if err := s.db.WithContext(ctx).Save(sr).Error; err != nil {
			logger.Error("failed to save step run job info", "step_run_id", sr.ID, "error", err)
		}

		logger.Info("step job submitted",
			"step_run_id", sr.ID,
			"step_name", sr.StepName,
			"job_name", sr.JobName,
			"run_id", run.ID,
		)

		// 等待此 Step 完成（輪詢 DB 狀態，由 Watcher 更新）
		if err := s.waitForStep(ctx, sr); err != nil {
			anyFailed = true
			continue
		}
		if sr.Status == models.StepRunStatusFailed {
			anyFailed = true
		}
	}

	// 啟動 Watcher 追蹤（如果尚未追蹤）
	if s.watcher != nil {
		s.watcher.WatchRun(run)
	}

	// 完結 Run
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
	)
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

// resolveSecrets 解析 Step 設定中的 ${{ secrets.* }} 引用，
// 從 PipelineSecretService 查詢實際值。
func (s *PipelineScheduler) resolveSecrets(ctx context.Context, pipelineID uint, configJSON string) (map[string]string, error) {
	if configJSON == "" || s.secretSvc == nil {
		return nil, nil
	}

	var cfg StepConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, nil // 無法解析時不報錯，由 JobBuilder 處理
	}

	resolved := make(map[string]string)
	for k, v := range cfg.Env {
		secretName, ok := parseSecretRef(v)
		if !ok {
			resolved[k] = v
			continue
		}
		// 查詢 secret：先查 pipeline scope，再查 global scope
		secretVal, err := s.lookupSecret(ctx, pipelineID, secretName)
		if err != nil {
			return nil, fmt.Errorf("secret %q: %w", secretName, err)
		}
		resolved[k] = secretVal
	}
	return resolved, nil
}

// parseSecretRef 解析 ${{ secrets.NAME }} 格式，回傳 secret name 和是否匹配。
func parseSecretRef(value string) (string, bool) {
	// 支援 ${{ secrets.NAME }} 和 ${{secrets.NAME}}
	v := strings.TrimSpace(value)
	if !strings.HasPrefix(v, "${{") || !strings.HasSuffix(v, "}}") {
		return "", false
	}
	inner := strings.TrimSpace(v[3 : len(v)-2])
	if !strings.HasPrefix(inner, "secrets.") {
		return "", false
	}
	name := strings.TrimSpace(inner[8:])
	if name == "" {
		return "", false
	}
	return name, true
}

// lookupSecret 依優先順序查詢 secret：pipeline → cluster → global。
func (s *PipelineScheduler) lookupSecret(ctx context.Context, pipelineID uint, name string) (string, error) {
	// 先查 pipeline scope
	secrets, err := s.secretSvc.ListSecrets(ctx, "pipeline", &pipelineID)
	if err != nil {
		return "", fmt.Errorf("list pipeline secrets: %w", err)
	}
	for _, sec := range secrets {
		if sec.Name == name {
			// ListSecrets 不回傳 ValueEnc，需要用 GetSecret 取得完整記錄
			full, err := s.secretSvc.GetSecret(ctx, sec.ID)
			if err != nil {
				return "", err
			}
			return full.ValueEnc, nil // AfterFind hook 已解密
		}
	}

	// 再查 global scope
	secrets, err = s.secretSvc.ListSecrets(ctx, "global", nil)
	if err != nil {
		return "", fmt.Errorf("list global secrets: %w", err)
	}
	for _, sec := range secrets {
		if sec.Name == name {
			full, err := s.secretSvc.GetSecret(ctx, sec.ID)
			if err != nil {
				return "", err
			}
			return full.ValueEnc, nil
		}
	}

	return "", fmt.Errorf("secret %q not found in pipeline or global scope", name)
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

func (s *PipelineScheduler) allDependenciesMet(stepRuns map[string]*models.StepRun, deps []string) bool {
	for _, dep := range deps {
		sr, ok := stepRuns[dep]
		if !ok || sr.Status != models.StepRunStatusSuccess {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// 拓撲排序（Kahn's algorithm）
// ---------------------------------------------------------------------------

func topoSortSteps(steps []StepDef) ([]StepDef, error) {
	byName := make(map[string]*StepDef, len(steps))
	inDegree := make(map[string]int, len(steps))
	for i := range steps {
		byName[steps[i].Name] = &steps[i]
		inDegree[steps[i].Name] = 0
	}

	// 計算入度
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", s.Name, dep)
			}
			inDegree[s.Name]++
		}
	}

	// BFS
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // 穩定排序

	var sorted []StepDef
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		sorted = append(sorted, *byName[name])

		// 減少依賴此 step 的入度
		for _, s := range steps {
			for _, dep := range s.DependsOn {
				if dep == name {
					inDegree[s.Name]--
					if inDegree[s.Name] == 0 {
						queue = append(queue, s.Name)
						sort.Strings(queue)
					}
				}
			}
		}
	}

	if len(sorted) != len(steps) {
		return nil, fmt.Errorf("cycle detected in steps DAG")
	}
	return sorted, nil
}
