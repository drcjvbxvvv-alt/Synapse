package services

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PermissionServiceTestSuite 定义权限服务测试套件
type PermissionServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *PermissionService
}

// SetupTest 每个测试前的设置
func (s *PermissionServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	s.db = gormDB
	s.mock = mock
	s.service = NewPermissionService(gormDB)
}

// TearDownTest 每个测试后的清理
func (s *PermissionServiceTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// TestCreateUserGroup 测试创建用户组
func (s *PermissionServiceTestSuite) TestCreateUserGroup() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_groups`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	group, err := s.service.CreateUserGroup("test-group", "Test group description")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), group)
	assert.Equal(s.T(), "test-group", group.Name)
}

// TestGetUserGroup_Success 测试获取用户组成功
func (s *PermissionServiceTestSuite) TestGetUserGroup_Success() {
	now := time.Now()
	// 主查询：获取用户组
	groupRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "test-group", "Test description", now, now, nil)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_groups` WHERE `user_groups`.`id` = ? AND `user_groups`.`deleted_at` IS NULL ORDER BY `user_groups`.`id` LIMIT ?")).
		WithArgs(1, 1).
		WillReturnRows(groupRows)

	// Preload Users 查询 - GORM 会通过 user_group_members 关联查询
	memberRows := sqlmock.NewRows([]string{"user_id", "user_group_id"}).
		AddRow(1, 1)

	s.mock.ExpectQuery(`SELECT.*FROM.*user_group_members.*WHERE.*user_group_id`).
		WithArgs(1).
		WillReturnRows(memberRows)

	// 查询关联的用户
	userRows := sqlmock.NewRows([]string{"id", "username", "password_hash", "email", "role", "status",
		"created_at", "updated_at", "last_login_at", "avatar", "display_name", "auth_type", "salt", "deleted_at"}).
		AddRow(1, "user1", "hash1", "user1@example.com", "user", "active", now, now, now, "", "User 1", "local", "salt", nil)

	s.mock.ExpectQuery(`SELECT.*FROM.*users.*WHERE.*id IN`).
		WithArgs(1).
		WillReturnRows(userRows)

	group, err := s.service.GetUserGroup(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), group)
	assert.Equal(s.T(), "test-group", group.Name)
}

// TestGetUserGroup_NotFound 测试获取不存在的用户组
func (s *PermissionServiceTestSuite) TestGetUserGroup_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_groups` WHERE `user_groups`.`id` = ?")).
		WithArgs(999).
		WillReturnError(gorm.ErrRecordNotFound)

	group, err := s.service.GetUserGroup(999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), group)
}

// TestListUserGroups 测试列出所有用户组
func (s *PermissionServiceTestSuite) TestListUserGroups() {
	now := time.Now()
	// 主查询：获取所有用户组
	groupRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
		AddRow(1, "group-1", "Group 1", now, now).
		AddRow(2, "group-2", "Group 2", now, now)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_groups`")).
		WillReturnRows(groupRows)

	// Preload Users 查询 - 查询用户组成员关联
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_group_members` WHERE `user_group_members`.`user_group_id` IN (?,?)")).
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_group_id"}))

	groups, err := s.service.ListUserGroups()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), groups, 2)
}

// TestDeleteUserGroup_Success 测试删除用户组成功
func (s *PermissionServiceTestSuite) TestDeleteUserGroup_Success() {
	// 检查关联的权限配置 - Count 查询
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM.*cluster_permissions.*WHERE.*user_group_id`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 删除关联的用户组成员
	s.mock.ExpectExec(`DELETE FROM.*user_group_members.*WHERE.*user_group_id`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 删除用户组
	s.mock.ExpectExec(`DELETE FROM.*user_groups.*WHERE.*id`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.service.DeleteUserGroup(1)
	assert.NoError(s.T(), err)
}

// TestHasClusterAccess 测试检查集群访问权限
func (s *PermissionServiceTestSuite) TestHasClusterAccess() {
	now := time.Now()
	// 1. 先查找用户直接权限（不存在）
	s.mock.ExpectQuery(`SELECT.*FROM.*cluster_permissions.*WHERE.*cluster_id.*AND.*user_id`).
		WithArgs(1, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	// 2. 查找用户组权限（不存在）
	s.mock.ExpectQuery(`SELECT.*FROM.*user_group_members.*WHERE.*user_id`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_group_id"}))

	// 3. 查询用户信息以获取默认权限
	userRows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "email", "role", "status",
		"created_at", "updated_at", "last_login_at", "avatar", "display_name", "auth_type", "salt", "deleted_at",
	}).AddRow(
		1, "admin", "hashedpassword", "admin@example.com", "admin", "active",
		now, now, now, "", "Admin User", "local", "salt123", nil,
	)

	s.mock.ExpectQuery(`SELECT.*FROM.*users.*WHERE.*id.*AND.*deleted_at`).
		WithArgs(1).
		WillReturnRows(userRows)

	// 管理员应该有所有集群的访问权限
	hasAccess := s.service.HasClusterAccess(1, 1)
	assert.True(s.T(), hasAccess)
}

// TestListUsers 测试列出用户
func (s *PermissionServiceTestSuite) TestListUsers() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password", "email", "role", "status",
		"created_at", "updated_at", "last_login_at", "avatar", "display_name",
	}).
		AddRow(1, "user1", "hash1", "user1@example.com", "user", "active", now, now, now, "", "User 1").
		AddRow(2, "user2", "hash2", "user2@example.com", "user", "active", now, now, now, "", "User 2")

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users`")).
		WillReturnRows(rows)

	users, err := s.service.ListUsers()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), users, 2)
}

// TestPermissionServiceSuite 运行测试套件
func TestPermissionServiceSuite(t *testing.T) {
	suite.Run(t, new(PermissionServiceTestSuite))
}
