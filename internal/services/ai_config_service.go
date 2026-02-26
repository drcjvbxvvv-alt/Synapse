package services

import (
	"fmt"

	"github.com/clay-wangzhi/KubePolaris/internal/models"
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"
	"gorm.io/gorm"
)

// AIConfigService AI 配置服务
type AIConfigService struct {
	db *gorm.DB
}

// NewAIConfigService 创建 AI 配置服务
func NewAIConfigService(db *gorm.DB) *AIConfigService {
	return &AIConfigService{db: db}
}

// GetConfig 获取 AI 配置（只取第一条记录，系统级单例配置）
func (s *AIConfigService) GetConfig() (*models.AIConfig, error) {
	var config models.AIConfig
	err := s.db.First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("获取 AI 配置失败: %w", err)
	}
	return &config, nil
}

// GetConfigWithAPIKey 获取包含 API Key 的完整配置（内部使用）
func (s *AIConfigService) GetConfigWithAPIKey() (*models.AIConfig, error) {
	var config models.AIConfig
	// 明确选择所有字段包括 api_key
	err := s.db.Select("*").First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("获取 AI 配置失败: %w", err)
	}
	return &config, nil
}

// SaveConfig 保存 AI 配置（创建或更新）
func (s *AIConfigService) SaveConfig(config *models.AIConfig) error {
	var existing models.AIConfig
	err := s.db.Select("id, api_key").First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询 AI 配置失败: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		if err := s.db.Create(config).Error; err != nil {
			return fmt.Errorf("创建 AI 配置失败: %w", err)
		}
		logger.Info("AI 配置创建成功")
		return nil
	}

	// 更新已有记录
	config.ID = existing.ID
	// 如果传入的 APIKey 为占位符，保持原有 key 不变
	if config.APIKey == "******" {
		config.APIKey = existing.APIKey
	}

	if err := s.db.Model(&existing).Select("provider", "endpoint", "api_key", "model", "enabled").Updates(config).Error; err != nil {
		return fmt.Errorf("更新 AI 配置失败: %w", err)
	}

	logger.Info("AI 配置更新成功")
	return nil
}

// IsEnabled 检查 AI 功能是否已启用且配置完整
func (s *AIConfigService) IsEnabled() bool {
	config, err := s.GetConfigWithAPIKey()
	if err != nil || config == nil {
		return false
	}
	return config.Enabled && config.APIKey != "" && config.Endpoint != ""
}
