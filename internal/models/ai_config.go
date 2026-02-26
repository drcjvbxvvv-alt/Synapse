package models

import (
	"time"

	"gorm.io/gorm"
)

// AIConfig AI 服务配置模型
type AIConfig struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Provider  string         `json:"provider" gorm:"not null;size:50;default:openai"` // openai（兼容 DeepSeek/通义千问等）
	Endpoint  string         `json:"endpoint" gorm:"size:255"`                        // API endpoint
	APIKey    string         `json:"-" gorm:"type:text"`                              // 加密存储，不对外暴露
	Model     string         `json:"model" gorm:"size:100"`                           // gpt-4o / deepseek-chat / qwen-turbo 等
	Enabled   bool           `json:"enabled" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (AIConfig) TableName() string {
	return "ai_configs"
}

// GetDefaultAIConfig 获取默认 AI 配置
func GetDefaultAIConfig() AIConfig {
	return AIConfig{
		Provider: "openai",
		Endpoint: "https://api.openai.com/v1",
		Model:    "gpt-4o",
		Enabled:  false,
	}
}
