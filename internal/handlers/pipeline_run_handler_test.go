package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services"
)

// ─── Setup ──────────────────────────────────────────────────────────────────

func newRunHandlerWithRouter(t *testing.T) (*PipelineRunHandler, sqlmock.Sqlmock, *gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	pipelineSvc := services.NewPipelineService(gormDB)
	scheduler := services.NewPipelineScheduler(gormDB, nil, nil, nil, nil, nil, services.SchedulerConfig{})
	h := NewPipelineRunHandler(pipelineSvc, scheduler, nil)

	r := gin.New()
	r.GET("/pipelines/:pipelineID/runs", h.ListRuns)
	r.GET("/pipelines/:pipelineID/runs/:runID", h.GetRun)
	r.POST("/pipelines/:pipelineID/runs/:runID/cancel", h.CancelRun)
	r.POST("/pipelines/:pipelineID/runs/:runID/rollback", h.RollbackRun)
	r.GET("/pipeline-step-types", h.ListStepTypes)

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return h, mock, r, cleanup
}

var runCols = []string{
	"id", "pipeline_id", "snapshot_id", "cluster_id", "namespace",
	"status", "trigger_type", "trigger_payload", "triggered_by_user",
	"concurrency_group", "rerun_from_id", "rerun_from_step", "error",
	"queued_at", "started_at", "finished_at",
	"created_at", "updated_at", "deleted_at", "bound_node_name",
}

var stepRunCols = []string{
	"id", "pipeline_run_id", "step_name", "step_type",
	"status", "pod_name", "namespace",
	"started_at", "finished_at", "error",
	"created_at", "updated_at",
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

// ─── ListRuns ─────────────────────────────────────────────────────────────────

func TestPipelineRunHandler_List_InvalidPipelineID(t *testing.T) {
	_, _, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/abc/runs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineRunHandler_List_Empty(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(runCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/runs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(0), body["total"])
}

func TestPipelineRunHandler_List_WithRuns(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	now := time.Now()
	rows := sqlmock.NewRows(runCols).
		AddRow(1, 1, 1, 1, "default", "success", "manual", "", 1, "", nil, "", "", now, nil, nil, now, now, nil, "").
		AddRow(2, 1, 1, 1, "default", "failed", "webhook", "", 2, "", nil, "", "", now, nil, nil, now, now, nil, "")
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/runs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(2), body["total"])
}

// ─── GetRun ───────────────────────────────────────────────────────────────────

func TestPipelineRunHandler_Get_InvalidRunID(t *testing.T) {
	_, _, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/runs/bad", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineRunHandler_Get_NotFound(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/runs/999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPipelineRunHandler_Get_Success(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	// GetPipelineRun
	mock.ExpectQuery(`SELECT`).WillReturnRows(newRunRow(5, 1))
	// ListStepRuns
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(stepRunCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/runs/5", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotNil(t, body["run"])
	assert.NotNil(t, body["steps"])
}

// ─── CancelRun ────────────────────────────────────────────────────────────────

func TestPipelineRunHandler_Cancel_InvalidRunID(t *testing.T) {
	_, _, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/runs/bad/cancel", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── ListStepTypes ────────────────────────────────────────────────────────────

func TestPipelineRunHandler_ListStepTypes(t *testing.T) {
	_, _, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipeline-step-types", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.Bytes())
}

// ─── RollbackRun ──────────────────────────────────────────────────────────────

func TestPipelineRunHandler_Rollback_InvalidRunID(t *testing.T) {
	_, _, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/runs/bad/rollback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineRunHandler_Rollback_SourceNotFound(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/runs/999/rollback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPipelineRunHandler_Rollback_WrongPipeline(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	// Source run belongs to pipeline 2, but request targets pipeline 1
	now := time.Now()
	wrongPipelineRow := sqlmock.NewRows(runCols).AddRow(
		42, 2, 1, 1, "default", // pipeline_id=2
		"success", "manual", "", 1,
		"", nil, "", "",
		now, nil, nil,
		now, now, nil, "",
	)
	mock.ExpectQuery(`SELECT`).WillReturnRows(wrongPipelineRow)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/runs/42/rollback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineRunHandler_Rollback_SourceNotSuccess(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	// Source run is failed — cannot rollback
	now := time.Now()
	failedRunRow := sqlmock.NewRows(runCols).AddRow(
		10, 1, 1, 1, "default",
		"failed", "manual", "", 1, // status=failed
		"", nil, "", "",
		now, nil, nil,
		now, now, nil, "",
	)
	mock.ExpectQuery(`SELECT`).WillReturnRows(failedRunRow)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/runs/10/rollback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "expected error object in response")
	assert.Contains(t, errObj["message"], "success")
}

func TestPipelineRunHandler_Rollback_Success(t *testing.T) {
	_, mock, r, cleanup := newRunHandlerWithRouter(t)
	defer cleanup()

	// GetPipelineRun → success run belonging to pipeline 1
	mock.ExpectQuery(`SELECT`).WillReturnRows(newRunRow(5, 1))

	// EnqueueRun: queue overflow COUNT
	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// EnqueueRun: INSERT new rollback run
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/runs/5/rollback", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(5), body["rollback_of_run"])
	assert.NotNil(t, body["run_id"])
}
