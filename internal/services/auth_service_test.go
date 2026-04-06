package services

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// AuthServiceTestSuite 認證服務測試套件
type AuthServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *AuthService
}

func (s *AuthServiceTestSuite) SetupTest() {
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
	s.service = NewAuthService(gormDB, "test-secret", 24)
}

func (s *AuthServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// makePasswordHash 產生含 salt 的 bcrypt 雜湊
func makePasswordHash(password, salt string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.MinCost)
	return string(h)
}

// userRow 產生一筆 User mock 行
func userRow(id uint, username, password, salt, status string) *sqlmock.Rows {
	now := time.Now()
	hash := makePasswordHash(password, salt)
	return sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(id, username, hash, salt, username+"@example.com", username,
		"local", status, now, "", now, now, nil)
}

// ---- Login ----

func (s *AuthServiceTestSuite) TestLogin_Success() {
	// 1. authenticateLocal: query user
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WithArgs("admin", sqlmock.AnyArg()).
		WillReturnRows(userRow(1, "admin", "Synapse@2026", "synapse_salt", "active"))

	// 2. db.Save (update last login)
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	// 3. buildPermissions: GetUserAllClusterPermissions
	//    - query user group members
	s.mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_group_id"}))
	//    - query cluster permissions
	s.mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "user_id", "permission_type"}))
	//    - query all clusters
	s.mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	//    - query user (for default permission type)
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(userRow(1, "admin", "Synapse@2026", "synapse_salt", "active"))

	result, err := s.service.Login("admin", "Synapse@2026", "local", "127.0.0.1")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), result)
	assert.NotEmpty(s.T(), result.Token)
	assert.Equal(s.T(), "admin", result.User.Username)
}

func (s *AuthServiceTestSuite) TestLogin_WrongPassword() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WithArgs("admin", sqlmock.AnyArg()).
		WillReturnRows(userRow(1, "admin", "Synapse@2026", "synapse_salt", "active"))

	_, err := s.service.Login("admin", "wrongpassword", "local", "127.0.0.1")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "使用者名稱或密碼錯誤")
}

func (s *AuthServiceTestSuite) TestLogin_UserNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WithArgs("nobody", sqlmock.AnyArg()).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := s.service.Login("nobody", "password", "local", "127.0.0.1")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "使用者名稱或密碼錯誤")
}

func (s *AuthServiceTestSuite) TestLogin_DisabledUser() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WithArgs("disabled", sqlmock.AnyArg()).
		WillReturnRows(userRow(2, "disabled", "password123", "salt456", "inactive"))

	_, err := s.service.Login("disabled", "password123", "local", "127.0.0.1")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "使用者賬號已被禁用")
}

func (s *AuthServiceTestSuite) TestLogin_UnsupportedAuthType() {
	_, err := s.service.Login("admin", "password", "oauth2", "127.0.0.1")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "不支援的認證型別")
}

// ---- ChangePassword ----

func (s *AuthServiceTestSuite) TestChangePassword_Success() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WillReturnRows(userRow(1, "admin", "oldpassword", "mysalt", "active"))

	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.ChangePassword(1, "oldpassword", "newpassword")
	assert.NoError(s.T(), err)
}

func (s *AuthServiceTestSuite) TestChangePassword_WrongOldPassword() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WillReturnRows(userRow(1, "admin", "correctpassword", "mysalt", "active"))

	err := s.service.ChangePassword(1, "wrongoldpassword", "newpassword")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "原密碼錯誤")
}

func (s *AuthServiceTestSuite) TestChangePassword_LDAPUser() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name",
		"auth_type", "status", "last_login_at", "last_login_ip",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(3, "ldapuser", "", "", "ldap@example.com", "LDAP User",
		"ldap", "active", now, "", now, now, nil)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).WillReturnRows(rows)

	err := s.service.ChangePassword(3, "anypassword", "newpassword")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "LDAP使用者不能在此修改密碼")
}

func (s *AuthServiceTestSuite) TestChangePassword_UserNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WillReturnError(gorm.ErrRecordNotFound)

	err := s.service.ChangePassword(999, "old", "new")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "使用者不存在")
}

// ---- GetProfile ----

func (s *AuthServiceTestSuite) TestGetProfile_Success() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WillReturnRows(userRow(1, "admin", "pass", "salt", "active"))

	user, err := s.service.GetProfile(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), user)
	assert.Equal(s.T(), "admin", user.Username)
}

func (s *AuthServiceTestSuite) TestGetProfile_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT")).
		WillReturnError(gorm.ErrRecordNotFound)

	user, err := s.service.GetProfile(999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), user)
}

func TestAuthServiceSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}
