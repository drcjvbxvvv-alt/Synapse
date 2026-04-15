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

// envCols returns the column list for SELECT queries on environments.
func envCols() []string {
	return []string{
		"id", "name", "pipeline_id", "cluster_id", "namespace",
		"order_index", "auto_promote", "approval_required",
		"approver_ids", "smoke_test_step_name", "notify_channel_ids",
		"variables_json", "created_at", "updated_at", "deleted_at",
	}
}

func newEnvSvcDB(t *testing.T) (*EnvironmentService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := NewEnvironmentService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

func envRow(id, pipelineID, clusterID uint, name, ns string, orderIdx int, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(envCols()).AddRow(
		id, name, pipelineID, clusterID, ns,
		orderIdx, false, false,
		"", "", "",
		"{}", now, now, nil,
	)
}

// ─── ListEnvironments ──────────────────────────────────────────────────────

func TestEnvironmentService_ListEnvironments_Empty(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(envCols()))

	envs, err := svc.ListEnvironments(context.Background(), 1)
	assert.NoError(t, err)
	assert.Empty(t, envs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnvironmentService_ListEnvironments_Multiple(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(envCols()).
		AddRow(1, "dev", 10, 1, "app-dev", 1, false, false, "", "", "", "{}", now, now, nil).
		AddRow(2, "staging", 10, 2, "app-staging", 2, true, false, "", "", "", "{}", now, now, nil).
		AddRow(3, "prod", 10, 3, "app-prod", 3, false, true, "", "", "", "{}", now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	envs, err := svc.ListEnvironments(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, envs, 3)
	assert.Equal(t, "dev", envs[0].Name)
	assert.Equal(t, "staging", envs[1].Name)
	assert.Equal(t, "prod", envs[2].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetEnvironment ────────────────────────────────────────────────────────

func TestEnvironmentService_GetEnvironment_Found(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`SELECT`).WillReturnRows(envRow(5, 10, 1, "dev", "app-dev", 1, now))

	env, err := svc.GetEnvironment(context.Background(), 5)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	assert.Equal(t, uint(5), env.ID)
	assert.Equal(t, "dev", env.Name)
}

func TestEnvironmentService_GetEnvironment_NotFound(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	env, err := svc.GetEnvironment(context.Background(), 999)
	assert.Error(t, err)
	assert.Nil(t, env)
	assert.Contains(t, err.Error(), "not found")
}

// ─── GetEnvironmentByPipelineAndID ─────────────────────────────────────────

func TestEnvironmentService_GetEnvironmentByPipelineAndID_Found(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`SELECT`).WillReturnRows(envRow(5, 10, 1, "dev", "app-dev", 1, now))

	env, err := svc.GetEnvironmentByPipelineAndID(context.Background(), 10, 5)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	assert.Equal(t, uint(10), env.PipelineID)
}

func TestEnvironmentService_GetEnvironmentByPipelineAndID_WrongPipeline(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	env, err := svc.GetEnvironmentByPipelineAndID(context.Background(), 99, 5)
	assert.Error(t, err)
	assert.Nil(t, env)
}

// ─── CreateEnvironment ─────────────────────────────────────────────────────

func TestEnvironmentService_CreateEnvironment_Success(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	// Auto-compute order index: MAX query
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(2))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .environments.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectCommit()

	req := &CreateEnvironmentRequest{
		Name:      "dev",
		ClusterID: 1,
		Namespace: "app-dev",
		// OrderIndex == 0 → auto-compute
	}
	env, err := svc.CreateEnvironment(context.Background(), 5, req)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	assert.Equal(t, uint(10), env.ID)
	assert.Equal(t, "{}", env.VariablesJSON)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnvironmentService_CreateEnvironment_WithExplicitOrderIndex(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	// OrderIndex > 0, no auto-compute query
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .environments.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	req := &CreateEnvironmentRequest{
		Name:       "staging",
		ClusterID:  2,
		Namespace:  "app-staging",
		OrderIndex: 2,
	}
	env, err := svc.CreateEnvironment(context.Background(), 5, req)
	assert.NoError(t, err)
	assert.Equal(t, uint(11), env.ID)
	assert.Equal(t, 2, env.OrderIndex)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── UpdateEnvironment ─────────────────────────────────────────────────────

func TestEnvironmentService_UpdateEnvironment_Success(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`SELECT`).WillReturnRows(envRow(5, 10, 1, "dev", "app-dev", 1, now))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .environments.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	newNs := "app-dev-v2"
	updated, err := svc.UpdateEnvironment(context.Background(), 10, 5, &UpdateEnvironmentRequest{
		Namespace: &newNs,
	})
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, "app-dev-v2", updated.Namespace)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnvironmentService_UpdateEnvironment_NotFound(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	newNs := "app-dev"
	updated, err := svc.UpdateEnvironment(context.Background(), 99, 5, &UpdateEnvironmentRequest{
		Namespace: &newNs,
	})
	assert.Error(t, err)
	assert.Nil(t, updated)
}

// ─── DeleteEnvironment ─────────────────────────────────────────────────────

func TestEnvironmentService_DeleteEnvironment_Success(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .environments.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteEnvironment(context.Background(), 10, 5)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnvironmentService_DeleteEnvironment_NotFound(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .environments.`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeleteEnvironment(context.Background(), 10, 999)
	assert.Error(t, err)
}

// ─── GetDefaultEnvironment ─────────────────────────────────────────────────

func TestEnvironmentService_GetDefaultEnvironment_Found(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`SELECT`).WillReturnRows(envRow(1, 10, 1, "dev", "app-dev", 1, now))

	env, err := svc.GetDefaultEnvironment(context.Background(), 10)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	assert.Equal(t, "dev", env.Name)
	assert.Equal(t, 1, env.OrderIndex)
}

func TestEnvironmentService_GetDefaultEnvironment_NoneExist(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	env, err := svc.GetDefaultEnvironment(context.Background(), 99)
	assert.Error(t, err)
	assert.Nil(t, env)
	assert.Contains(t, err.Error(), "no environments")
}

// ─── GetNextEnvironment ─────────────────────────────────────────────────────

func TestEnvironmentService_GetNextEnvironment_Found(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	now := time.Now()
	// current order_index=1 → next is staging (order_index=2)
	mock.ExpectQuery(`SELECT`).WillReturnRows(envRow(2, 10, 2, "staging", "app-staging", 2, now))

	next, err := svc.GetNextEnvironment(context.Background(), 10, 1)
	assert.NoError(t, err)
	assert.NotNil(t, next)
	assert.Equal(t, "staging", next.Name)
	assert.Equal(t, 2, next.OrderIndex)
}

func TestEnvironmentService_GetNextEnvironment_LastEnv(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	// Already last environment — no record found
	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	next, err := svc.GetNextEnvironment(context.Background(), 10, 3)
	assert.NoError(t, err) // not an error — just nil
	assert.Nil(t, next)
}

// ─── RecordPromotion ───────────────────────────────────────────────────────

func TestEnvironmentService_RecordPromotion_Success(t *testing.T) {
	svc, mock, cleanup := newEnvSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO .promotion_history.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	h := &models.PromotionHistory{
		PipelineID:      10,
		PipelineRunID:   5,
		FromEnvironment: "dev",
		ToEnvironment:   "staging",
		Status:          models.PromotionStatusAutoPromoted,
	}
	err := svc.RecordPromotion(context.Background(), h)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
