package services

import (
	"github.com/clay-wangzhi/Synapse/internal/models"

	"gorm.io/gorm"
)

// GrafanaSettingService Grafana 配置服務（讀寫 system_settings 表）
type GrafanaSettingService struct {
	db *gorm.DB
}

// NewGrafanaSettingService 建立 Grafana 配置服務
func NewGrafanaSettingService(db *gorm.DB) *GrafanaSettingService {
	return &GrafanaSettingService{db: db}
}

// GetGrafanaConfig 從資料庫獲取 Grafana 配置
func (s *GrafanaSettingService) GetGrafanaConfig() (*models.GrafanaSettingConfig, error) {
	var config models.GrafanaSettingConfig
	found, err := GetSystemSetting(s.db, "grafana_config", &config)
	if err != nil {
		return nil, err
	}
	if !found {
		defaultConfig := models.GetDefaultGrafanaSettingConfig()
		return &defaultConfig, nil
	}
	return &config, nil
}

// SaveGrafanaConfig 儲存 Grafana 配置到資料庫
func (s *GrafanaSettingService) SaveGrafanaConfig(config *models.GrafanaSettingConfig) error {
	return SaveSystemSetting(s.db, "grafana_config", "grafana", config)
}
