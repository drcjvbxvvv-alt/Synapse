package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// gormErrRecordNotFound returns a gorm.ErrRecordNotFound for mock expectations.
func gormErrRecordNotFound() error { return gorm.ErrRecordNotFound }

// ---------------------------------------------------------------------------
// configurable stub adapter for run-operation tests
// ---------------------------------------------------------------------------

type runStubAdapter struct {
	triggerResult *engine.TriggerResult
	triggerErr    error
	runStatus     *engine.RunStatus
	runErr        error
	cancelErr     error
	logsRC        io.ReadCloser
	logsErr       error
	artifacts     []*engine.Artifact
	artifactsErr  error
}

func (a *runStubAdapter) Type() engine.EngineType        { return engine.EngineGitLab }
func (a *runStubAdapter) IsAvailable(context.Context) bool { return true }
func (a *runStubAdapter) Version(context.Context) (string, error) {
	return "test", nil
}
func (a *runStubAdapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{SupportsLiveLog: true}
}
func (a *runStubAdapter) Trigger(_ context.Context, _ *engine.TriggerRequest) (*engine.TriggerResult, error) {
	return a.triggerResult, a.triggerErr
}
func (a *runStubAdapter) GetRun(_ context.Context, _ string) (*engine.RunStatus, error) {
	return a.runStatus, a.runErr
}
func (a *runStubAdapter) Cancel(_ context.Context, _ string) error { return a.cancelErr }
func (a *runStubAdapter) StreamLogs(_ context.Context, _, _ string) (io.ReadCloser, error) {
	return a.logsRC, a.logsErr
}
func (a *runStubAdapter) GetArtifacts(_ context.Context, _ string) ([]*engine.Artifact, error) {
	return a.artifacts, a.artifactsErr
}

// newRunSvcWithStub wires up a CIEngineService backed by sqlmock that will
// return one ci_engine_configs row (id=1, engine_type=gitlab), and registers a
// factory builder that returns the provided stub adapter.
func newRunSvcWithStub(t *testing.T, stub *runStubAdapter) (*CIEngineService, sqlmock.Sqlmock, func()) {
	t.Helper()
	svc, mock, cleanup := newCIEngineServiceTest(t)

	require.NoError(t, svc.Factory().Register(engine.EngineGitLab,
		func(*models.CIEngineConfig) (engine.CIEngineAdapter, error) { return stub, nil },
	))
	return svc, mock, cleanup
}

// expectGetCfg seeds the mock to return a single engine config row for id=1.
func expectGetCfg(mock sqlmock.Sqlmock, id uint, engineType string) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs" WHERE "ci_engine_configs"."id" = $1`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "engine_type", "name", "enabled"}).
			AddRow(id, engineType, "test-engine", true))
}

// ---------------------------------------------------------------------------
// mapEngineError
// ---------------------------------------------------------------------------

func TestMapEngineError_Nil(t *testing.T) {
	assert.Nil(t, mapEngineError(nil, "op"))
}

func TestMapEngineError_NotFound(t *testing.T) {
	err := mapEngineError(fmt.Errorf("wrap: %w", engine.ErrNotFound), "get run")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, ae.HTTPStatus)
	assert.Equal(t, "CI_ENGINE_RUN_NOT_FOUND", ae.Code)
}

func TestMapEngineError_InvalidInput(t *testing.T) {
	err := mapEngineError(fmt.Errorf("w: %w", engine.ErrInvalidInput), "trigger")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, ae.HTTPStatus)
	assert.Equal(t, "CI_ENGINE_RUN_INVALID_INPUT", ae.Code)
}

func TestMapEngineError_Unavailable(t *testing.T) {
	err := mapEngineError(fmt.Errorf("w: %w", engine.ErrUnavailable), "stream logs")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, ae.HTTPStatus)
	assert.Equal(t, "CI_ENGINE_UNAVAILABLE", ae.Code)
}

func TestMapEngineError_Unsupported(t *testing.T) {
	err := mapEngineError(fmt.Errorf("w: %w", engine.ErrUnsupported), "artifacts")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotImplemented, ae.HTTPStatus)
	assert.Equal(t, "CI_ENGINE_NOT_SUPPORTED", ae.Code)
}

func TestMapEngineError_Generic(t *testing.T) {
	raw := errors.New("something blew up")
	err := mapEngineError(raw, "cancel")
	// Generic errors are wrapped but NOT converted to AppError.
	_, ok := apierrors.As(err)
	assert.False(t, ok)
	assert.ErrorContains(t, err, "cancel")
}

// ---------------------------------------------------------------------------
// TriggerRun
// ---------------------------------------------------------------------------

func TestCIEngineService_TriggerRun_NilRequest(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.TriggerRun(context.Background(), 1, nil)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_REQUEST_NIL", ae.Code)
}

func TestCIEngineService_TriggerRun_ConfigNotFound(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WithArgs(uint(99), 1).WillReturnError(gormErrRecordNotFound())

	_, err := svc.TriggerRun(context.Background(), 99, &engine.TriggerRequest{})
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_NOT_FOUND", ae.Code)
}

func TestCIEngineService_TriggerRun_AdapterError(t *testing.T) {
	stub := &runStubAdapter{triggerErr: fmt.Errorf("w: %w", engine.ErrUnavailable)}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	_, err := svc.TriggerRun(context.Background(), 1, &engine.TriggerRequest{})
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_UNAVAILABLE", ae.Code)
}

func TestCIEngineService_TriggerRun_Success(t *testing.T) {
	stub := &runStubAdapter{
		triggerResult: &engine.TriggerResult{RunID: "pipeline-42", ExternalID: "42"},
	}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	res, err := svc.TriggerRun(context.Background(), 1, &engine.TriggerRequest{Ref: "main"})
	require.NoError(t, err)
	assert.Equal(t, "pipeline-42", res.RunID)
	assert.Equal(t, "42", res.ExternalID)
}

// ---------------------------------------------------------------------------
// GetRun
// ---------------------------------------------------------------------------

func TestCIEngineService_GetRun_EmptyRunID(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.GetRun(context.Background(), 1, "")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_ID_REQUIRED", ae.Code)
}

func TestCIEngineService_GetRun_NotFound(t *testing.T) {
	stub := &runStubAdapter{runErr: fmt.Errorf("w: %w", engine.ErrNotFound)}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	_, err := svc.GetRun(context.Background(), 1, "pipeline-99")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_NOT_FOUND", ae.Code)
}

func TestCIEngineService_GetRun_Success(t *testing.T) {
	stub := &runStubAdapter{
		runStatus: &engine.RunStatus{RunID: "pipeline-1", Phase: engine.RunPhaseRunning},
	}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	status, err := svc.GetRun(context.Background(), 1, "pipeline-1")
	require.NoError(t, err)
	assert.Equal(t, engine.RunPhaseRunning, status.Phase)
}

// ---------------------------------------------------------------------------
// CancelRun
// ---------------------------------------------------------------------------

func TestCIEngineService_CancelRun_EmptyRunID(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	err := svc.CancelRun(context.Background(), 1, "")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_ID_REQUIRED", ae.Code)
}

func TestCIEngineService_CancelRun_AdapterError(t *testing.T) {
	stub := &runStubAdapter{cancelErr: fmt.Errorf("w: %w", engine.ErrNotFound)}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	err := svc.CancelRun(context.Background(), 1, "pipeline-1")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_NOT_FOUND", ae.Code)
}

func TestCIEngineService_CancelRun_Success(t *testing.T) {
	stub := &runStubAdapter{cancelErr: nil}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	require.NoError(t, svc.CancelRun(context.Background(), 1, "pipeline-1"))
}

// ---------------------------------------------------------------------------
// StreamLogs
// ---------------------------------------------------------------------------

func TestCIEngineService_StreamLogs_EmptyRunID(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.StreamLogs(context.Background(), 1, "", "")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_ID_REQUIRED", ae.Code)
}

func TestCIEngineService_StreamLogs_Unsupported(t *testing.T) {
	stub := &runStubAdapter{logsErr: fmt.Errorf("w: %w", engine.ErrUnsupported)}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	_, err := svc.StreamLogs(context.Background(), 1, "pipeline-1", "")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_NOT_SUPPORTED", ae.Code)
	assert.Equal(t, http.StatusNotImplemented, ae.HTTPStatus)
}

func TestCIEngineService_StreamLogs_Success(t *testing.T) {
	content := "step 1: building...\nstep 1: done\n"
	stub := &runStubAdapter{logsRC: io.NopCloser(strings.NewReader(content))}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	rc, err := svc.StreamLogs(context.Background(), 1, "pipeline-1", "job-55")
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
}

// ---------------------------------------------------------------------------
// GetArtifacts
// ---------------------------------------------------------------------------

func TestCIEngineService_GetArtifacts_EmptyRunID(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.GetArtifacts(context.Background(), 1, "")
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_RUN_ID_REQUIRED", ae.Code)
}

func TestCIEngineService_GetArtifacts_Success(t *testing.T) {
	stub := &runStubAdapter{
		artifacts: []*engine.Artifact{
			{Name: "build.jar", Kind: "file", SizeBytes: 1024},
		},
	}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	arts, err := svc.GetArtifacts(context.Background(), 1, "pipeline-1")
	require.NoError(t, err)
	require.Len(t, arts, 1)
	assert.Equal(t, "build.jar", arts[0].Name)
}

func TestCIEngineService_GetArtifacts_EmptyList(t *testing.T) {
	stub := &runStubAdapter{artifacts: []*engine.Artifact{}}
	svc, mock, cleanup := newRunSvcWithStub(t, stub)
	defer cleanup()

	expectGetCfg(mock, 1, "gitlab")

	arts, err := svc.GetArtifacts(context.Background(), 1, "pipeline-1")
	require.NoError(t, err)
	assert.Empty(t, arts)
}
