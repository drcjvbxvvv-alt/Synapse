package services

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newCIEngineServiceTest wires up sqlmock + GORM + an isolated engine.Factory.
func newCIEngineServiceTest(t *testing.T) (*CIEngineService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)

	f := engine.NewFactory()
	svc := NewCIEngineService(gormDB, f)

	cleanup := func() {
		sqlDB, _ := gormDB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

// fakeRunner is a minimal NativeRunner for wiring tests.
type fakeRunner struct{}

func (fakeRunner) Trigger(context.Context, *engine.TriggerRequest) (*engine.TriggerResult, error) {
	return &engine.TriggerResult{RunID: "r-1"}, nil
}
func (fakeRunner) GetRun(_ context.Context, runID string) (*engine.RunStatus, error) {
	return &engine.RunStatus{RunID: runID, Phase: engine.RunPhaseRunning}, nil
}
func (fakeRunner) Cancel(context.Context, string) error { return nil }

// stubExternalBuilder produces an adapter that reports whatever availability
// and version the test requests.
type stubExternalAdapter struct {
	typ     engine.EngineType
	avail   bool
	version string
	err     error
}

func (s *stubExternalAdapter) Type() engine.EngineType                        { return s.typ }
func (s *stubExternalAdapter) IsAvailable(context.Context) bool               { return s.avail }
func (s *stubExternalAdapter) Version(context.Context) (string, error)        { return s.version, s.err }
func (s *stubExternalAdapter) Capabilities() engine.EngineCapabilities {
	return engine.EngineCapabilities{SupportsDAG: true}
}
func (s *stubExternalAdapter) Trigger(context.Context, *engine.TriggerRequest) (*engine.TriggerResult, error) {
	return nil, nil
}
func (s *stubExternalAdapter) GetRun(context.Context, string) (*engine.RunStatus, error) {
	return nil, nil
}
func (s *stubExternalAdapter) Cancel(context.Context, string) error { return nil }
func (s *stubExternalAdapter) StreamLogs(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (s *stubExternalAdapter) GetArtifacts(context.Context, string) ([]*engine.Artifact, error) {
	return []*engine.Artifact{}, nil
}

// ---------------------------------------------------------------------------
// validateRequest / error paths
// ---------------------------------------------------------------------------

func TestCIEngineService_Create_InvalidType(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.Create(context.Background(), &models.CIEngineConfigRequest{
		Name:       "bad",
		EngineType: "circleci",
	}, 1)
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_TYPE_INVALID", ae.Code)
}

func TestCIEngineService_Create_EmptyName(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.Create(context.Background(), &models.CIEngineConfigRequest{
		EngineType: "gitlab",
	}, 1)
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_NAME_REQUIRED", ae.Code)
}

func TestCIEngineService_Create_RejectsNativeEngine(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.Create(context.Background(), &models.CIEngineConfigRequest{
		Name:       "native-dummy",
		EngineType: "native",
	}, 1)
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_NATIVE_NOT_STORED", ae.Code)
}

func TestCIEngineService_Create_NilRequest(t *testing.T) {
	svc, _, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	_, err := svc.Create(context.Background(), nil, 1)
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_REQUEST_NIL", ae.Code)
}

// ---------------------------------------------------------------------------
// CRUD happy paths
// ---------------------------------------------------------------------------

func TestCIEngineService_Create_Success(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	cfg, err := svc.Create(context.Background(), &models.CIEngineConfigRequest{
		Name:       "gitlab-prod",
		EngineType: "gitlab",
		Endpoint:   "https://gitlab.example.com",
		AuthType:   models.CIEngineAuthToken,
		Token:      "pat-123",
	}, 42)
	require.NoError(t, err)
	assert.Equal(t, uint(1), cfg.ID)
	assert.Equal(t, "gitlab-prod", cfg.Name)
	assert.Equal(t, uint(42), cfg.CreatedBy)
}

func TestCIEngineService_Get_NotFound(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs" WHERE "ci_engine_configs"."id" = $1`)).
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Get(context.Background(), 999)
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_NOT_FOUND", ae.Code)
}

func TestCIEngineService_Update_EngineTypeImmutable(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "name", "engine_type"}).
		AddRow(1, "gl", "gitlab")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WithArgs(uint(1), 1).
		WillReturnRows(rows)

	_, err := svc.Update(context.Background(), 1, &models.CIEngineConfigRequest{
		Name:       "gl",
		EngineType: "jenkins", // different type → must fail
	})
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_TYPE_IMMUTABLE", ae.Code)
}

func TestCIEngineService_Delete_PropagatesNotFound(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WithArgs(uint(77), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.Delete(context.Background(), 77)
	require.Error(t, err)
	ae, ok := apierrors.As(err)
	require.True(t, ok)
	assert.Equal(t, "CI_ENGINE_NOT_FOUND", ae.Code)
}

// ---------------------------------------------------------------------------
// Probing / ListAvailableEngines
// ---------------------------------------------------------------------------

func TestCIEngineService_ListAvailableEngines_NativeNotRegistered(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	// No external engines configured.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1) // only native
	assert.Equal(t, "native", got[0].Type)
	assert.False(t, got[0].Available)
	assert.Contains(t, got[0].Error, "not registered")
}

func TestCIEngineService_ListAvailableEngines_IncludesNativeAndExternal(t *testing.T) {
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	// Register native.
	err := engine.RegisterNative(svc.Factory(), fakeRunner{}, "v-test")
	require.NoError(t, err)

	// Register a stub gitlab adapter.
	err = svc.Factory().Register(engine.EngineGitLab, func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return &stubExternalAdapter{typ: engine.EngineGitLab, avail: true, version: "16.0.0"}, nil
	})
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "engine_type", "enabled",
		}).AddRow(1, "gitlab-main", "gitlab", true))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Native first (the service intentionally lists it at the top).
	assert.Equal(t, "native", got[0].Type)
	assert.True(t, got[0].Available)
	assert.Equal(t, "v-test", got[0].Version)
	assert.True(t, got[0].Default)

	// External gitlab second.
	assert.Equal(t, "gitlab", got[1].Type)
	assert.True(t, got[1].Available)
	assert.Equal(t, "16.0.0", got[1].Version)
	assert.Equal(t, "gitlab-main", got[1].Name)
}

func TestCIEngineService_ListAvailableEngines_PerEngineErrorsAreScoped(t *testing.T) {
	// A failing builder for one external engine must NOT fail the whole
	// response; it surfaces as EngineStatus.Error per CLAUDE §8 Observer
	// Pattern.
	svc, mock, cleanup := newCIEngineServiceTest(t)
	defer cleanup()

	buildErr := errors.New("gitlab connection refused")
	require.NoError(t, svc.Factory().Register(engine.EngineGitLab, func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return nil, buildErr
	}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "engine_type", "enabled",
		}).AddRow(1, "gl", "gitlab", true))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err) // overall call succeeds
	require.Len(t, got, 2)
	assert.Equal(t, "gitlab", got[1].Type)
	assert.False(t, got[1].Available)
	assert.Contains(t, got[1].Error, "gitlab connection refused")
}
