package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runHandlerStub is a fully configurable CIEngineAdapter for handler tests.
type runHandlerStub struct {
	triggerResult *engine.TriggerResult
	triggerErr    error
	runStatus     *engine.RunStatus
	runErr        error
	cancelErr     error
	logsContent   string
	logsErr       error
	artifacts     []*engine.Artifact
	artifactsErr  error
}

func (s *runHandlerStub) Type() engine.EngineType                        { return engine.EngineGitLab }
func (s *runHandlerStub) IsAvailable(context.Context) bool               { return true }
func (s *runHandlerStub) Version(context.Context) (string, error)        { return "test", nil }
func (s *runHandlerStub) Capabilities() engine.EngineCapabilities        { return engine.EngineCapabilities{} }
func (s *runHandlerStub) Trigger(_ context.Context, _ *engine.TriggerRequest) (*engine.TriggerResult, error) {
	return s.triggerResult, s.triggerErr
}
func (s *runHandlerStub) GetRun(_ context.Context, _ string) (*engine.RunStatus, error) {
	return s.runStatus, s.runErr
}
func (s *runHandlerStub) Cancel(_ context.Context, _ string) error { return s.cancelErr }
func (s *runHandlerStub) StreamLogs(_ context.Context, _, _ string) (io.ReadCloser, error) {
	if s.logsErr != nil {
		return nil, s.logsErr
	}
	return io.NopCloser(strings.NewReader(s.logsContent)), nil
}
func (s *runHandlerStub) GetArtifacts(_ context.Context, _ string) ([]*engine.Artifact, error) {
	return s.artifacts, s.artifactsErr
}

// newRunHandlerTest sets up a CIEngineHandler backed by a stub adapter and a
// sqlmock DB. The mock will be pre-configured to return one ci_engine_configs
// row (id=1, engine_type=gitlab) for all buildAdapter calls.
func newRunHandlerTest(t *testing.T, stub *runHandlerStub) (*gin.Engine, sqlmock.Sqlmock, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)

	f := engine.NewFactory()
	require.NoError(t, f.Register(engine.EngineGitLab,
		func(*models.CIEngineConfig) (engine.CIEngineAdapter, error) { return stub, nil },
	))
	svc := services.NewCIEngineService(gormDB, f)
	h := NewCIEngineHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(42)) })
	r.POST("/api/v1/ci-engines/:id/runs", h.TriggerRun)
	r.GET("/api/v1/ci-engines/:id/runs/:runId", h.GetRun)
	r.DELETE("/api/v1/ci-engines/:id/runs/:runId", h.CancelRun)
	r.GET("/api/v1/ci-engines/:id/runs/:runId/logs", h.StreamLogs)
	r.GET("/api/v1/ci-engines/:id/runs/:runId/artifacts", h.GetArtifacts)

	// expectCfg seeds the sqlmock to return a gitlab config row for the next
	// SELECT on ci_engine_configs.
	expectCfg := func() {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs" WHERE "ci_engine_configs"."id" = $1`)).
			WithArgs(uint(1), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "engine_type", "name", "enabled"}).
				AddRow(1, "gitlab", "gl-prod", true))
	}
	// Expose via mock's Sqlmock interface (t.Cleanup instead of returning a function).
	_ = expectCfg

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return r, mock, cleanup
}

// expectCfgRow seeds one config row on the mock (id=1, gitlab).
func expectCfgRow(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs" WHERE "ci_engine_configs"."id" = $1`)).
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "engine_type", "name", "enabled"}).
			AddRow(1, "gitlab", "gl-prod", true))
}

// ---------------------------------------------------------------------------
// TriggerRun
// ---------------------------------------------------------------------------

func TestCIEngineHandler_TriggerRun_InvalidID(t *testing.T) {
	r, _, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines/bad/runs", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCIEngineHandler_TriggerRun_BadJSON(t *testing.T) {
	r, _, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines/1/runs", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCIEngineHandler_TriggerRun_AdapterError(t *testing.T) {
	stub := &runHandlerStub{triggerErr: fmt.Errorf("w: %w", engine.ErrUnavailable)}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	body, _ := json.Marshal(map[string]any{"ref": "main"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines/1/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "CI_ENGINE_UNAVAILABLE")
}

func TestCIEngineHandler_TriggerRun_Success(t *testing.T) {
	stub := &runHandlerStub{
		triggerResult: &engine.TriggerResult{RunID: "gl-pipeline-42", ExternalID: "42"},
	}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	body, _ := json.Marshal(map[string]any{"ref": "main", "pipeline_id": 1})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ci-engines/1/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "gl-pipeline-42", resp["run_id"])
	assert.Equal(t, "42", resp["external_id"])
}

// ---------------------------------------------------------------------------
// GetRun
// ---------------------------------------------------------------------------

func TestCIEngineHandler_GetRun_InvalidID(t *testing.T) {
	r, _, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/bad/runs/pipeline-1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCIEngineHandler_GetRun_NotFound(t *testing.T) {
	stub := &runHandlerStub{runErr: fmt.Errorf("w: %w", engine.ErrNotFound)}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "CI_ENGINE_RUN_NOT_FOUND")
}

func TestCIEngineHandler_GetRun_Success(t *testing.T) {
	stub := &runHandlerStub{
		runStatus: &engine.RunStatus{RunID: "pipeline-1", Phase: engine.RunPhaseRunning},
	}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "running", resp["phase"])
}

// ---------------------------------------------------------------------------
// CancelRun
// ---------------------------------------------------------------------------

func TestCIEngineHandler_CancelRun_InvalidID(t *testing.T) {
	r, _, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ci-engines/bad/runs/r-1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCIEngineHandler_CancelRun_NotFound(t *testing.T) {
	stub := &runHandlerStub{cancelErr: fmt.Errorf("w: %w", engine.ErrNotFound)}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ci-engines/1/runs/pipeline-999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCIEngineHandler_CancelRun_Success(t *testing.T) {
	r, mock, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ci-engines/1/runs/pipeline-1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ---------------------------------------------------------------------------
// StreamLogs
// ---------------------------------------------------------------------------

func TestCIEngineHandler_StreamLogs_InvalidID(t *testing.T) {
	r, _, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/bad/runs/r-1/logs", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCIEngineHandler_StreamLogs_Unsupported(t *testing.T) {
	stub := &runHandlerStub{logsErr: fmt.Errorf("w: %w", engine.ErrUnsupported)}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-1/logs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "CI_ENGINE_NOT_SUPPORTED")
}

func TestCIEngineHandler_StreamLogs_Success(t *testing.T) {
	const logContent = "building image...\ndone\n"
	stub := &runHandlerStub{logsContent: logContent}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-1/logs?step=job-55", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, logContent, w.Body.String())
}

func TestCIEngineHandler_StreamLogs_EmptyStep_OK(t *testing.T) {
	stub := &runHandlerStub{logsContent: "auto log\n"}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	// No ?step= parameter → auto-select
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-1/logs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "auto log")
}

// ---------------------------------------------------------------------------
// GetArtifacts
// ---------------------------------------------------------------------------

func TestCIEngineHandler_GetArtifacts_InvalidID(t *testing.T) {
	r, _, cleanup := newRunHandlerTest(t, &runHandlerStub{})
	defer cleanup()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/bad/runs/r-1/artifacts", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCIEngineHandler_GetArtifacts_Empty(t *testing.T) {
	stub := &runHandlerStub{artifacts: []*engine.Artifact{}}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-1/artifacts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []any `json:"items"`
		Total int   `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Total)
}

func TestCIEngineHandler_GetArtifacts_Success(t *testing.T) {
	stub := &runHandlerStub{
		artifacts: []*engine.Artifact{
			{Name: "app.jar", Kind: "file", SizeBytes: 2048},
			{Name: "report.html", Kind: "scan-report"},
		},
	}
	r, mock, cleanup := newRunHandlerTest(t, stub)
	defer cleanup()
	expectCfgRow(mock)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ci-engines/1/runs/pipeline-1/artifacts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, "app.jar", resp.Items[0]["name"])
}
