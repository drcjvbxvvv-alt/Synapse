package repositories_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shaia/Synapse/internal/repositories"
)

// clusterRows builds a sqlmock rows object matching the columns of
// models.Cluster. Kept in one place so individual tests stay compact.
func clusterRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "api_server", "kubeconfig_enc", "ca_enc", "sa_token_enc",
		"version", "status", "labels", "cert_expire_at", "last_heartbeat",
		"created_by", "created_at", "updated_at", "deleted_at",
		"monitoring_config", "alert_manager_config",
	})
}

func addClusterRow(rows *sqlmock.Rows, id uint, name, status string) *sqlmock.Rows {
	now := time.Now()
	return rows.AddRow(
		id, name, "https://k8s.example:6443", "", "", "",
		"v1.29.0", status, "{}", nil, now,
		0, now, now, nil,
		"null", "null",
	)
}

func TestClusterRepository_FindByName_Success(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	rows := addClusterRow(clusterRows(), 1, "prod-01", "healthy")
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `clusters` WHERE name = ? AND `clusters`.`deleted_at` IS NULL ORDER BY `clusters`.`id` LIMIT ?",
	)).WithArgs("prod-01", 1).WillReturnRows(rows)

	repo := repositories.NewClusterRepository(gdb)
	got, err := repo.FindByName(context.Background(), "prod-01")

	require.NoError(t, err)
	assert.Equal(t, "prod-01", got.Name)
	assert.Equal(t, "healthy", got.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClusterRepository_FindByName_Empty(t *testing.T) {
	gdb, _, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewClusterRepository(gdb)
	_, err := repo.FindByName(context.Background(), "")

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrInvalidArgument)
}

func TestClusterRepository_ListConnectable(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	rows := clusterRows()
	addClusterRow(rows, 1, "prod-01", "healthy")
	addClusterRow(rows, 2, "stg-01", "unknown")

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `clusters` WHERE status != ? AND `clusters`.`deleted_at` IS NULL",
	)).WithArgs("unhealthy").WillReturnRows(rows)

	repo := repositories.NewClusterRepository(gdb)
	got, err := repo.ListConnectable(context.Background())

	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClusterRepository_FindByIDs_EmptyShortCircuit(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewClusterRepository(gdb)
	got, err := repo.FindByIDs(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, got)
	// Zero expectations: no query should hit the DB.
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClusterRepository_FindByIDs_WithIDs(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	rows := clusterRows()
	addClusterRow(rows, 1, "prod-01", "healthy")
	addClusterRow(rows, 2, "prod-02", "healthy")

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `clusters` WHERE id IN (?,?) AND `clusters`.`deleted_at` IS NULL",
	)).WithArgs(1, 2).WillReturnRows(rows)

	repo := repositories.NewClusterRepository(gdb)
	got, err := repo.FindByIDs(context.Background(), []uint{1, 2})

	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClusterRepository_CountByStatus(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `clusters` WHERE status = ? AND `clusters`.`deleted_at` IS NULL",
	)).WithArgs("healthy").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(5),
	)

	repo := repositories.NewClusterRepository(gdb)
	count, err := repo.CountByStatus(context.Background(), "healthy")

	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}
