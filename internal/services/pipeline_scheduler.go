package services

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
//
// 子檔案：
//   - pipeline_scheduler_enqueue.go    EnqueueRun / EnqueueRunInEnvironment / CancelRun
//   - pipeline_scheduler_concurrency.go concurrencyCounts / canSchedule / applyConcurrencyPolicy
//   - pipeline_scheduler_dag.go        StepDef / executeRunAsync / waitForStep / failRun
//   - pipeline_scheduler_step.go       executeStepWithRetry
//   - pipeline_scheduler_secrets.go    resolveSecrets / injectRegistryCredentials / resolveImagePullSecret
//   - pipeline_scheduler_rerun.go      loadOriginalStepResults / isBeforeStep
//   - pipeline_scheduler_rollback.go   loadRollbackArtifacts / copyArtifactsToRollbackRun
//   - pipeline_topo.go                 topoSortSteps
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

// PipelineK8sProvider 提供 Pipeline 排程所需的 K8s client。
type PipelineK8sProvider interface {
	GetK8sClientByID(clusterID uint) *K8sClient
}

// PipelineScheduler 管理 Pipeline Run 的排程與並發控制。
type PipelineScheduler struct {
	db          *gorm.DB
	jobBuilder  *JobBuilder
	secretSvc   *PipelineSecretService
	registrySvc *RegistryService
	k8sProvider PipelineK8sProvider
	watcher     *JobWatcher
	notifier    *PipelineNotifier
	cfg         SchedulerConfig

	stopCh    chan struct{}
	once      sync.Once
	loopAlive atomic.Bool
}

// SetRegistryService 設定 Registry 服務（用於 push-image 自動注入認證）。
func (s *PipelineScheduler) SetRegistryService(svc *RegistryService) {
	s.registrySvc = svc
}

// NewPipelineScheduler 建立排程器。
func NewPipelineScheduler(
	db *gorm.DB,
	jobBuilder *JobBuilder,
	secretSvc *PipelineSecretService,
	k8sProvider PipelineK8sProvider,
	watcher *JobWatcher,
	notifier *PipelineNotifier,
	cfg SchedulerConfig,
) *PipelineScheduler {
	return &PipelineScheduler{
		db:          db,
		jobBuilder:  jobBuilder,
		secretSvc:   secretSvc,
		k8sProvider: k8sProvider,
		watcher:     watcher,
		notifier:    notifier,
		cfg:         cfg,
		stopCh:      make(chan struct{}),
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Start 啟動排程迴圈（background goroutine）。
func (s *PipelineScheduler) Start() {
	s.loopAlive.Store(true)
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
// Health
// ---------------------------------------------------------------------------

// IsAlive reports whether the scheduler loop goroutine is still running.
func (s *PipelineScheduler) IsAlive() bool {
	return s.loopAlive.Load()
}

// QueueDepth returns the number of pipeline runs currently in the queued state.
func (s *PipelineScheduler) QueueDepth(ctx context.Context) (int64, error) {
	if s.db == nil {
		return 0, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.PipelineRun{}).
		Where("status = ?", models.PipelineRunStatusQueued).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("queue depth: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Scheduler loop
// ---------------------------------------------------------------------------

func (s *PipelineScheduler) loop() {
	defer s.loopAlive.Store(false)
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

	counts, err := s.loadConcurrencyCounts(ctx)
	if err != nil {
		return err
	}

	for i := range queuedRuns {
		run := &queuedRuns[i]

		if !s.canSchedule(ctx, run, counts) {
			continue
		}

		now := time.Now()
		run.Status = models.PipelineRunStatusRunning
		run.StartedAt = &now
		if err := s.db.WithContext(ctx).Save(run).Error; err != nil {
			logger.Error("failed to start run", "run_id", run.ID, "error", err)
			continue
		}

		counts.system++
		counts.cluster[run.ClusterID]++
		counts.pipeline[run.PipelineID]++

		logger.Info("pipeline run started",
			"run_id", run.ID,
			"pipeline_id", run.PipelineID,
			"cluster_id", run.ClusterID,
		)

		go s.executeRunAsync(run)
	}

	return nil
}
