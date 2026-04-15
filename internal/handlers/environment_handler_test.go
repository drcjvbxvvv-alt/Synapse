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

var envHandlerCols = []string{
	"id", "name", "pipeline_id", "cluster_id", "namespace",
	"order_index", "auto_promote", "approval_required",
	"approver_ids", "smoke_test_step_name", "notify_channel_ids",
	"variables_json", "created_at", "updated_at", "deleted_at",
}

func newEnvHandlerWithRouter(t *testing.T) (*EnvironmentHandler, sqlmock.Sqlmock, *gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := services.NewEnvironmentService(gormDB)
	h := NewEnvironmentHandler(svc)

	r := gin.New()
	r.GET("/pipelines/:pipelineID/environments", h.ListEnvironments)
	r.GET("/pipelines/:pipelineID/environments/:envID", h.GetEnvironment)
	r.POST("/pipelines/:pipelineID/environments", h.CreateEnvironment)
	r.PUT("/pipelines/:pipelineID/environments/:envID", h.UpdateEnvironment)
	r.DELETE("/pipelines/:pipelineID/environments/:envID", h.DeleteEnvironment)

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return h, mock, r, cleanup
}

// ─── ListEnvironments ──────────────────────────────────────────────────────

func TestEnvironmentHandler_List_InvalidPipelineID(t *testing.T) {
	_, _, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/abc/environments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnvironmentHandler_List_Empty(t *testing.T) {
	_, mock, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	// GetEnvironmentsForPipeline uses a JOIN query
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(
		[]string{"id", "name", "pipeline_id", "cluster_id", "namespace",
			"order_index", "auto_promote", "approval_required",
			"approver_ids", "smoke_test_step_name", "notify_channel_ids",
			"variables_json", "created_at", "updated_at", "deleted_at",
			"cluster_name", "cluster_status"},
	))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/environments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(0), body["total"])
}

// ─── GetEnvironment ────────────────────────────────────────────────────────

func TestEnvironmentHandler_Get_InvalidEnvID(t *testing.T) {
	_, _, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/environments/bad", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnvironmentHandler_Get_NotFound(t *testing.T) {
	_, mock, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/environments/999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEnvironmentHandler_Get_Success(t *testing.T) {
	_, mock, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(envHandlerCols).AddRow(
			5, "dev", 1, 2, "app-dev",
			1, false, false, "", "", "", "{}", now, now, nil,
		),
	)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/1/environments/5", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "dev", body["name"])
}

// ─── CreateEnvironment ─────────────────────────────────────────────────────

func TestEnvironmentHandler_Create_InvalidPipelineID(t *testing.T) {
	_, _, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/xyz/environments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnvironmentHandler_Create_MissingFields(t *testing.T) {
	_, _, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	body := `{"name":"dev"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/1/environments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── DeleteEnvironment ─────────────────────────────────────────────────────

func TestEnvironmentHandler_Delete_InvalidEnvID(t *testing.T) {
	_, _, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/pipelines/1/environments/abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnvironmentHandler_Delete_Success(t *testing.T) {
	_, mock, r, cleanup := newEnvHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .environments.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/pipelines/1/environments/5", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
