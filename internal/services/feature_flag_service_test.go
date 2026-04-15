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

func newFeatureFlagService(t *testing.T) (*FeatureFlagService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := NewFeatureFlagService(gormDB)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

var featureFlagCols = []string{"key", "enabled", "description", "updated_by", "created_at", "updated_at"}

// ─── ListFlags ────────────────────────────────────────────────────────────────

func TestFeatureFlagService_List_Empty(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(featureFlagCols))

	flags, err := svc.ListFlags(context.Background())
	require.NoError(t, err)
	assert.Empty(t, flags)
}

func TestFeatureFlagService_List_Multiple(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows(featureFlagCols).
		AddRow("feature-a", true, "desc a", "admin", now, now).
		AddRow("feature-b", false, "desc b", "admin", now, now)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	flags, err := svc.ListFlags(context.Background())
	require.NoError(t, err)
	assert.Len(t, flags, 2)
	assert.Equal(t, "feature-a", flags[0].Key)
	assert.True(t, flags[0].Enabled)
	assert.Equal(t, "feature-b", flags[1].Key)
	assert.False(t, flags[1].Enabled)
}

func TestFeatureFlagService_List_DBError(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidData)

	_, err := svc.ListFlags(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list feature flags")
}

// ─── SetFlag (create path) ────────────────────────────────────────────────────

func TestFeatureFlagService_SetFlag_Create(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	// First call: WHERE key = ? → not found
	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)
	// Then insert (string PK — no RETURNING, uses Exec)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO .feature_flags.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.SetFlag(context.Background(), "new-flag", true, "a new flag", "admin")
	assert.NoError(t, err)
}

func TestFeatureFlagService_SetFlag_CreateDBError(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO .feature_flags.`).WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.SetFlag(context.Background(), "bad-flag", true, "", "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create feature flag")
}

// ─── SetFlag (update path) ────────────────────────────────────────────────────

func TestFeatureFlagService_SetFlag_Update(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	now := time.Now()
	// First call: WHERE key = ? → found
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(featureFlagCols).AddRow("feature-a", true, "old desc", "user1", now, now),
	)
	// Save (UPDATE)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .feature_flags.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.SetFlag(context.Background(), "feature-a", false, "new desc", "admin")
	assert.NoError(t, err)
}

func TestFeatureFlagService_SetFlag_UpdateDBError(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`SELECT`).WillReturnRows(
		sqlmock.NewRows(featureFlagCols).AddRow("feature-a", true, "desc", "user1", now, now),
	)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .feature_flags.`).WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.SetFlag(context.Background(), "feature-a", false, "", "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update feature flag")
}

func TestFeatureFlagService_SetFlag_QueryError(t *testing.T) {
	svc, mock, cleanup := newFeatureFlagService(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidTransaction)

	err := svc.SetFlag(context.Background(), "feature-a", true, "", "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query feature flag")
}
