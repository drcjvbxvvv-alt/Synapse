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

// userRows returns an empty sqlmock rows object with every column that
// models.User persists. Tests compose rows via addUserRow.
func userRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"phone", "auth_type", "status", "system_role",
		"last_login_at", "last_login_ip", "created_at", "updated_at", "deleted_at",
	})
}

func addUserRow(rows *sqlmock.Rows, id uint, username, status, authType string) *sqlmock.Rows {
	now := time.Now()
	return rows.AddRow(
		id, username, "hash", "salt", username+"@example.com", username,
		"", authType, status, "user",
		now, "", now, now, nil,
	)
}

func TestUserRepository_FindByUsername_Success(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	rows := addUserRow(userRows(), 1, "alice", "active", "local")
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `users` WHERE username = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?",
	)).WithArgs("alice", 1).WillReturnRows(rows)

	repo := repositories.NewUserRepository(gdb)
	got, err := repo.FindByUsername(context.Background(), "alice")

	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "local", got.AuthType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_FindByUsername_Empty(t *testing.T) {
	gdb, _, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	repo := repositories.NewUserRepository(gdb)
	_, err := repo.FindByUsername(context.Background(), "")

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrInvalidArgument)
}

func TestUserRepository_FindByUsername_NotFound(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `users` WHERE username = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?",
	)).WithArgs("ghost", 1).WillReturnRows(userRows())

	repo := repositories.NewUserRepository(gdb)
	_, err := repo.FindByUsername(context.Background(), "ghost")

	require.Error(t, err)
	assert.ErrorIs(t, err, repositories.ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_ListPaged_Search(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	// Count comes first (ListOptions path), then the SELECT.
	// GORM wraps the outer WHERE clause in parentheses.
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `users` WHERE ((username LIKE ? OR display_name LIKE ? OR email LIKE ?)) AND `users`.`deleted_at` IS NULL",
	)).WithArgs("%ali%", "%ali%", "%ali%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	rows := addUserRow(userRows(), 1, "alice", "active", "local")
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `users` WHERE ((username LIKE ? OR display_name LIKE ? OR email LIKE ?)) AND `users`.`deleted_at` IS NULL ORDER BY id ASC LIMIT ?",
	)).WithArgs("%ali%", "%ali%", "%ali%", 20).WillReturnRows(rows)

	repo := repositories.NewUserRepository(gdb)
	got, total, err := repo.ListPaged(context.Background(), repositories.ListUsersFilter{
		Page:     1,
		PageSize: 20,
		Search:   "ali",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, got, 1)
	assert.Equal(t, "alice", got[0].Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_ListPaged_StatusAndAuthFilter(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	// GORM wraps the outer WHERE clause in parentheses.
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT count(*) FROM `users` WHERE (status = ? AND auth_type = ?) AND `users`.`deleted_at` IS NULL",
	)).WithArgs("active", "ldap").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `users` WHERE (status = ? AND auth_type = ?) AND `users`.`deleted_at` IS NULL ORDER BY id ASC LIMIT ?",
	)).WithArgs("active", "ldap", 10).WillReturnRows(userRows())

	repo := repositories.NewUserRepository(gdb)
	got, total, err := repo.ListPaged(context.Background(), repositories.ListUsersFilter{
		Page:     1,
		PageSize: 10,
		Status:   "active",
		AuthType: "ldap",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}
