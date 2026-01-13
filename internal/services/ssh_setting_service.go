package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"kubepolaris/internal/models"

	"gorm.io/gorm"
)

// SSHSettingService SSH配置服务
type SSHSettingService struct {
	db *gorm.DB
}

// NewSSHSettingService 创建SSH配置服务
func NewSSHSettingService(db *gorm.DB) *SSHSettingService {
	return &SSHSettingService{db: db}
}

// GetSSHConfig 从数据库获取SSH配置
func (s *SSHSettingService) GetSSHConfig() (*models.SSHConfig, error) {
	var setting models.SystemSetting
	if err := s.db.Where("config_key = ?", "ssh_config").First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 返回默认配置
			defaultConfig := models.GetDefaultSSHConfig()
			return &defaultConfig, nil
		}
		return nil, err
	}

	var config models.SSHConfig
	if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
		return nil, fmt.Errorf("解析SSH配置失败: %w", err)
	}

	return &config, nil
}

// SaveSSHConfig 保存SSH配置到数据库
func (s *SSHSettingService) SaveSSHConfig(config *models.SSHConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化SSH配置失败: %w", err)
	}

	var setting models.SystemSetting
	result := s.db.Where("config_key = ?", "ssh_config").First(&setting)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 创建新配置
		setting = models.SystemSetting{
			ConfigKey: "ssh_config",
			Value:     string(configJSON),
			Type:      "ssh",
		}
		return s.db.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	// 更新现有配置
	setting.Value = string(configJSON)
	return s.db.Save(&setting).Error
}
