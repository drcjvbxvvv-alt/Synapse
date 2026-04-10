package services

import (
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

// AIConfigServiceTestSuite AI 配置服務測試套件
type AIConfigServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *AIConfigService
}

func (s *AIConfigServiceTestSuite) SetupTest() {
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
	s.service = NewAIConfigService(gormDB)
}

func (s *AIConfigServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

var aiConfigCols = []string{"id", "provider", "endpoint", "api_key", "model", "api_version", "enabled", "created_at", "updated_at", "deleted_at"}

// TestGetConfig_Found 測試獲取 AI 配置成功
func (s *AIConfigServiceTestSuite) TestGetConfig_Found() {
	now := time.Now()
	rows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "openai", "https://api.openai.com/v1", "sk-secret", "gpt-4o", "", true, now, now, nil)

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	cfg, err := s.service.GetConfig()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.Equal(s.T(), "openai", cfg.Provider)
	assert.True(s.T(), cfg.Enabled)
}

// TestGetConfig_NotFound 測試無配置時返回 nil（非錯誤）
func (s *AIConfigServiceTestSuite) TestGetConfig_NotFound() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	cfg, err := s.service.GetConfig()
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), cfg)
}

// TestGetConfig_DBError 測試 DB 錯誤
func (s *AIConfigServiceTestSuite) TestGetConfig_DBError() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	cfg, err := s.service.GetConfig()
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cfg)
	assert.Contains(s.T(), err.Error(), "獲取 AI 配置失敗")
}

// TestGetConfigWithAPIKey_Found 測試獲取含 API Key 的完整配置
func (s *AIConfigServiceTestSuite) TestGetConfigWithAPIKey_Found() {
	now := time.Now()
	rows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "anthropic", "https://api.anthropic.com", "sk-ant-key", "claude-3-5-sonnet-20241022", "", true, now, now, nil)

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	cfg, err := s.service.GetConfigWithAPIKey()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.Equal(s.T(), "anthropic", cfg.Provider)
}

// TestGetConfigWithAPIKey_NotFound 測試無配置時返回 nil
func (s *AIConfigServiceTestSuite) TestGetConfigWithAPIKey_NotFound() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	cfg, err := s.service.GetConfigWithAPIKey()
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), cfg)
}

// TestSaveConfig_Create 測試建立新 AI 配置（不存在時）
func (s *AIConfigServiceTestSuite) TestSaveConfig_Create() {
	// First: query to check existing → not found
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	// Then: INSERT
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .ai_configs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.AIConfig{
		Provider: "openai",
		Endpoint: "https://api.openai.com/v1",
		APIKey:   "sk-newkey",
		Model:    "gpt-4o",
		Enabled:  true,
	}

	err := s.service.SaveConfig(cfg)
	assert.NoError(s.T(), err)
}

// TestSaveConfig_Update 測試更新現有 AI 配置
func (s *AIConfigServiceTestSuite) TestSaveConfig_Update() {
	now := time.Now()
	// First: query existing record → found
	existRows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "openai", "https://api.openai.com/v1", "sk-existing", "gpt-4o", "", true, now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(existRows)

	// Then: UPDATE
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .ai_configs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.AIConfig{
		Provider: "openai",
		Endpoint: "https://api.openai.com/v1",
		APIKey:   "sk-newkey",
		Model:    "gpt-4o-mini",
		Enabled:  true,
	}

	err := s.service.SaveConfig(cfg)
	assert.NoError(s.T(), err)
}

// TestSaveConfig_UpdateKeepsExistingKey 佔位符 "******" 應保留原有 API key
func (s *AIConfigServiceTestSuite) TestSaveConfig_UpdateKeepsExistingKey() {
	now := time.Now()
	existRows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "openai", "https://api.openai.com/v1", "sk-original", "gpt-4o", "", true, now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(existRows)

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .ai_configs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.AIConfig{
		Provider: "openai",
		Endpoint: "https://api.openai.com/v1",
		APIKey:   "******", // placeholder — service should keep original key
		Model:    "gpt-4o",
		Enabled:  true,
	}

	err := s.service.SaveConfig(cfg)
	assert.NoError(s.T(), err)
}

// TestSaveConfig_QueryError 測試查詢現有配置失敗
func (s *AIConfigServiceTestSuite) TestSaveConfig_QueryError() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	cfg := &models.AIConfig{Provider: "openai", Endpoint: "http://x", APIKey: "k", Enabled: true}
	err := s.service.SaveConfig(cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "查詢 AI 配置失敗")
}

// TestIsEnabled_True 測試啟用且配置完整時返回 true
func (s *AIConfigServiceTestSuite) TestIsEnabled_True() {
	now := time.Now()
	rows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "openai", "https://api.openai.com/v1", "sk-key", "gpt-4o", "", true, now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	assert.True(s.T(), s.service.IsEnabled())
}

// TestIsEnabled_FalseWhenDisabled 測試 enabled=false 時返回 false
func (s *AIConfigServiceTestSuite) TestIsEnabled_FalseWhenDisabled() {
	now := time.Now()
	rows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "openai", "https://api.openai.com/v1", "sk-key", "gpt-4o", "", false, now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	assert.False(s.T(), s.service.IsEnabled())
}

// TestIsEnabled_FalseWhenNoConfig 測試無配置時返回 false
func (s *AIConfigServiceTestSuite) TestIsEnabled_FalseWhenNoConfig() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	assert.False(s.T(), s.service.IsEnabled())
}

// TestIsEnabled_FalseWhenEmptyEndpoint 測試 endpoint 為空時返回 false
func (s *AIConfigServiceTestSuite) TestIsEnabled_FalseWhenEmptyEndpoint() {
	now := time.Now()
	rows := sqlmock.NewRows(aiConfigCols).
		AddRow(1, "openai", "", "sk-key", "gpt-4o", "", true, now, now, nil)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	assert.False(s.T(), s.service.IsEnabled())
}

// TestAIConfigServiceSuite 執行測試套件
func TestAIConfigServiceSuite(t *testing.T) {
	suite.Run(t, new(AIConfigServiceTestSuite))
}
