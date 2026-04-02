package services

import (
	"github.com/clay-wangzhi/Synapse/internal/models"

	"gorm.io/gorm"
)

// GrafanaSettingService Grafana 配置服务（读写 system_settings 表）
type GrafanaSettingService struct {
	db *gorm.DB
}

// NewGrafanaSettingService 创建 Grafana 配置服务
func NewGrafanaSettingService(db *gorm.DB) *GrafanaSettingService {
	return &GrafanaSettingService{db: db}
}

// GetGrafanaConfig 从数据库获取 Grafana 配置
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

// SaveGrafanaConfig 保存 Grafana 配置到数据库
func (s *GrafanaSettingService) SaveGrafanaConfig(config *models.GrafanaSettingConfig) error {
	return SaveSystemSetting(s.db, "grafana_config", "grafana", config)
}
