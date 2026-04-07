package services

import (
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// AIConfigService AI 配置服務
type AIConfigService struct {
	db *gorm.DB
}

// NewAIConfigService 建立 AI 配置服務
func NewAIConfigService(db *gorm.DB) *AIConfigService {
	return &AIConfigService{db: db}
}

// GetConfig 獲取 AI 配置（只取第一條記錄，系統級單例配置）
func (s *AIConfigService) GetConfig() (*models.AIConfig, error) {
	var config models.AIConfig
	err := s.db.First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("獲取 AI 配置失敗: %w", err)
	}
	return &config, nil
}

// GetConfigWithAPIKey 獲取包含 API Key 的完整配置（內部使用）
func (s *AIConfigService) GetConfigWithAPIKey() (*models.AIConfig, error) {
	var config models.AIConfig
	// 明確選擇所有欄位包括 api_key
	err := s.db.Select("*").First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("獲取 AI 配置失敗: %w", err)
	}
	return &config, nil
}

// SaveConfig 儲存 AI 配置（建立或更新）
func (s *AIConfigService) SaveConfig(config *models.AIConfig) error {
	var existing models.AIConfig
	err := s.db.Select("id, api_key").First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查詢 AI 配置失敗: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		if err := s.db.Create(config).Error; err != nil {
			return fmt.Errorf("建立 AI 配置失敗: %w", err)
		}
		logger.Info("AI 配置建立成功")
		return nil
	}

	// 更新已有記錄
	config.ID = existing.ID
	// 如果傳入的 APIKey 為佔位符，保持原有 key 不變
	if config.APIKey == "******" {
		config.APIKey = existing.APIKey
	}

	if err := s.db.Model(&existing).Select("provider", "endpoint", "api_key", "model", "enabled").Updates(config).Error; err != nil {
		return fmt.Errorf("更新 AI 配置失敗: %w", err)
	}

	logger.Info("AI 配置更新成功")
	return nil
}

// IsEnabled 檢查 AI 功能是否已啟用且配置完整
func (s *AIConfigService) IsEnabled() bool {
	config, err := s.GetConfigWithAPIKey()
	if err != nil || config == nil {
		return false
	}
	return config.Enabled && config.APIKey != "" && config.Endpoint != ""
}
