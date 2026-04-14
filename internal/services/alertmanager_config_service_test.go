package services

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/models"
)

// AlertManagerConfigServiceTestSuite Alertmanager 配置服務測試套件
type AlertManagerConfigServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *AlertManagerConfigService
}

func (s *AlertManagerConfigServiceTestSuite) SetupTest() {
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
	s.service = NewAlertManagerConfigService(gormDB)
}

func (s *AlertManagerConfigServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// clusterRowsWithAMConfig builds a one-row sqlmock result for clusters
// with the alert_manager_config field filled in.
func clusterRowsWithAMConfig(amConfigJSON string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "name", "api_server", "kube_config_enc", "ca_enc", "sa_token_enc",
		"version", "status", "labels", "cert_expire_at", "last_heartbeat",
		"created_by", "created_at", "updated_at", "deleted_at",
		"monitoring_config", "alert_manager_config",
	}).AddRow(
		1, "test-cluster", "https://k8s.example.com:6443", "", "", "",
		"v1.28.0", "connected", "{}", nil, nil,
		0, now, now, nil,
		"{}", amConfigJSON,
	)
}

// TestGetAlertManagerConfig_WithConfig 測試獲取有效配置
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfig_WithConfig() {
	amJSON := `{"enabled":true,"endpoint":"http://alertmanager:9093"}`

	s.mock.ExpectQuery(`SELECT`).WillReturnRows(clusterRowsWithAMConfig(amJSON))

	cfg, err := s.service.GetAlertManagerConfig(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.True(s.T(), cfg.Enabled)
	assert.Equal(s.T(), "http://alertmanager:9093", cfg.Endpoint)
}

// TestGetAlertManagerConfig_EmptyConfig 測試叢集無配置時返回預設值
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfig_EmptyConfig() {
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(clusterRowsWithAMConfig(""))

	cfg, err := s.service.GetAlertManagerConfig(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.False(s.T(), cfg.Enabled)
}

// TestGetAlertManagerConfig_NullJSON 測試 "null" JSON 時返回預設值（空字串等效）
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfig_NullJSON() {
	// "null" is not valid JSON for AlertManagerConfig → unmarshal will fail → defaults
	// Actually "null" parses as nil pointer and will return empty config
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(clusterRowsWithAMConfig("null"))

	cfg, err := s.service.GetAlertManagerConfig(1)
	// null JSON → json.Unmarshal into struct yields zero value (enabled=false)
	// OR the "" check in service will catch it
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
}

// TestGetAlertManagerConfig_NotFound 測試叢集不存在
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfig_NotFound() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	cfg, err := s.service.GetAlertManagerConfig(999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cfg)
	assert.Contains(s.T(), err.Error(), "叢集不存在")
}

// TestGetAlertManagerConfig_DBError 測試 DB 錯誤
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfig_DBError() {
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	cfg, err := s.service.GetAlertManagerConfig(1)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cfg)
	assert.Contains(s.T(), err.Error(), "獲取叢集失敗")
}

// TestGetAlertManagerConfig_InvalidJSON 測試無效 JSON 返回預設停用配置
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfig_InvalidJSON() {
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(clusterRowsWithAMConfig("{invalid json}"))

	cfg, err := s.service.GetAlertManagerConfig(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cfg)
	assert.False(s.T(), cfg.Enabled) // graceful degradation
}

// TestUpdateAlertManagerConfig_Success 測試更新配置成功
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.NoError(s.T(), err)
}

// TestUpdateAlertManagerConfig_ClusterNotFound 測試叢集不存在時更新失敗
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_ClusterNotFound() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnResult(sqlmock.NewResult(0, 0)) // RowsAffected = 0
	s.mock.ExpectCommit()

	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
	}

	err := s.service.UpdateAlertManagerConfig(999, cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "叢集不存在")
}

// TestUpdateAlertManagerConfig_DBError 測試 DB 錯誤
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "更新 Alertmanager 配置失敗")
}

// TestUpdateAlertManagerConfig_ValidationFail_NoEndpoint 啟用時缺少 endpoint 應失敗
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_ValidationFail_NoEndpoint() {
	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "", // missing
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "配置驗證失敗")
}

// TestUpdateAlertManagerConfig_ValidationFail_BasicAuthNoPassword basic auth 缺密碼應失敗
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_ValidationFail_BasicAuthNoPassword() {
	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
		Auth: &models.MonitoringAuth{
			Type:     "basic",
			Username: "admin",
			Password: "", // missing
		},
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "配置驗證失敗")
}

// TestUpdateAlertManagerConfig_ValidationFail_BearerNoToken bearer auth 缺 token 應失敗
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_ValidationFail_BearerNoToken() {
	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
		Auth: &models.MonitoringAuth{
			Type:  "bearer",
			Token: "", // missing
		},
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "配置驗證失敗")
}

// TestUpdateAlertManagerConfig_ValidationFail_UnknownAuthType 不支援的認證型別應失敗
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_ValidationFail_UnknownAuthType() {
	cfg := &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
		Auth: &models.MonitoringAuth{
			Type: "oauth2", // unsupported
		},
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "配置驗證失敗")
}

// TestUpdateAlertManagerConfig_Disabled 停用時不需要 endpoint（驗證通過）
func (s *AlertManagerConfigServiceTestSuite) TestUpdateAlertManagerConfig_Disabled() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	cfg := &models.AlertManagerConfig{
		Enabled:  false,
		Endpoint: "", // no endpoint needed when disabled
	}

	err := s.service.UpdateAlertManagerConfig(1, cfg)
	assert.NoError(s.T(), err)
}

// TestDeleteAlertManagerConfig_Success 測試刪除配置成功
func (s *AlertManagerConfigServiceTestSuite) TestDeleteAlertManagerConfig_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteAlertManagerConfig(1)
	assert.NoError(s.T(), err)
}

// TestDeleteAlertManagerConfig_NotFound 測試叢集不存在
func (s *AlertManagerConfigServiceTestSuite) TestDeleteAlertManagerConfig_NotFound() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	err := s.service.DeleteAlertManagerConfig(999)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "叢集不存在")
}

// TestDeleteAlertManagerConfig_DBError 測試 DB 錯誤
func (s *AlertManagerConfigServiceTestSuite) TestDeleteAlertManagerConfig_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .clusters.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.DeleteAlertManagerConfig(1)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "刪除 Alertmanager 配置失敗")
}

// TestGetDefaultConfig 測試返回預設停用配置
func (s *AlertManagerConfigServiceTestSuite) TestGetDefaultConfig() {
	cfg := s.service.GetDefaultConfig()
	assert.NotNil(s.T(), cfg)
	assert.False(s.T(), cfg.Enabled)
}

// TestGetAlertManagerConfigTemplate 測試配置模板包含預設值
func (s *AlertManagerConfigServiceTestSuite) TestGetAlertManagerConfigTemplate() {
	tpl := s.service.GetAlertManagerConfigTemplate()
	assert.NotNil(s.T(), tpl)
	assert.True(s.T(), tpl.Enabled)
	assert.NotEmpty(s.T(), tpl.Endpoint)
}

// TestAlertManagerConfigServiceSuite 執行測試套件
func TestAlertManagerConfigServiceSuite(t *testing.T) {
	suite.Run(t, new(AlertManagerConfigServiceTestSuite))
}
