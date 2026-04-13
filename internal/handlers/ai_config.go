package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// AIConfigHandler AI 配置處理器
type AIConfigHandler struct {
	configService *services.AIConfigService
}

// NewAIConfigHandler 建立 AI 配置處理器
func NewAIConfigHandler(svc *services.AIConfigService) *AIConfigHandler {
	return &AIConfigHandler{configService: svc}
}

// GetConfig 獲取 AI 配置
func (h *AIConfigHandler) GetConfig(c *gin.Context) {
	config, err := h.configService.GetConfig()
	if err != nil {
		logger.Error("獲取 AI 配置失敗", "error", err)
		response.InternalError(c, "獲取 AI 配置失敗: "+err.Error())
		return
	}

	if config == nil {
		defaultCfg := models.GetDefaultAIConfig()
		response.OK(c, gin.H{
			"provider":    defaultCfg.Provider,
			"endpoint":    defaultCfg.Endpoint,
			"api_key":     "",
			"model":       defaultCfg.Model,
			"api_version": "",
			"enabled":     defaultCfg.Enabled,
		})
		return
	}

	apiKeyDisplay := ""
	if config.ID > 0 {
		// 檢查是否有 API Key 已配置（需要查帶 key 的記錄）
		fullConfig, _ := h.configService.GetConfigWithAPIKey()
		if fullConfig != nil && fullConfig.APIKey != "" {
			apiKeyDisplay = "******"
		}
	}

	response.OK(c, gin.H{
		"provider":    config.Provider,
		"endpoint":    config.Endpoint,
		"api_key":     apiKeyDisplay,
		"model":       config.Model,
		"api_version": config.APIVersion,
		"enabled":     config.Enabled,
	})
}

// UpdateConfig 更新 AI 配置
func (h *AIConfigHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		Model      string `json:"model"`
		APIVersion string `json:"api_version"`
		Enabled    bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	config := &models.AIConfig{
		Provider:   req.Provider,
		Endpoint:   req.Endpoint,
		APIKey:     req.APIKey,
		Model:      req.Model,
		APIVersion: req.APIVersion,
		Enabled:    req.Enabled,
	}

	if err := h.configService.SaveConfig(config); err != nil {
		logger.Error("儲存 AI 配置失敗", "error", err)
		response.InternalError(c, "儲存 AI 配置失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "儲存成功"})
}

// TestConnection 測試 AI 連線
func (h *AIConfigHandler) TestConnection(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// ollama 本地部署不需要 API Key，其他 provider 才強制要求
	apiKeyOptional := req.Provider == "ollama"

	apiKey := req.APIKey
	if apiKey == "" || apiKey == "******" {
		fullConfig, err := h.configService.GetConfigWithAPIKey()
		if err != nil || fullConfig == nil || fullConfig.APIKey == "" {
			if !apiKeyOptional {
				response.BadRequest(c, "請提供 API Key")
				return
			}
			// ollama 允許空 key，繼續測試
		} else {
			apiKey = fullConfig.APIKey
		}
	}

	endpoint := req.Endpoint
	if endpoint == "" {
		if req.Provider == "ollama" {
			endpoint = "http://localhost:11434"
		} else {
			endpoint = "https://api.openai.com/v1"
		}
	}

	model := req.Model
	if model == "" {
		if req.Provider == "ollama" {
			model = "llama3"
		} else {
			model = "gpt-4o"
		}
	}

	testConfig := &models.AIConfig{
		Provider: req.Provider,
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	}

	provider := services.NewAIProvider(testConfig)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := provider.TestConnection(ctx); err != nil {
		logger.Error("AI 連線測試失敗", "error", err)
		response.OK(c, gin.H{
			"message": "連線測試失敗: " + err.Error(),
			"success": false,
		})
		return
	}

	response.OK(c, gin.H{
		"message": "連線測試成功",
		"success": true,
	})
}
