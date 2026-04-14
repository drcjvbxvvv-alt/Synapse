package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/repositories"
)

// fixture model used only by this test — kept local so base_test has no
// dependency on the production models package.
type fixture struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"size:64"`
	CreatedAt time.Time
}

func (fixture) TableName() string { return "fixtures" }

// newMockDB wires up gorm on top of go-sqlmock and returns both handles.
func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	dialector := postgres.New(postgres.Config{
		Conn:                      sqlDB,
		PreferSimpleProtocol: true,
	})
	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	return gdb, mock, sqlDB
}

func TestBaseRepository_Get_Success(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "fixtures" WHERE "fixtures"."id" = $1 ORDER BY "fixtures"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).
			AddRow(7, "alpha", time.Now()))

	repo := repositories.NewBaseRepository[fixture](gdb)
	got, err := repo.Get(context.Background(), 7)

	require.NoError(t, err)
	assert.Equal(t, uint(7), got.ID)
	assert.Equal(t, "alpha", got.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBaseRepository_Get_NotFound(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "fixtures" WHERE "fixtures"."id" = $1 ORDER BY "fixtures"."id" LIMIT $2`)).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	repo := repositories.NewBaseRepository[fixture](gdb)
	_, err := repo.Get(context.Background(), 99)

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBaseRepository_Get_ZeroID(t *testing.T) {
	gdb, _, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewBaseRepository[fixture](gdb)
	_, err := repo.Get(context.Background(), 0)

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrInvalidArgument)
}

func TestBaseRepository_Count(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "fixtures" WHERE name = $1`)).
		WithArgs("alpha").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	repo := repositories.NewBaseRepository[fixture](gdb)
	count, err := repo.Count(context.Background(), "name = ?", "alpha")

	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBaseRepository_UpdateFields_RowsAffected(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "fixtures" SET "name"=$1 WHERE id = $2`)).
		WithArgs("beta", 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := repositories.NewBaseRepository[fixture](gdb)
	affected, err := repo.UpdateFields(context.Background(), 7, map[string]any{"name": "beta"})

	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBaseRepository_Transaction_RollbackOnError(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	repo := repositories.NewBaseRepository[fixture](gdb)
	sentinel := errors.New("business rule failed")

	err := repo.Transaction(context.Background(), func(tx *gorm.DB) error {
		return sentinel
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBaseRepository_Exists_True(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "fixtures" WHERE name = $1`)).
		WithArgs("alpha").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	repo := repositories.NewBaseRepository[fixture](gdb)
	ok, err := repo.Exists(context.Background(), "name = ?", "alpha")

	require.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mock.ExpectationsWereMet())
}
