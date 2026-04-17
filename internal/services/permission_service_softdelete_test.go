package services

// Soft-delete audit tests (R-11)
//
// These tests verify that list queries on soft-deleted models always include
// `"deleted_at" IS NULL` in the generated SQL — ensuring soft-deleted records
// are never returned to callers. Each test uses a sqlmock expectation pattern
// that requires the soft-delete predicate to be present; if GORM stops
// generating it (e.g. after a GORM upgrade), the expectation will fail to
// match and the test will error.

import (
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func userGroupCols() []string {
	return []string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}
}

func userCols() []string {
	return []string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}
}

func clusterPermissionCols() []string {
	return []string{
		"id", "cluster_id", "user_id", "user_group_id", "permission_type",
		"namespaces", "custom_role_ref", "feature_policy",
		"created_at", "updated_at", "deleted_at",
	}
}

// ── ListUsers ────────────────────────────────────────────────────────────────

// TestListUsers_SoftDeleteFilter verifies that GORM adds `"deleted_at" IS NULL`
// when querying Users so soft-deleted users never leak into the response.
func (s *PermissionServiceTestSuite) TestListUsers_SoftDeleteFilter() {
	now := time.Now()
	// ExpectQuery pattern must contain "deleted_at IS NULL" — the test fails if
	// GORM ever stops generating the soft-delete predicate.
	s.mock.ExpectQuery(`"deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows(userCols()).
			AddRow(1, "alice", "hash", "salt", "alice@example.com", "Alice", "local", "active", now, "", now, now, nil).
			AddRow(2, "bob", "hash", "salt", "bob@example.com", "Bob", "local", "active", now, "", now, now, nil))

	users, err := s.service.ListUsers()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), users, 2)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

// TestListUsers_ExcludesSoftDeleted confirms that when the database returns
// only active users (because the query includes `"deleted_at" IS NULL`), the
// caller receives exactly those records — no extra filtering needed in Go.
func (s *PermissionServiceTestSuite) TestListUsers_ExcludesSoftDeleted() {
	now := time.Now()
	// DB returns one active user; the soft-deleted user is excluded at the
	// SQL level by GORM's scope, so it never appears in the result set.
	s.mock.ExpectQuery(`"deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows(userCols()).
			AddRow(1, "alice", "hash", "salt", "alice@example.com", "Alice", "local", "active", now, "", now, now, nil))

	users, err := s.service.ListUsers()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), users, 1)
	assert.Equal(s.T(), "alice", users[0].Username)
}

// ── ListUserGroups ────────────────────────────────────────────────────────────

// TestListUserGroups_SoftDeleteFilter verifies `"deleted_at" IS NULL` appears in
// the user_groups SELECT so soft-deleted groups are filtered out.
func (s *PermissionServiceTestSuite) TestListUserGroups_SoftDeleteFilter() {
	now := time.Now()
	// First query: list user_groups — must include soft-delete predicate.
	s.mock.ExpectQuery(`"deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows(userGroupCols()).
			AddRow(1, "dev-team", "Developers", now, now, nil))

	// Second query: GORM Preload("Users") inner SELECT (loose match is fine
	// here — the important assertion is on the outer query above).
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_group_id"}))

	groups, err := s.service.ListUserGroups()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), groups, 1)
	assert.Equal(s.T(), "dev-team", groups[0].Name)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

// ── ListClusterPermissions ────────────────────────────────────────────────────

// TestListClusterPermissions_SoftDeleteFilter verifies `"deleted_at" IS NULL`
// appears in the cluster_permissions SELECT.
func (s *PermissionServiceTestSuite) TestListClusterPermissions_SoftDeleteFilter() {
	now := time.Now()
	userID := uint(1)

	// Main SELECT must contain soft-delete predicate.
	s.mock.ExpectQuery(`"deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows(clusterPermissionCols()).
			AddRow(1, 10, userID, nil, "dev", `["*"]`, "", "", now, now, nil))

	// Preload("User") fires because user_id=1 (non-nil).
	// Preload("UserGroup") does NOT fire because user_group_id is nil for all rows.
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(userCols()))

	perms, err := s.service.ListClusterPermissions(10)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), perms, 1)
	assert.Equal(s.T(), "dev", perms[0].PermissionType)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

// TestListClusterPermissions_Empty verifies the query still applies the
// soft-delete scope even when no records match.
func (s *PermissionServiceTestSuite) TestListClusterPermissions_Empty() {
	s.mock.ExpectQuery(`"deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows(clusterPermissionCols()))

	// No Preload inner queries fired when main result is empty.
	perms, err := s.service.ListClusterPermissions(99)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), perms)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

// ── ListAllClusterPermissions ─────────────────────────────────────────────────

// TestListAllClusterPermissions_SoftDeleteFilter verifies the all-cluster
// variant also applies the soft-delete scope.
func (s *PermissionServiceTestSuite) TestListAllClusterPermissions_SoftDeleteFilter() {
	now := time.Now()
	userID := uint(2)

	s.mock.ExpectQuery(`"deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows(clusterPermissionCols()).
			AddRow(1, 5, userID, nil, "admin", `["*"]`, "", "", now, now, nil).
			AddRow(2, 7, userID, nil, "readonly", `["default"]`, "", "", now, now, nil))

	// Preload("User") fires: user_id=2 for both rows.
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows(userCols()))
	// Preload("UserGroup") does NOT fire: user_group_id is nil for all rows.
	// Preload("Cluster") fires: cluster_id IN (5, 7).
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "name", "status", "created_at", "updated_at", "deleted_at",
	}))

	perms, err := s.service.ListAllClusterPermissions()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), perms, 2)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}
