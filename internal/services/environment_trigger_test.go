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

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// EnqueueRunInEnvironment
// ---------------------------------------------------------------------------

func newSchedulerWithMock(t *testing.T) (*PipelineScheduler, *EnvironmentService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	sched := &PipelineScheduler{
		db:  gormDB,
		cfg: DefaultSchedulerConfig(),
	}
	envSvc := NewEnvironmentService(gormDB)

	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return sched, envSvc, mock, cleanup
}

func TestEnqueueRunInEnvironment_Success(t *testing.T) {
	sched, envSvc, mock, cleanup := newSchedulerWithMock(t)
	defer cleanup()

	now := time.Now()
	currentVersionID := uint(5)

	// 1. GetEnvironment → SELECT environments
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(envCols()).AddRow(
			10, "dev", 1, 2, "app-dev",
			1, false, false, "", "", "", "{}", now, now, nil,
		),
	)

	// 2. Get Pipeline → SELECT pipelines
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "current_version_id", "concurrency_group", "max_concurrent_runs"}).
			AddRow(1, currentVersionID, "", 1),
	)

	// 3. EnqueueRun → count queued
	mock.ExpectQuery(`SELECT count`).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(0),
	)

	// 4. INSERT pipeline_runs
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_runs.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectCommit()

	run, err := sched.EnqueueRunInEnvironment(context.Background(), envSvc, TriggerRunInput{
		PipelineID:  1,
		EnvID:       10,
		UserID:      7,
		TriggerType: models.TriggerTypeManual,
	})

	require.NoError(t, err)
	assert.NotNil(t, run)
	assert.Equal(t, uint(42), run.ID)
	assert.Equal(t, uint(2), run.ClusterID)   // from environment
	assert.Equal(t, "app-dev", run.Namespace) // from environment
	assert.Equal(t, uint(currentVersionID), run.SnapshotID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnqueueRunInEnvironment_EnvNotFound(t *testing.T) {
	sched, envSvc, mock, cleanup := newSchedulerWithMock(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	run, err := sched.EnqueueRunInEnvironment(context.Background(), envSvc, TriggerRunInput{
		PipelineID:  1,
		EnvID:       999,
		UserID:      7,
		TriggerType: models.TriggerTypeManual,
	})

	assert.Error(t, err)
	assert.Nil(t, run)
	assert.Contains(t, err.Error(), "resolve environment")
}

func TestEnqueueRunInEnvironment_PipelineNoVersion(t *testing.T) {
	sched, envSvc, mock, cleanup := newSchedulerWithMock(t)
	defer cleanup()

	now := time.Now()

	// GetEnvironment OK
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(envCols()).AddRow(
			10, "dev", 1, 2, "app-dev",
			1, false, false, "", "", "", "{}", now, now, nil,
		),
	)

	// Get Pipeline — no current_version_id (NULL)
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "current_version_id", "concurrency_group", "max_concurrent_runs"}).
			AddRow(1, nil, "", 1),
	)

	run, err := sched.EnqueueRunInEnvironment(context.Background(), envSvc, TriggerRunInput{
		PipelineID:  1,
		EnvID:       10,
		UserID:      7,
		TriggerType: models.TriggerTypeManual,
	})

	assert.Error(t, err)
	assert.Nil(t, run)
	assert.Contains(t, err.Error(), "no active version")
}

func TestEnqueueRunInEnvironment_WithExplicitVersionID(t *testing.T) {
	sched, envSvc, mock, cleanup := newSchedulerWithMock(t)
	defer cleanup()

	now := time.Now()
	explicitVersion := uint(3)

	// GetEnvironment
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(envCols()).AddRow(
			10, "staging", 1, 3, "app-staging",
			2, false, false, "", "", "", "{}", now, now, nil,
		),
	)

	// Get Pipeline (current version is 5, but we pass explicit 3)
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "current_version_id", "concurrency_group", "max_concurrent_runs"}).
			AddRow(1, uint(5), "", 1),
	)

	// Count queued
	mock.ExpectQuery(`SELECT count`).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(0),
	)

	// INSERT
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .pipeline_runs.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	mock.ExpectCommit()

	run, err := sched.EnqueueRunInEnvironment(context.Background(), envSvc, TriggerRunInput{
		PipelineID:  1,
		EnvID:       10,
		VersionID:   &explicitVersion,
		UserID:      7,
		TriggerType: models.TriggerTypeWebhook,
	})

	require.NoError(t, err)
	assert.Equal(t, uint(3), run.SnapshotID) // explicit version used
	assert.Equal(t, uint(3), run.ClusterID)  // from staging environment
	assert.Equal(t, "app-staging", run.Namespace)
	assert.NoError(t, mock.ExpectationsWereMet())
}
