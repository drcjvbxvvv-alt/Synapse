package repositories_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/repositories"
)

// permRows builds an empty sqlmock rows object with every column persisted
// by models.ClusterPermission.
func permRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "cluster_id", "user_id", "user_group_id", "permission_type",
		"namespaces", "custom_role_ref", "created_at", "updated_at", "deleted_at",
	})
}

func addPermRow(rows *sqlmock.Rows, id, clusterID uint, userID *uint, permType string) *sqlmock.Rows {
	now := time.Now()
	return rows.AddRow(
		id, clusterID, userID, nil, permType,
		`["*"]`, "", now, now, nil,
	)
}

func TestPermissionRepository_GetWithRelations_ZeroID(t *testing.T) {
	gdb, _, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewPermissionRepository(gdb)
	_, err := repo.GetWithRelations(context.Background(), 0)

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrInvalidArgument)
}

func TestPermissionRepository_FindByClusterUser_NotFound(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `cluster_permissions` WHERE (cluster_id = ? AND user_id = ?) AND `cluster_permissions`.`deleted_at` IS NULL ORDER BY `cluster_permissions`.`id` LIMIT ?",
	)).WithArgs(1, 2, 1).WillReturnError(gorm.ErrRecordNotFound)

	repo := repositories.NewPermissionRepository(gdb)
	_, err := repo.FindByClusterUser(context.Background(), 1, 2)

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_FindByClusterGroups_EmptyShortCircuit(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewPermissionRepository(gdb)
	_, err := repo.FindByClusterGroups(context.Background(), 1, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	// No query should have hit the DB.
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_ExistsForClusterUser_True(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `cluster_permissions` WHERE (cluster_id = ? AND user_id = ?) AND `cluster_permissions`.`deleted_at` IS NULL",
	)).WithArgs(1, 2).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)

	repo := repositories.NewPermissionRepository(gdb)
	ok, err := repo.ExistsForClusterUser(context.Background(), 1, 2)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_CountAdminByUser(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `cluster_permissions` WHERE (user_id = ? AND permission_type = ?) AND `cluster_permissions`.`deleted_at` IS NULL",
	)).WithArgs(7, "admin").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(3),
	)

	repo := repositories.NewPermissionRepository(gdb)
	count, err := repo.CountAdminByUser(context.Background(), 7)

	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_CountAdminByGroups_EmptyShortCircuit(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewPermissionRepository(gdb)
	count, err := repo.CountAdminByGroups(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_BatchDeletePermissions_EmptyShortCircuit(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewPermissionRepository(gdb)
	affected, err := repo.BatchDeletePermissions(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, int64(0), affected)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_AddUserToGroup_Idempotent(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	// First call to AddUserToGroup: membership already exists, count = 1,
	// repository returns nil without issuing an INSERT.
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `user_group_members` WHERE user_id = ? AND user_group_id = ?",
	)).WithArgs(1, 2).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)

	repo := repositories.NewPermissionRepository(gdb)
	err := repo.AddUserToGroup(context.Background(), 1, 2)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_AddUserToGroup_NewMembership(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	// Exists-check returns 0, so the repo performs an INSERT.
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `user_group_members` WHERE user_id = ? AND user_group_id = ?",
	)).WithArgs(1, 2).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(0),
	)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		"INSERT INTO `user_group_members`",
	)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := repositories.NewPermissionRepository(gdb)
	err := repo.AddUserToGroup(context.Background(), 1, 2)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_DeleteUserGroupTx_Success(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM `user_group_members` WHERE user_group_id = ?",
	)).WithArgs(5).WillReturnResult(sqlmock.NewResult(0, 2))
	// UserGroup has soft-delete (gorm.DeletedAt), so the final delete is an
	// UPDATE SET deleted_at=? rather than a DELETE FROM.
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE `user_groups` SET `deleted_at`=? WHERE `user_groups`.`id` = ? AND `user_groups`.`deleted_at` IS NULL",
	)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := repositories.NewPermissionRepository(gdb)
	err := repo.DeleteUserGroupTx(context.Background(), 5)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_ListGroupIDsForUser(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT `user_group_id` FROM `user_group_members` WHERE user_id = ?",
	)).WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"user_group_id"}).AddRow(10).AddRow(20),
	)

	repo := repositories.NewPermissionRepository(gdb)
	ids, err := repo.ListGroupIDsForUser(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, []uint{10, 20}, ids)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_ListByCluster_AllClusters(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	rows := permRows()
	uid := uint(1)
	addPermRow(rows, 1, 10, &uid, "admin")

	// clusterID = 0 means "all clusters" — no WHERE cluster_id filter, but
	// the preload queries for User/UserGroup are wired in. We match only
	// the main query; unmatched preload queries would fail the test.
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `cluster_permissions` WHERE `cluster_permissions`.`deleted_at` IS NULL",
	)).WillReturnRows(rows)
	// Preloads: user_id=1 → users lookup; user_group_id IS NULL → groups
	// lookup never runs because all UserGroupIDs are nil.
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL",
	)).WithArgs(1).WillReturnRows(userRows())

	repo := repositories.NewPermissionRepository(gdb)
	got, err := repo.ListByCluster(context.Background(), 0)

	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "admin", got[0].PermissionType)
	assert.NoError(t, mock.ExpectationsWereMet())
}
