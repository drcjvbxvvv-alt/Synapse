package handlers

import (
	"bytes"
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

// pipelineHandlerCols is the column list for Pipeline SELECT queries.
var pipelineHandlerCols = []string{
	"id", "name", "description", "concurrency_group", "concurrency_policy",
	"max_concurrent_runs", "created_by", "project_id",
	"created_at", "updated_at", "deleted_at",
}

func newPipelineHandler(t *testing.T) (*PipelineHandler, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := services.NewPipelineService(gormDB)
	h := NewPipelineHandler(svc, nil) // nil auditSvc → logPipelineAudit is a no-op

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return h, mock, cleanup
}

func pipelineRouter(h *PipelineHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/pipelines", h.CreatePipeline)
	r.GET("/pipelines", h.ListPipelines)
	r.GET("/pipelines/:pipelineID", h.GetPipeline)
	r.PUT("/pipelines/:pipelineID", h.UpdatePipeline)
	r.DELETE("/pipelines/:pipelineID", h.DeletePipeline)
	r.POST("/pipelines/:pipelineID/versions", h.CreateVersion)
	r.GET("/pipelines/:pipelineID/versions", h.ListVersions)
	r.GET("/pipelines/:pipelineID/versions/:version", h.GetVersion)
	return r
}

// ─── parseUintParam (via GetPipeline) ──────────────────────────────────────

func TestPipelineHandler_GetPipeline_InvalidID(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/not-a-number", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_GetPipeline_NotFound(t *testing.T) {
	h, mock, cleanup := newPipelineHandler(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/999", nil)
	r.ServeHTTP(w, req)

	// service returns ErrRecordNotFound wrapped in AppError → propagated to handler
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
}

func TestPipelineHandler_GetPipeline_Success(t *testing.T) {
	h, mock, cleanup := newPipelineHandler(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(pipelineHandlerCols).
		AddRow(1, "ci-pipeline", "desc", "", "queue", 3, 1, nil, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ci-pipeline", body["name"])
}

// ─── CreatePipeline ─────────────────────────────────────────────────────────

func TestPipelineHandler_CreatePipeline_InvalidBody(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines", bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_CreatePipeline_MissingName(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{"description": "no name"})
	req, _ := http.NewRequest(http.MethodPost, "/pipelines", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_CreatePipeline_Success(t *testing.T) {
	h, mock, cleanup := newPipelineHandler(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipelines.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{
		"name":        "my-pipeline",
		"description": "test",
	})
	req, _ := http.NewRequest(http.MethodPost, "/pipelines", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// ─── ListPipelines ──────────────────────────────────────────────────────────

func TestPipelineHandler_ListPipelines_Empty(t *testing.T) {
	h, mock, cleanup := newPipelineHandler(t)
	defer cleanup()

	// COUNT query
	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// SELECT query
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(pipelineHandlerCols))

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(0), body["total"])
}

// ─── UpdatePipeline ─────────────────────────────────────────────────────────

func TestPipelineHandler_UpdatePipeline_InvalidID(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/pipelines/abc", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_UpdatePipeline_InvalidBody(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/pipelines/1", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── DeletePipeline ─────────────────────────────────────────────────────────

func TestPipelineHandler_DeletePipeline_InvalidID(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/pipelines/xyz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_DeletePipeline_Success(t *testing.T) {
	h, mock, cleanup := newPipelineHandler(t)
	defer cleanup()

	// Soft delete → UPDATE SET deleted_at
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipelines.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/pipelines/1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── CreateVersion ──────────────────────────────────────────────────────────

func TestPipelineHandler_CreateVersion_InvalidPipelineID(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/bad/versions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_CreateVersion_InvalidBody(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	// steps_json is required
	body, _ := json.Marshal(map[string]any{"description": "no steps"})
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/versions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── ListVersions ───────────────────────────────────────────────────────────

func TestPipelineHandler_ListVersions_InvalidID(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/nope/versions", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── GetVersion ─────────────────────────────────────────────────────────────

func TestPipelineHandler_GetVersion_InvalidVersionNumber(t *testing.T) {
	h, _, cleanup := newPipelineHandler(t)
	defer cleanup()

	r := pipelineRouter(h)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/versions/abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
