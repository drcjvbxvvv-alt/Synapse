package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newPipelineService(t *testing.T) (*PipelineService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := NewPipelineService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

var runCols = []string{
	"id", "pipeline_id", "snapshot_id", "cluster_id", "namespace",
	"status", "trigger_type", "trigger_payload", "triggered_by_user",
	"concurrency_group", "rerun_from_id", "rerun_from_step", "error",
	"queued_at", "started_at", "finished_at",
	"created_at", "updated_at", "deleted_at", "bound_node_name",
}

func newRunRow(id, pipelineID uint) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(runCols).AddRow(
		id, pipelineID, 1, 1, "default",
		"success", "manual", "", 1,
		"", nil, "", "",
		now, nil, nil,
		now, now, nil, "",
	)
}

var pipelineCols = []string{
	"id", "name", "description", "project_id", "current_version_id",
	"concurrency_group", "concurrency_policy", "max_concurrent_runs",
	"notify_on_success", "notify_on_failure", "notify_on_scan",
	"created_by", "created_at", "updated_at", "deleted_at",
}

var pipelineVersionCols = []string{
	"id", "pipeline_id", "version", "steps_json", "triggers_json",
	"env_json", "runtime_json", "workspace_json", "hash_sha256",
	"created_by", "created_at",
}

func pipelineRow(id uint, name string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pipelineCols).AddRow(
		id, name, "", nil, nil,
		"", "cancel_previous", 1,
		"[]", "[]", "[]",
		1, now, now, nil,
	)
}

func pipelineVersionRow(id, pipelineID uint, version int) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pipelineVersionCols).AddRow(
		id, pipelineID, version, `[{"name":"build"}]`, "",
		"", "", "", "abc123",
		1, now,
	)
}

// ---------------------------------------------------------------------------
// DB()
// ---------------------------------------------------------------------------

func TestPipelineService_DB_ReturnsDB(t *testing.T) {
	svc, _, cleanup := newPipelineService(t)
	defer cleanup()
	assert.NotNil(t, svc.DB())
}

// ---------------------------------------------------------------------------
// ListPipelines
// ---------------------------------------------------------------------------

func TestPipelineService_ListPipelines_Empty(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(pipelineCols))

	pipelines, total, err := svc.ListPipelines(context.Background(), &ListPipelinesParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Empty(t, pipelines)
	assert.Equal(t, int64(0), total)
}

func TestPipelineService_ListPipelines_WithSearch(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT`).WillReturnRows(pipelineRow(1, "my-pipeline"))

	pipelines, total, err := svc.ListPipelines(context.Background(), &ListPipelinesParams{
		Search:   "my",
		Page:     1,
		PageSize: 20,
	})
	require.NoError(t, err)
	assert.Len(t, pipelines, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "my-pipeline", pipelines[0].Name)
}

func TestPipelineService_ListPipelines_CountError(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).WillReturnError(gorm.ErrInvalidData)

	_, _, err := svc.ListPipelines(context.Background(), &ListPipelinesParams{Page: 1, PageSize: 20})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count pipelines")
}

// ---------------------------------------------------------------------------
// UpdatePipeline
// ---------------------------------------------------------------------------

func TestPipelineService_UpdatePipeline_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	// GetPipeline
	mock.ExpectQuery(`SELECT`).WillReturnRows(pipelineRow(1, "old-name"))
	// Save
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipelines.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	desc := "new description"
	got, err := svc.UpdatePipeline(context.Background(), 1, &UpdatePipelineRequest{Description: &desc})
	require.NoError(t, err)
	assert.Equal(t, "new description", got.Description)
}

func TestPipelineService_UpdatePipeline_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	desc := "x"
	_, err := svc.UpdatePipeline(context.Background(), 999, &UpdatePipelineRequest{Description: &desc})
	assert.Error(t, err)
}

func TestPipelineService_UpdatePipeline_AllFields(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(pipelineRow(1, "p1"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipelines.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	desc := "desc"
	group := "deploy"
	policy := "queue"
	maxRuns := 3

	got, err := svc.UpdatePipeline(context.Background(), 1, &UpdatePipelineRequest{
		Description:       &desc,
		ConcurrencyGroup:  &group,
		ConcurrencyPolicy: &policy,
		MaxConcurrentRuns: &maxRuns,
	})
	require.NoError(t, err)
	assert.Equal(t, "desc", got.Description)
	assert.Equal(t, "deploy", got.ConcurrencyGroup)
}

// ---------------------------------------------------------------------------
// DeletePipeline
// ---------------------------------------------------------------------------

func TestPipelineService_DeletePipeline_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipelines.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeletePipeline(context.Background(), 1)
	assert.NoError(t, err)
}

func TestPipelineService_DeletePipeline_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipelines.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeletePipeline(context.Background(), 999)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// GetVersion
// ---------------------------------------------------------------------------

func TestPipelineService_GetVersion_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(pipelineVersionRow(1, 1, 1))

	v, err := svc.GetVersion(context.Background(), 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, v.Version)
}

func TestPipelineService_GetVersion_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetVersion(context.Background(), 1, 99)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// ListVersions
// ---------------------------------------------------------------------------

func TestPipelineService_ListVersions_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	// GetPipeline check
	mock.ExpectQuery(`SELECT`).WillReturnRows(pipelineRow(1, "p1"))
	// List versions
	now := time.Now()
	rows := sqlmock.NewRows(pipelineVersionCols).
		AddRow(2, 1, 2, `[]`, "", "", "", "", "hash2", 1, now).
		AddRow(1, 1, 1, `[]`, "", "", "", "", "hash1", 1, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	versions, err := svc.ListVersions(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, versions, 2)
}

func TestPipelineService_ListVersions_PipelineNotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.ListVersions(context.Background(), 999)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// GetStepRun
// ---------------------------------------------------------------------------

func TestPipelineService_GetStepRun_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "pipeline_run_id", "step_name", "step_type",
		"status", "pod_name", "namespace",
		"started_at", "finished_at", "error",
		"created_at", "updated_at",
	}).AddRow(5, 3, "build", "build-image", "success", "pod-1", "default",
		now, nil, "", now, now)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	sr, err := svc.GetStepRun(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, "build", sr.StepName)
	assert.Equal(t, "success", sr.Status)
}

func TestPipelineService_GetStepRun_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetStepRun(context.Background(), 999)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// ListPipelineRuns
// ---------------------------------------------------------------------------

func TestPipelineService_ListRuns_Empty(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(runCols))

	runs, total, err := svc.ListPipelineRuns(context.Background(), &ListPipelineRunsParams{PipelineID: 1, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Empty(t, runs)
	assert.Equal(t, int64(0), total)
}

func TestPipelineService_ListRuns_WithStatus(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT`).WillReturnRows(newRunRow(1, 1))

	runs, total, err := svc.ListPipelineRuns(context.Background(), &ListPipelineRunsParams{
		PipelineID: 1,
		Status:     "success",
		Page:       1,
		PageSize:   20,
	})
	require.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.Equal(t, int64(1), total)
}

// ---------------------------------------------------------------------------
// GetPipelineRun
// ---------------------------------------------------------------------------

func TestPipelineService_GetRun_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newRunRow(5, 1))

	run, err := svc.GetPipelineRun(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, uint(5), run.ID)
	assert.Equal(t, uint(1), run.PipelineID)
}

func TestPipelineService_GetRun_NotFound(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetPipelineRun(context.Background(), 999)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// ListStepRuns
// ---------------------------------------------------------------------------

func TestPipelineService_ListStepRuns_Success(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "pipeline_run_id", "step_name", "step_type",
		"status", "pod_name", "namespace",
		"started_at", "finished_at", "error",
		"created_at", "updated_at",
	}).
		AddRow(1, 5, "build", "build-image", "success", "pod-1", "default", now, nil, "", now, now).
		AddRow(2, 5, "deploy", "deploy", "running", "pod-2", "default", now, nil, "", now, now)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	steps, err := svc.ListStepRuns(context.Background(), 5)
	require.NoError(t, err)
	assert.Len(t, steps, 2)
	assert.Equal(t, "build", steps[0].StepName)
	assert.Equal(t, "deploy", steps[1].StepName)
}

func TestPipelineService_ListStepRuns_Empty(t *testing.T) {
	svc, mock, cleanup := newPipelineService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "pipeline_run_id", "step_name", "step_type",
		"status", "pod_name", "namespace",
		"started_at", "finished_at", "error",
		"created_at", "updated_at",
	}))

	steps, err := svc.ListStepRuns(context.Background(), 5)
	require.NoError(t, err)
	assert.Empty(t, steps)
}
