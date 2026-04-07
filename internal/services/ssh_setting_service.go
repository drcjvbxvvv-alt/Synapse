package services

import (
	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
)

// SSHSettingService SSH配置服務
type SSHSettingService struct {
	db *gorm.DB
}

// NewSSHSettingService 建立SSH配置服務
func NewSSHSettingService(db *gorm.DB) *SSHSettingService {
	return &SSHSettingService{db: db}
}

// GetSSHConfig 從資料庫獲取SSH配置
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

// SaveSSHConfig 儲存SSH配置到資料庫
func (s *SSHSettingService) SaveSSHConfig(config *models.SSHConfig) error {
	return SaveSystemSetting(s.db, "ssh_config", "ssh", config)
}
