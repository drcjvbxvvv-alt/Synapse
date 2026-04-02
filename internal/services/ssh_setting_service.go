package services

import (
	"github.com/clay-wangzhi/Synapse/internal/models"

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
	var config models.SSHConfig
	found, err := GetSystemSetting(s.db, "ssh_config", &config)
	if err != nil {
		return nil, err
	}
	if !found {
		defaultConfig := models.GetDefaultSSHConfig()
		return &defaultConfig, nil
	}
	return &config, nil
}

// SaveSSHConfig 保存SSH配置到数据库
func (s *SSHSettingService) SaveSSHConfig(config *models.SSHConfig) error {
	return SaveSystemSetting(s.db, "ssh_config", "ssh", config)
}
