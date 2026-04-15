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

// ─── Setup ──────────────────────────────────────────────────────────────────

func newSecretHandlerWithRouter(t *testing.T) (*PipelineSecretHandler, sqlmock.Sqlmock, *gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := services.NewPipelineSecretService(gormDB)
	h := NewPipelineSecretHandler(svc)

	r := gin.New()
	// Global scope routes
	r.POST("/pipeline-secrets", h.CreateSecret)
	r.GET("/pipeline-secrets/:secretID", h.GetSecret)
	r.GET("/pipeline-secrets", h.ListSecrets)
	r.PUT("/pipeline-secrets/:secretID", h.UpdateSecret)
	r.DELETE("/pipeline-secrets/:secretID", h.DeleteSecret)
	// Pipeline-scoped routes
	r.GET("/pipelines/:pipelineID/secrets", h.ListPipelineSecrets)
	r.POST("/pipelines/:pipelineID/secrets", h.CreatePipelineSecret)
	// Environment-scoped routes
	r.GET("/pipelines/:pipelineID/environments/:envID/secrets", h.ListEnvSecrets)
	r.POST("/pipelines/:pipelineID/environments/:envID/secrets", h.CreateEnvSecret)

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return h, mock, r, cleanup
}

var secretListCols = []string{"id", "scope", "scope_ref", "name", "description", "created_by", "created_at", "updated_at"}
var secretAllCols = []string{"id", "scope", "scope_ref", "name", "value_enc", "description", "created_by", "created_at", "updated_at", "deleted_at"}

func newSecretListRow() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(secretListCols).AddRow(1, "global", nil, "MY_SECRET", "desc", 1, now, now)
}

func newSecretRow() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(secretAllCols).AddRow(1, "global", nil, "MY_SECRET", "enc", "desc", 1, now, now, nil)
}

// ─── CreateSecret ─────────────────────────────────────────────────────────────

func TestSecretHandler_Create_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_secrets.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	body := `{"scope":"global","name":"MY_SECRET","value":"supersecret"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipeline-secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "MY_SECRET", resp["name"])
}

func TestSecretHandler_Create_MissingFields(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	body := `{"name":"MY_SECRET"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipeline-secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecretHandler_Create_Duplicate(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	body := `{"scope":"global","name":"MY_SECRET","value":"v"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipeline-secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// apierrors returns 409 for duplicate
	assert.Equal(t, http.StatusConflict, w.Code)
}

// ─── GetSecret ────────────────────────────────────────────────────────────────

func TestSecretHandler_Get_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newSecretRow())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipeline-secrets/1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "MY_SECRET", resp["name"])
}

func TestSecretHandler_Get_InvalidID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipeline-secrets/abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecretHandler_Get_NotFound(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipeline-secrets/999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── ListSecrets ──────────────────────────────────────────────────────────────

func TestSecretHandler_List_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newSecretListRow())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipeline-secrets", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandler_List_WithScope(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newSecretListRow())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipeline-secrets?scope=global&scope_ref=1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── UpdateSecret ─────────────────────────────────────────────────────────────

func TestSecretHandler_Update_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newSecretRow())
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"value":"newval"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/pipeline-secrets/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandler_Update_InvalidID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	body := `{"value":"v"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/pipeline-secrets/bad", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── DeleteSecret ─────────────────────────────────────────────────────────────

func TestSecretHandler_Delete_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .pipeline_secrets.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/pipeline-secrets/1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandler_Delete_InvalidID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/pipeline-secrets/xyz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── Pipeline-scoped endpoints ────────────────────────────────────────────────

func TestSecretHandler_ListPipelineSecrets_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newSecretListRow())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/5/secrets", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandler_ListPipelineSecrets_InvalidPipelineID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/abc/secrets", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecretHandler_CreatePipelineSecret_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_secrets.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	mock.ExpectCommit()

	body := `{"name":"PIPELINE_TOKEN","value":"tok"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/5/secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "PIPELINE_TOKEN", resp["name"])
	assert.Equal(t, "pipeline", resp["scope"])
}

func TestSecretHandler_CreatePipelineSecret_InvalidPipelineID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	body := `{"name":"X","value":"v"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/abc/secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── Environment-scoped endpoints ────────────────────────────────────────────

func TestSecretHandler_ListEnvSecrets_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(newSecretListRow())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/5/environments/3/secrets", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandler_ListEnvSecrets_InvalidEnvID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/pipelines/5/environments/bad/secrets", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecretHandler_CreateEnvSecret_Success(t *testing.T) {
	_, mock, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_secrets.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectCommit()

	body := `{"name":"ENV_SECRET","value":"val"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/5/environments/3/secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ENV_SECRET", resp["name"])
	assert.Equal(t, "environment", resp["scope"])
}

func TestSecretHandler_CreateEnvSecret_InvalidEnvID(t *testing.T) {
	_, _, r, cleanup := newSecretHandlerWithRouter(t)
	defer cleanup()

	body := `{"name":"X","value":"v"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/pipelines/5/environments/bad/secrets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
