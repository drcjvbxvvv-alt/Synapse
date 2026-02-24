package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/clay-wangzhi/KubePolaris/internal/models"

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
	var setting models.SystemSetting
	if err := s.db.Where("config_key = ?", "grafana_config").First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			defaultConfig := models.GetDefaultGrafanaSettingConfig()
			return &defaultConfig, nil
		}
		return nil, err
	}

	var config models.GrafanaSettingConfig
	if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
		return nil, fmt.Errorf("解析 Grafana 配置失败: %w", err)
	}

	return &config, nil
}

// SaveGrafanaConfig 保存 Grafana 配置到数据库
func (s *GrafanaSettingService) SaveGrafanaConfig(config *models.GrafanaSettingConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化 Grafana 配置失败: %w", err)
	}

	var setting models.SystemSetting
	result := s.db.Where("config_key = ?", "grafana_config").First(&setting)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		setting = models.SystemSetting{
			ConfigKey: "grafana_config",
			Value:     string(configJSON),
			Type:      "grafana",
		}
		return s.db.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	setting.Value = string(configJSON)
	return s.db.Save(&setting).Error
}
