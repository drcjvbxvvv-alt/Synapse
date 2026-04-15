package services

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	permModels "github.com/shaia/Synapse/internal/models"
)

func newPermSvcDB(t *testing.T) (*PermissionService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	svc := NewPermissionService(gormDB, nil)
	cleanup := func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return svc, mock, cleanup
}

// ─── UpdateUserGroup ────────────────────────────────────────────────────────

func TestPermissionService_UpdateUserGroup_Success(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "old-name", "old-desc", now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .user_groups.`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	group, err := svc.UpdateUserGroup(1, "new-name", "new-desc")
	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, "new-name", group.Name)
}

func TestPermissionService_UpdateUserGroup_NotFound(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	group, err := svc.UpdateUserGroup(999, "x", "y")
	assert.Error(t, err)
	assert.Nil(t, group)
}

// ─── AddUserToGroup ─────────────────────────────────────────────────────────

func TestPermissionService_AddUserToGroup_Success(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	now := time.Now()
	// 1. Check user exists
	userRows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "system_role", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(1, "alice", "hash", "salt", "alice@example.com", "Alice",
		"local", "active", "user", now, "", now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(userRows)

	// 2. Check group exists
	groupRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(2, "dev-team", "", now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(groupRows)

	// 3. Count existing membership (0 → not already in group)
	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 4. Insert membership (composite PK → no RETURNING, uses Exec not Query)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO .user_group_members.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.AddUserToGroup(1, 2)
	assert.NoError(t, err)
}

func TestPermissionService_AddUserToGroup_AlreadyMember(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	now := time.Now()
	userRows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "system_role", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(1, "alice", "hash", "salt", "alice@example.com", "Alice",
		"local", "active", "user", now, "", now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(userRows)

	groupRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(2, "dev-team", "", now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(groupRows)

	// Already in group (count=1) → skip without error
	mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	err := svc.AddUserToGroup(1, 2)
	assert.NoError(t, err)
}

func TestPermissionService_AddUserToGroup_UserNotFound(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	err := svc.AddUserToGroup(999, 2)
	assert.Error(t, err)
}

// ─── RemoveUserFromGroup ────────────────────────────────────────────────────

func TestPermissionService_RemoveUserFromGroup_Success(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM .user_group_members.`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.RemoveUserFromGroup(1, 2)
	assert.NoError(t, err)
}

// ─── DeleteClusterPermission ────────────────────────────────────────────────

func TestPermissionService_DeleteClusterPermission_Success(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	// Soft delete → UPDATE SET deleted_at
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .cluster_permissions.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteClusterPermission(5)
	assert.NoError(t, err)
}

func TestPermissionService_DeleteClusterPermission_NotFound(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	// Soft delete → 0 rows affected
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .cluster_permissions.`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.DeleteClusterPermission(999)
	assert.Error(t, err)
}

// ─── GetClusterPermission ───────────────────────────────────────────────────

func TestPermissionService_GetClusterPermission_Found(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	now := time.Now()
	permRows := sqlmock.NewRows([]string{
		"id", "cluster_id", "user_id", "user_group_id", "permission_type",
		"namespaces", "created_at", "updated_at", "deleted_at",
	}).AddRow(3, 1, 1, nil, permModels.PermissionTypeAdmin, `["*"]`, now, now, nil)

	// Preload queries + main query
	mock.ExpectQuery(`SELECT`).WillReturnRows(permRows)
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil)) // User preload
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil)) // UserGroup preload
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil)) // Cluster preload

	perm, err := svc.GetClusterPermission(3)
	assert.NoError(t, err)
	assert.NotNil(t, perm)
}

func TestPermissionService_GetClusterPermission_NotFound(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	perm, err := svc.GetClusterPermission(999)
	assert.Error(t, err)
	assert.Nil(t, perm)
}

// ─── ListClusterPermissions ─────────────────────────────────────────────────

func TestPermissionService_ListClusterPermissions_Success(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "cluster_id", "user_id", "user_group_id", "permission_type",
		"namespaces", "created_at", "updated_at", "deleted_at",
	}).
		AddRow(1, 10, 1, nil, permModels.PermissionTypeAdmin, `["*"]`, now, now, nil).
		AddRow(2, 10, 2, nil, permModels.PermissionTypeReadonly, `["default"]`, now, now, nil)

	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil)) // User preload
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil)) // UserGroup preload

	perms, err := svc.ListClusterPermissions(10)
	assert.NoError(t, err)
	assert.Len(t, perms, 2)
}

func TestPermissionService_ListClusterPermissions_Empty(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil))
	mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(nil))

	perms, err := svc.ListClusterPermissions(99)
	assert.NoError(t, err)
	assert.Empty(t, perms)
}

// ─── UpdateClusterPermission ────────────────────────────────────────────────

func TestPermissionService_UpdateClusterPermission_InvalidType(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	now := time.Now()
	permRows := sqlmock.NewRows([]string{
		"id", "cluster_id", "user_id", "user_group_id", "permission_type",
		"namespaces", "created_at", "updated_at", "deleted_at",
	}).AddRow(1, 1, 1, nil, permModels.PermissionTypeAdmin, `["*"]`, now, now, nil)
	mock.ExpectQuery(`SELECT`).WillReturnRows(permRows)

	req := &UpdateClusterPermissionRequest{
		PermissionType: "godmode", // invalid
	}
	perm, err := svc.UpdateClusterPermission(1, req)
	assert.Error(t, err)
	assert.Nil(t, perm)
	assert.Contains(t, err.Error(), "無效")
}

func TestPermissionService_UpdateClusterPermission_NotFound(t *testing.T) {
	svc, mock, cleanup := newPermSvcDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	req := &UpdateClusterPermissionRequest{PermissionType: permModels.PermissionTypeOps}
	perm, err := svc.UpdateClusterPermission(999, req)
	assert.Error(t, err)
	assert.Nil(t, perm)
}

// ─── rbac_service pure helper functions ────────────────────────────────────

func TestGetUserServiceAccountName(t *testing.T) {
	name := GetUserServiceAccountName(42)
	assert.Equal(t, "synapse-user-42-sa", name)
}

func TestGetUserRoleBindingName(t *testing.T) {
	name := GetUserRoleBindingName(7, "admin")
	assert.Equal(t, "synapse-user-7-admin", name)
}

func TestGetUserClusterRoleBindingName(t *testing.T) {
	name := GetUserClusterRoleBindingName(3, "ops")
	assert.Equal(t, "synapse-user-3-ops-cluster", name)
}
