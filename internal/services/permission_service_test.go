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
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
		AddRow(1, "test-group", "Test description", now, now)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_groups` WHERE `user_groups`.`id` = ?")).
		WithArgs(1).
		WillReturnRows(rows)

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
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
		AddRow(1, "group-1", "Group 1", now, now).
		AddRow(2, "group-2", "Group 2", now, now)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_groups`")).
		WillReturnRows(rows)

	groups, err := s.service.ListUserGroups()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), groups, 2)
}

// TestDeleteUserGroup_Success 测试删除用户组成功
func (s *PermissionServiceTestSuite) TestDeleteUserGroup_Success() {
	// 先查询用户组
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
		AddRow(1, "test-group", "Test description", now, now)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_groups` WHERE `user_groups`.`id` = ?")).
		WithArgs(1).
		WillReturnRows(rows)

	// 删除关联的用户组成员
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("DELETE")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// 删除用户组
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `user_groups`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteUserGroup(1)
	assert.NoError(s.T(), err)
}

// TestHasClusterAccess 测试检查集群访问权限
func (s *PermissionServiceTestSuite) TestHasClusterAccess() {
	now := time.Now()
	// 模拟查询用户
	userRows := sqlmock.NewRows([]string{
		"id", "username", "password", "email", "role", "status",
		"created_at", "updated_at", "last_login_at", "avatar", "display_name",
	}).AddRow(
		1, "admin", "hashedpassword", "admin@example.com", "admin", "active",
		now, now, now, "", "Admin User",
	)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ?")).
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
