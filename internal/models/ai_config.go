package models

import (
	"time"

	"gorm.io/gorm"
)

// AIConfig AI 服务配置模型
type AIConfig struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	Provider   string         `json:"provider" gorm:"not null;size:50;default:openai"` // openai / azure / anthropic / ollama
	Endpoint   string         `json:"endpoint" gorm:"size:255"`                        // API endpoint
	APIKey     string         `json:"-" gorm:"type:text"`                              // 加密存储，不对外暴露
	Model      string         `json:"model" gorm:"size:100"`                           // gpt-4o / claude-3-5-sonnet-20241022 / llama3 等
	APIVersion string         `json:"api_version" gorm:"size:50"`                      // Azure OpenAI api-version, e.g. 2024-05-01-preview
	Enabled    bool           `json:"enabled" gorm:"default:false"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
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

// ProviderDefaults 各 Provider 的预设端点与模型
var ProviderDefaults = map[string]struct {
	Endpoint string
	Model    string
}{
	"openai":    {Endpoint: "https://api.openai.com/v1", Model: "gpt-4o"},
	"azure":     {Endpoint: "https://{resource}.openai.azure.com", Model: "gpt-4o"},
	"anthropic": {Endpoint: "https://api.anthropic.com", Model: "claude-3-5-sonnet-20241022"},
	"ollama":    {Endpoint: "http://localhost:11434", Model: "llama3"},
}
