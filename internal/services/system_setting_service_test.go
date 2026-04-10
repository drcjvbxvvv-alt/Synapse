package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
)

// ─── system_setting_helper tests ────────────────────────────────────────────

type SystemSettingHelperTestSuite struct {
	suite.Suite
	db   *gorm.DB
	mock sqlmock.Sqlmock
}

func (s *SystemSettingHelperTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	s.Require().NoError(err)
	s.db = gormDB
	s.mock = mock
}

func (s *SystemSettingHelperTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

func (s *SystemSettingHelperTestSuite) TestGetSystemSetting_Found() {
	type cfgStruct struct{ Foo string }
	payload := `{"Foo":"bar"}`
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "config_key", "value", "type", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "test_key", payload, "test", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	var out cfgStruct
	found, err := GetSystemSetting(s.db, "test_key", &out)
	assert.NoError(s.T(), err)
	assert.True(s.T(), found)
	assert.Equal(s.T(), "bar", out.Foo)
}

func (s *SystemSettingHelperTestSuite) TestGetSystemSetting_NotFound() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	var out map[string]interface{}
	found, err := GetSystemSetting(s.db, "missing_key", &out)
	assert.NoError(s.T(), err)
	assert.False(s.T(), found)
}

func (s *SystemSettingHelperTestSuite) TestGetSystemSetting_DBError() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	var out map[string]interface{}
	found, err := GetSystemSetting(s.db, "bad_key", &out)
	assert.Error(s.T(), err)
	assert.False(s.T(), found)
}

func (s *SystemSettingHelperTestSuite) TestGetSystemSetting_InvalidJSON() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "config_key", "value", "type", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "test_key", "{invalid}", "test", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	var out map[string]interface{}
	found, err := GetSystemSetting(s.db, "test_key", &out)
	assert.Error(s.T(), err)
	assert.False(s.T(), found)
	assert.Contains(s.T(), err.Error(), "解析配置")
}

func (s *SystemSettingHelperTestSuite) TestSaveSystemSetting_Create() {
	type cfgStruct struct{ Foo string }

	// First call: First → not found → create
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .system_settings.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := SaveSystemSetting(s.db, "new_key", "test", &cfgStruct{Foo: "baz"})
	assert.NoError(s.T(), err)
}

func (s *SystemSettingHelperTestSuite) TestSaveSystemSetting_Update() {
	type cfgStruct struct{ Foo string }
	now := time.Now()

	// First call: First → found
	rows := sqlmock.NewRows([]string{"id", "config_key", "value", "type", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "existing_key", `{"Foo":"old"}`, "test", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	// Then Save → UPDATE
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .system_settings.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := SaveSystemSetting(s.db, "existing_key", "test", &cfgStruct{Foo: "new"})
	assert.NoError(s.T(), err)
}

func TestSystemSettingHelperSuite(t *testing.T) {
	suite.Run(t, new(SystemSettingHelperTestSuite))
}

// ─── SystemSecurityService tests ────────────────────────────────────────────

type SystemSecurityServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *SystemSecurityService
}

func (s *SystemSecurityServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	s.Require().NoError(err)
	s.db = gormDB
	s.mock = mock
	s.service = NewSystemSecurityService(gormDB)
}

func (s *SystemSecurityServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

var securitySettingCols = []string{"id", "config_key", "value", "type", "created_at", "updated_at", "deleted_at"}

func (s *SystemSecurityServiceTestSuite) TestGetSecurityConfig_Found() {
	now := time.Now()
	payload := `{"session_ttl_minutes":60,"login_fail_lock_threshold":3,"lock_duration_minutes":15,"password_min_length":10}`
	rows := sqlmock.NewRows(securitySettingCols).
		AddRow(1, "security_config", payload, "security", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	cfg, err := s.service.GetSecurityConfig(context.Background())
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.Equal(s.T(), 60, cfg.SessionTTLMinutes)
	assert.Equal(s.T(), 3, cfg.LoginFailLockThreshold)
}

func (s *SystemSecurityServiceTestSuite) TestGetSecurityConfig_NotFound_ReturnsDefaults() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	cfg, err := s.service.GetSecurityConfig(context.Background())
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	// Default values
	assert.Equal(s.T(), 480, cfg.SessionTTLMinutes)
	assert.Equal(s.T(), 5, cfg.LoginFailLockThreshold)
}

func (s *SystemSecurityServiceTestSuite) TestGetSecurityConfig_DBError() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	cfg, err := s.service.GetSecurityConfig(context.Background())
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cfg)
}

func (s *SystemSecurityServiceTestSuite) TestGetSecurityConfig_InvalidJSON_ReturnsDefaults() {
	now := time.Now()
	rows := sqlmock.NewRows(securitySettingCols).
		AddRow(1, "security_config", "{bad json}", "security", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	cfg, err := s.service.GetSecurityConfig(context.Background())
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	// Falls back to defaults when JSON parse fails
	assert.Equal(s.T(), 480, cfg.SessionTTLMinutes)
}

func (s *SystemSecurityServiceTestSuite) TestSaveSecurityConfig_Create() {
	// SELECT → not found → INSERT
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .system_settings.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.SystemSecurityConfig{
		SessionTTLMinutes:      60,
		LoginFailLockThreshold: 3,
		LockDurationMinutes:    10,
		PasswordMinLength:      8,
	}
	err := s.service.SaveSecurityConfig(context.Background(), cfg)
	assert.NoError(s.T(), err)
}

func (s *SystemSecurityServiceTestSuite) TestSaveSecurityConfig_Update() {
	now := time.Now()
	// SELECT → found (ID=1) → UPDATE via Save
	rows := sqlmock.NewRows(securitySettingCols).
		AddRow(1, "security_config", `{"session_ttl_minutes":480}`, "security", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .system_settings.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.SystemSecurityConfig{
		SessionTTLMinutes: 120,
	}
	err := s.service.SaveSecurityConfig(context.Background(), cfg)
	assert.NoError(s.T(), err)
}

func (s *SystemSecurityServiceTestSuite) TestListAPITokens_Success() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "user_id", "name", "token_hash", "scopes", "expires_at", "last_used_at", "created_at"}).
		AddRow(1, 5, "my-token", "abc123hash", `["read"]`, nil, nil, now).
		AddRow(2, 5, "ci-token", "def456hash", `["read","write"]`, nil, nil, now)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	tokens, err := s.service.ListAPITokens(context.Background(), 5)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), tokens, 2)
	assert.Equal(s.T(), "my-token", tokens[0].Name)
}

func (s *SystemSecurityServiceTestSuite) TestListAPITokens_Empty() {
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "token_hash", "scopes", "expires_at", "last_used_at", "created_at"}))

	tokens, err := s.service.ListAPITokens(context.Background(), 99)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), tokens, 0)
}

func (s *SystemSecurityServiceTestSuite) TestCreateAPIToken_Success() {
	token := &models.APIToken{
		UserID:    1,
		Name:      "test-token",
		TokenHash: "sha256hash",
	}
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .api_tokens.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.CreateAPIToken(context.Background(), token)
	assert.NoError(s.T(), err)
}

func (s *SystemSecurityServiceTestSuite) TestCreateAPIToken_DBError() {
	token := &models.APIToken{UserID: 1, Name: "bad", TokenHash: "h"}
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .api_tokens.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.CreateAPIToken(context.Background(), token)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "create api token")
}

func (s *SystemSecurityServiceTestSuite) TestDeleteAPIToken_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM .api_tokens.`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteAPIToken(context.Background(), 1, 5)
	assert.NoError(s.T(), err)
}

func (s *SystemSecurityServiceTestSuite) TestDeleteAPIToken_NotFound() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM .api_tokens.`).
		WillReturnResult(sqlmock.NewResult(0, 0)) // RowsAffected=0
	s.mock.ExpectCommit()

	err := s.service.DeleteAPIToken(context.Background(), 999, 5)
	assert.Error(s.T(), err)
}

func (s *SystemSecurityServiceTestSuite) TestDeleteAPIToken_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM .api_tokens.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.DeleteAPIToken(context.Background(), 1, 5)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "delete api token")
}

func TestSystemSecurityServiceSuite(t *testing.T) {
	suite.Run(t, new(SystemSecurityServiceTestSuite))
}

// ─── SSHSettingService tests ─────────────────────────────────────────────────

type SSHSettingServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *SSHSettingService
}

func (s *SSHSettingServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	s.Require().NoError(err)
	s.db = gormDB
	s.mock = mock
	s.service = NewSSHSettingService(gormDB)
}

func (s *SSHSettingServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

func (s *SSHSettingServiceTestSuite) TestGetSSHConfig_Found() {
	now := time.Now()
	payload := `{"enabled":true,"username":"deploy","port":22,"auth_type":"key","password":"","private_key":"-----BEGIN RSA PRIVATE KEY-----"}`
	rows := sqlmock.NewRows(securitySettingCols).
		AddRow(1, "ssh_config", payload, "ssh", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	cfg, err := s.service.GetSSHConfig()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.Equal(s.T(), "deploy", cfg.Username)
	assert.True(s.T(), cfg.Enabled)
}

func (s *SSHSettingServiceTestSuite) TestGetSSHConfig_NotFound_ReturnsDefaults() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	cfg, err := s.service.GetSSHConfig()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.Equal(s.T(), "root", cfg.Username)
	assert.Equal(s.T(), 22, cfg.Port)
}

func (s *SSHSettingServiceTestSuite) TestSaveSSHConfig_Create() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .system_settings.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.SSHConfig{
		Enabled:  true,
		Username: "admin",
		Port:     2222,
		AuthType: "password",
		Password: "secret",
	}
	err := s.service.SaveSSHConfig(cfg)
	assert.NoError(s.T(), err)
}

func (s *SSHSettingServiceTestSuite) TestSaveSSHConfig_Update() {
	now := time.Now()
	rows := sqlmock.NewRows(securitySettingCols).
		AddRow(1, "ssh_config", `{"enabled":false}`, "ssh", now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .system_settings.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.SSHConfig{Enabled: true, Username: "deploy", Port: 22, AuthType: "key"}
	err := s.service.SaveSSHConfig(cfg)
	assert.NoError(s.T(), err)
}

func TestSSHSettingServiceSuite(t *testing.T) {
	suite.Run(t, new(SSHSettingServiceTestSuite))
}
