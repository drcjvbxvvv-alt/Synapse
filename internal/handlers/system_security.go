package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const securityConfigKey = "security_config"

// SystemSecurityHandler 安全設定處理器（登入安全設定 + API Token 管理）
type SystemSecurityHandler struct {
	db *gorm.DB
}

// NewSystemSecurityHandler 建立安全設定處理器
func NewSystemSecurityHandler(db *gorm.DB) *SystemSecurityHandler {
	return &SystemSecurityHandler{db: db}
}

// ─── 登入安全設定 ─────────────────────────────────────────────────────────────

// GetSecurityConfig GET /system/security/config（PlatformAdmin）
func (h *SystemSecurityHandler) GetSecurityConfig(c *gin.Context) {
	cfg, err := h.loadSecurityConfig()
	if err != nil {
		logger.Error("取得安全設定失敗: %v", err)
		response.InternalError(c, "取得安全設定失敗")
		return
	}
	response.OK(c, cfg)
}

// UpdateSecurityConfig PUT /system/security/config（PlatformAdmin）
func (h *SystemSecurityHandler) UpdateSecurityConfig(c *gin.Context) {
	var req models.SystemSecurityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	// 合理值範圍保護
	if req.SessionTTLMinutes <= 0 {
		req.SessionTTLMinutes = 480
	}
	if req.LoginFailLockThreshold <= 0 {
		req.LoginFailLockThreshold = 5
	}
	if req.LockDurationMinutes <= 0 {
		req.LockDurationMinutes = 30
	}
	if req.PasswordMinLength < 6 {
		req.PasswordMinLength = 6
	}

	b, _ := json.Marshal(req)

	var setting models.SystemSetting
	h.db.Where("config_key = ?", securityConfigKey).First(&setting)
	setting.ConfigKey = securityConfigKey
	setting.Type = "security"
	setting.Value = string(b)
	if setting.ID == 0 {
		if err := h.db.Create(&setting).Error; err != nil {
			logger.Error("建立安全設定失敗: %v", err)
			response.InternalError(c, "儲存安全設定失敗")
			return
		}
	} else {
		if err := h.db.Save(&setting).Error; err != nil {
			logger.Error("更新安全設定失敗: %v", err)
			response.InternalError(c, "儲存安全設定失敗")
			return
		}
	}

	logger.Info("安全設定更新成功")
	response.OK(c, gin.H{"message": "安全設定儲存成功"})
}

// loadSecurityConfig 從 DB 讀取安全設定，找不到時回傳預設值
func (h *SystemSecurityHandler) loadSecurityConfig() (*models.SystemSecurityConfig, error) {
	var setting models.SystemSetting
	err := h.db.Where("config_key = ?", securityConfigKey).First(&setting).Error
	if err != nil {
		// 找不到記錄時回傳預設值（非錯誤）
		if err == gorm.ErrRecordNotFound {
			cfg := models.GetDefaultSystemSecurityConfig()
			return &cfg, nil
		}
		return nil, err
	}
	var cfg models.SystemSecurityConfig
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		def := models.GetDefaultSystemSecurityConfig()
		return &def, nil
	}
	return &cfg, nil
}

// ─── API Token 管理 ───────────────────────────────────────────────────────────

// apiTokenResponse Token 列表回傳結構（不含 TokenHash）
type apiTokenResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ListAPITokens GET /users/me/tokens
func (h *SystemSecurityHandler) ListAPITokens(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var tokens []models.APIToken
	h.db.Where("user_id = ?", userID).Order("created_at desc").Find(&tokens)

	result := make([]apiTokenResponse, 0, len(tokens))
	for _, t := range tokens {
		var scopes []string
		_ = json.Unmarshal([]byte(t.Scopes), &scopes)
		if scopes == nil {
			scopes = []string{}
		}
		result = append(result, apiTokenResponse{
			ID:         t.ID,
			Name:       t.Name,
			Scopes:     scopes,
			ExpiresAt:  t.ExpiresAt,
			LastUsedAt: t.LastUsedAt,
			CreatedAt:  t.CreatedAt,
		})
	}
	response.OK(c, result)
}

// CreateAPITokenRequest 建立 Token 請求
type CreateAPITokenRequest struct {
	Name      string   `json:"name" binding:"required,max=100"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expires_at"` // RFC3339 或 null
}

// CreateAPIToken POST /users/me/tokens
func (h *SystemSecurityHandler) CreateAPIToken(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req CreateAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 用 crypto/rand 生成 32 bytes，hex 編碼為 64 字元明文 token
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { // nolint:gosec
		logger.Error("生成 Token 失敗: %v", err)
		response.InternalError(c, "生成 Token 失敗")
		return
	}
	plaintext := hex.EncodeToString(raw)

	// 儲存 SHA-256 hash
	sum := sha256.Sum256([]byte(plaintext))
	tokenHash := hex.EncodeToString(sum[:])

	// Scopes JSON
	scopesJSON := "[]"
	if len(req.Scopes) > 0 {
		b, _ := json.Marshal(req.Scopes)
		scopesJSON = string(b)
	}

	// 解析到期日（可選）
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, *req.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}

	token := models.APIToken{
		UserID:    userID.(uint),
		Name:      req.Name,
		TokenHash: tokenHash,
		Scopes:    scopesJSON,
		ExpiresAt: expiresAt,
	}
	if err := h.db.Create(&token).Error; err != nil {
		logger.Error("建立 API Token 失敗: %v", err)
		response.InternalError(c, "建立 Token 失敗")
		return
	}

	logger.Info("API Token 建立成功: user=%v name=%s", userID, req.Name)

	// 明文 token 僅此一次回傳，之後不可再取
	response.OK(c, gin.H{
		"id":         token.ID,
		"name":       token.Name,
		"token":      plaintext,
		"expires_at": token.ExpiresAt,
		"created_at": token.CreatedAt,
	})
}

// DeleteAPIToken DELETE /users/me/tokens/:id
func (h *SystemSecurityHandler) DeleteAPIToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	tokenID := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", tokenID, userID).Delete(&models.APIToken{})
	if result.Error != nil {
		logger.Error("撤銷 API Token 失敗: %v", result.Error)
		response.InternalError(c, "撤銷 Token 失敗")
		return
	}
	if result.RowsAffected == 0 {
		response.NotFound(c, "Token 不存在或無權限")
		return
	}

	logger.Info("API Token 已撤銷: user=%v id=%s", userID, tokenID)
	response.OK(c, gin.H{"message": "Token 已撤銷"})
}
