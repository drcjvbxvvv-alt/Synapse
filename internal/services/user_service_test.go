package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// UserServiceTestSuite 使用者管理服務測試套件
type UserServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *UserService
}

func (s *UserServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	s.db = gormDB
	s.mock = mock
	// nil repository → service falls back to the legacy *gorm.DB path.
	s.service = NewUserService(gormDB, nil)
}

func (s *UserServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// ---- CreateUser ----

func (s *UserServiceTestSuite) TestCreateUser_Success() {
	// COUNT 只有 1 個參數（deleted_at IS NULL 是 literal，不是佔位符）
	s.mock.ExpectQuery(`SELECT count`).
		WithArgs("newuser").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO .users.`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	user, err := s.service.CreateUser(context.Background(), &CreateUserRequest{
		Username: "newuser",
		Password: "password123",
		Email:    "new@example.com",
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), user)
	assert.Equal(s.T(), "newuser", user.Username)
	assert.Equal(s.T(), "local", user.AuthType)
	assert.Equal(s.T(), "active", user.Status)
	assert.NotEqual(s.T(), "password123", user.PasswordHash)
}

func (s *UserServiceTestSuite) TestCreateUser_DuplicateUsername() {
	s.mock.ExpectQuery(`SELECT count`).
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	user, err := s.service.CreateUser(context.Background(), &CreateUserRequest{
		Username: "admin",
		Password: "password123",
	})
	assert.Error(s.T(), err)
	assert.Nil(s.T(), user)
	assert.Contains(s.T(), err.Error(), "使用者名稱已存在")
}

// ---- GetUser ----

func (s *UserServiceTestSuite) TestGetUser_Success() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "system_role", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(1, "admin", "hash", "salt", "admin@example.com", "Admin",
		"local", "active", "platform_admin", now, "", now, now, nil)

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	user, err := s.service.GetUser(context.Background(), 1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), user)
	assert.Equal(s.T(), "admin", user.Username)
}

func (s *UserServiceTestSuite) TestGetUser_NotFound() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	user, err := s.service.GetUser(context.Background(), 999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), user)
}

// ---- UpdateUserStatus ----

func (s *UserServiceTestSuite) TestUpdateUserStatus_Active() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "system_role", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(2, "testuser", "hash", "salt", "test@example.com", "Test",
		"local", "inactive", "user", now, "", now, now, nil)

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE`).WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.UpdateUserStatus(context.Background(), 2, "active")
	assert.NoError(s.T(), err)
}

func (s *UserServiceTestSuite) TestUpdateUserStatus_UserNotFound() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	err := s.service.UpdateUserStatus(context.Background(), 999, "active")
	assert.Error(s.T(), err)
}

// ---- DeleteUser ----

func (s *UserServiceTestSuite) TestDeleteUser_Success() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "system_role", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(2, "testuser", "hash", "salt", "test@example.com", "Test",
		"local", "active", "user", now, "", now, now, nil)

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// DeleteUser: UserGroupMember 無 DeletedAt → 硬刪除 DELETE FROM
	// ClusterPermission、User 有 DeletedAt → 軟刪除 UPDATE SET deleted_at
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM.*user_group_members`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE.*cluster_permissions`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE.*users`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteUser(context.Background(), 2)
	assert.NoError(s.T(), err)
}

func (s *UserServiceTestSuite) TestDeleteUser_CannotDeleteAdmin() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "system_role", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(1, "admin", "hash", "salt", "admin@example.com", "Admin",
		"local", "active", "platform_admin", now, "", now, now, nil)

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	err := s.service.DeleteUser(context.Background(), 1)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "平台管理員")
}

func TestUserServiceSuite(t *testing.T) {
	suite.Run(t, new(UserServiceTestSuite))
}
