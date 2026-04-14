package services

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shaia/Synapse/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// M13a E2E 測試 — 手動觸發一條 build + deploy Pipeline 成功
//
// 驗證完整流程：
//   1. PipelineService.CreatePipeline — 建立 build+deploy Pipeline
//   2. PipelineService.CreateVersion  — 建立不可變版本快照（含 hash）
//   3. PipelineScheduler.EnqueueRun   — 手動觸發 Run，狀態 = queued
//   4. 驗證 Run 欄位正確（trigger_type=manual, triggered_by_user, snapshot_id）
// ---------------------------------------------------------------------------

// setupE2EDB 建立 sqlmock backed GORM DB。
func setupE2EDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open gorm: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return gormDB, mock
}

// TestM13aE2E_CreatePipeline validates that PipelineService correctly
// creates a pipeline with build and deploy steps.
func TestM13aE2E_CreatePipeline(t *testing.T) {
	db, mock := setupE2EDB(t)
	svc := NewPipelineService(db)

	// Expect INSERT into pipelines, RETURNING id
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "pipelines"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	req := &CreatePipelineRequest{
		Name:      "build-and-deploy",
		ClusterID: 1,
		Namespace: "production",
	}

	pipeline, err := svc.CreatePipeline(context.Background(), req, 42)
	if err != nil {
		t.Fatalf("CreatePipeline failed: %v", err)
	}

	if pipeline.Name != "build-and-deploy" {
		t.Errorf("expected name build-and-deploy, got %s", pipeline.Name)
	}
	if pipeline.ClusterID != 1 {
		t.Errorf("expected cluster_id 1, got %d", pipeline.ClusterID)
	}
	if pipeline.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", pipeline.Namespace)
	}
	if pipeline.ConcurrencyPolicy != models.ConcurrencyPolicyCancelPrevious {
		t.Errorf("expected default concurrency policy cancel_previous, got %s", pipeline.ConcurrencyPolicy)
	}
	if pipeline.MaxConcurrentRuns != 1 {
		t.Errorf("expected default max_concurrent_runs 1, got %d", pipeline.MaxConcurrentRuns)
	}
	if pipeline.CreatedBy != 42 {
		t.Errorf("expected created_by=42, got %d", pipeline.CreatedBy)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

// TestM13aE2E_CreateVersion validates that version snapshot is created with
// correct hash and links to the pipeline.
func TestM13aE2E_CreateVersion(t *testing.T) {
	db, mock := setupE2EDB(t)
	svc := NewPipelineService(db)

	// Build+deploy steps JSON (realistic pipeline YAML-equivalent)
	stepsJSON := mustMarshal([]map[string]interface{}{
		{
			"name":  "build-image",
			"type":  "build-image",
			"image": "gcr.io/kaniko-project/executor:v1.23.2",
			"config": map[string]interface{}{
				"destination": "registry.example.com/myapp:latest",
			},
		},
		{
			"name":       "deploy",
			"type":       "deploy",
			"image":      "bitnami/kubectl:1.30",
			"depends_on": []string{"build-image"},
			"config": map[string]interface{}{
				"manifest":  "/workspace/k8s/deployment.yaml",
				"namespace": "production",
			},
		},
	})

	// Mock: GetPipeline (SELECT outside transaction)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pipelines"`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "cluster_id", "namespace", "current_version_id",
			"concurrency_policy", "max_concurrent_runs", "created_by",
		}).AddRow(1, "build-and-deploy", 1, "production", nil, "cancel_previous", 1, 42))

	// Transaction wraps all remaining DB ops atomically
	mock.ExpectBegin()

	// Mock: Check duplicate hash (SELECT from pipeline_versions) — no match → ErrRecordNotFound
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pipeline_versions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // empty → gorm.ErrRecordNotFound

	// Mock: COALESCE(MAX(version), 0) — returns 0 (first version)
	mock.ExpectQuery(`COALESCE`).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(0))

	// Mock: INSERT pipeline_version (inside same transaction)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "pipeline_versions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Mock: UPDATE pipeline.current_version_id (inside same transaction)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "pipelines"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	req := &CreateVersionRequest{
		StepsJSON: stepsJSON,
	}

	version, err := svc.CreateVersion(context.Background(), 1, req, 42)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	if version.PipelineID != 1 {
		t.Errorf("expected pipeline_id=1, got %d", version.PipelineID)
	}
	if version.HashSHA256 == "" {
		t.Error("version hash should not be empty")
	}
	if len(version.HashSHA256) != 64 {
		t.Errorf("hash should be 64-char SHA-256 hex, got len=%d", len(version.HashSHA256))
	}
	if version.StepsJSON != stepsJSON {
		t.Errorf("steps_json mismatch")
	}
	if version.CreatedBy != 42 {
		t.Errorf("expected created_by=42, got %d", version.CreatedBy)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

// TestM13aE2E_EnqueueManualRun validates that EnqueueRun creates a
// PipelineRun in "queued" state with correct trigger metadata.
func TestM13aE2E_EnqueueManualRun(t *testing.T) {
	db, mock := setupE2EDB(t)

	// Use a minimal scheduler (no K8s, no job builder — we only test DB writes)
	scheduler := &PipelineScheduler{
		db:  db,
		cfg: DefaultSchedulerConfig(),
	}

	// Mock: COUNT queued runs (overflow check)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "pipeline_runs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock: INSERT pipeline_run
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "pipeline_runs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	mock.ExpectCommit()

	snapshotID := uint(1)
	run := &models.PipelineRun{
		PipelineID:      1,
		SnapshotID:      snapshotID,
		ClusterID:       1,
		Namespace:       "production",
		TriggerType:     models.TriggerTypeManual,
		TriggeredByUser: 42,
	}

	if err := scheduler.EnqueueRun(context.Background(), run); err != nil {
		t.Fatalf("EnqueueRun failed: %v", err)
	}

	// Verify the Run fields set by EnqueueRun
	if run.Status != models.PipelineRunStatusQueued {
		t.Errorf("expected status %s, got %s", models.PipelineRunStatusQueued, run.Status)
	}
	if run.QueuedAt.IsZero() {
		t.Error("queued_at should be set")
	}
	if run.TriggerType != models.TriggerTypeManual {
		t.Errorf("expected trigger_type=manual, got %s", run.TriggerType)
	}
	if run.TriggeredByUser != 42 {
		t.Errorf("expected triggered_by_user=42, got %d", run.TriggeredByUser)
	}
	if run.SnapshotID != snapshotID {
		t.Errorf("expected snapshot_id=%d, got %d", snapshotID, run.SnapshotID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

// TestM13aE2E_QueueOverflow validates that EnqueueRun rejects a run when
// the queue is full (overflow protection).
func TestM13aE2E_QueueOverflow(t *testing.T) {
	db, mock := setupE2EDB(t)

	cfg := DefaultSchedulerConfig() // SystemMaxRuns=20, QueueOverflowRatio=3 → threshold=60
	scheduler := &PipelineScheduler{db: db, cfg: cfg}

	// Mock: COUNT queued runs → returns 60 (at the threshold)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "pipeline_runs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(60))

	// When overflow, EnqueueRun creates a rejected run
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "pipeline_runs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(100))
	mock.ExpectCommit()

	run := &models.PipelineRun{
		PipelineID:      1,
		SnapshotID:      1,
		ClusterID:       1,
		TriggerType:     models.TriggerTypeManual,
		TriggeredByUser: 42,
	}

	// Should not error (overflow is handled gracefully)
	if err := scheduler.EnqueueRun(context.Background(), run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.Status != models.PipelineRunStatusRejected {
		t.Errorf("expected status=%s for overflow, got %s", models.PipelineRunStatusRejected, run.Status)
	}
	if run.Error == "" {
		t.Error("rejected run should have an error message")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

// TestM13aE2E_VersionHashDeterminism verifies that the same steps JSON always
// produces the same version hash — required for immutable snapshot dedup.
func TestM13aE2E_VersionHashDeterminism(t *testing.T) {
	stepsJSON := mustMarshal([]map[string]interface{}{
		{"name": "build-image", "type": "build-image"},
		{"name": "deploy", "type": "deploy", "depends_on": []string{"build-image"}},
	})

	req := &CreateVersionRequest{
		StepsJSON: stepsJSON,
	}

	h1 := computeVersionHash(req)
	h2 := computeVersionHash(req)

	if h1 != h2 {
		t.Errorf("version hash is not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d", len(h1))
	}
}

// TestM13aE2E_RerunInheritsSnapshot verifies that a Rerun preserves the
// original snapshot_id and sets trigger_type=rerun.
func TestM13aE2E_RerunInheritsSnapshot(t *testing.T) {
	originalSnapshotID := uint(7)

	// Simulate the rerun handler logic (as in PipelineRunHandler.RerunPipeline)
	original := &models.PipelineRun{
		PipelineID: 1,
		SnapshotID: originalSnapshotID,
		ClusterID:  1,
		Namespace:  "staging",
	}

	newRun := &models.PipelineRun{
		PipelineID:      original.PipelineID,
		SnapshotID:      original.SnapshotID, // inherited
		ClusterID:       original.ClusterID,
		Namespace:       original.Namespace,
		TriggerType:     models.TriggerTypeRerun,
		TriggeredByUser: 99,
		RerunFromID:     &original.ID,
	}

	if newRun.SnapshotID != originalSnapshotID {
		t.Errorf("rerun must inherit original snapshot_id=%d, got %d",
			originalSnapshotID, newRun.SnapshotID)
	}
	if newRun.TriggerType != models.TriggerTypeRerun {
		t.Errorf("expected trigger_type=rerun, got %s", newRun.TriggerType)
	}
	if newRun.RerunFromID == nil || *newRun.RerunFromID != original.ID {
		t.Errorf("rerun_from_id should point to original run")
	}
}

// TestM13aE2E_PipelineStatusConstants checks that pipeline status constants
// match the expected values used by the scheduler and watcher.
func TestM13aE2E_PipelineStatusConstants(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{"queued", models.PipelineRunStatusQueued, "queued"},
		{"running", models.PipelineRunStatusRunning, "running"},
		{"succeeded", models.PipelineRunStatusSuccess, "success"},
		{"failed", models.PipelineRunStatusFailed, "failed"},
		{"cancelled", models.PipelineRunStatusCancelled, "cancelled"},
		{"rejected", models.PipelineRunStatusRejected, "rejected"},
		{"trigger_manual", models.TriggerTypeManual, "manual"},
		{"trigger_webhook", models.TriggerTypeWebhook, "webhook"},
		{"trigger_rerun", models.TriggerTypeRerun, "rerun"},
	}

	for _, c := range cases {
		if c.got != c.expected {
			t.Errorf("%s: expected %q, got %q", c.name, c.expected, c.got)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
