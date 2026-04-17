package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ArgoCDService ArgoCD 服務
type ArgoCDService struct {
	db         *gorm.DB
	httpClient *http.Client
}

// NewArgoCDService 建立 ArgoCD 服務
func NewArgoCDService(db *gorm.DB) *ArgoCDService {
	return &ArgoCDService{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetConfig 獲取 ArgoCD 配置
func (s *ArgoCDService) GetConfig(ctx context.Context, clusterID uint) (*models.ArgoCDConfig, error) {
	var config models.ArgoCDConfig
	if err := s.db.Where("cluster_id = ?", clusterID).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回預設空配置
			return &models.ArgoCDConfig{
				ClusterID:     clusterID,
				Enabled:       false,
				GitBranch:     "main",
				ArgoCDProject: "default",
			}, nil
		}
		return nil, fmt.Errorf("query argocd config for cluster %d: %w", clusterID, err)
	}
	return &config, nil
}

// SaveConfig 儲存 ArgoCD 配置
func (s *ArgoCDService) SaveConfig(ctx context.Context, config *models.ArgoCDConfig) error {
	var existing models.ArgoCDConfig
	if err := s.db.Where("cluster_id = ?", config.ClusterID).First(&existing).Error; err == nil {
		// 更新
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		if err := s.db.Save(config).Error; err != nil {
			return fmt.Errorf("save argocd config for cluster %d: %w", config.ClusterID, err)
		}
		return nil
	}
	// 新建
	if err := s.db.Create(config).Error; err != nil {
		return fmt.Errorf("create argocd config for cluster %d: %w", config.ClusterID, err)
	}
	return nil
}

// TestConnection 測試 ArgoCD 連線
func (s *ArgoCDService) TestConnection(ctx context.Context, config *models.ArgoCDConfig) error {
	client := s.createHTTPClient(config.Insecure)

	// 如果使用使用者名稱密碼認證，先嚐試登入獲取 token
	var authToken string
	if config.Token != "" {
		authToken = config.Token
	} else if config.Username != "" && config.Password != "" {
		// 使用使用者名稱密碼登入
		token, err := s.getSessionToken(config)
		if err != nil {
			return fmt.Errorf("使用者名稱密碼認證失敗: %w", err)
		}
		authToken = token
	} else {
		return fmt.Errorf("請提供 API Token 或使用者名稱密碼")
	}

	// 使用獲取到的 token 驗證連線
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/v1/session/userinfo", config.ServerURL), nil)
	if err != nil {
		return fmt.Errorf("建立請求失敗: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("連線 ArgoCD 失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("認證失敗: Token 無效或已過期")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ArgoCD 響應錯誤 (狀態碼 %d): %s", resp.StatusCode, string(body))
	}

	// 更新連線狀態
	now := time.Now()
	config.ConnectionStatus = "connected"
	config.LastTestAt = &now
	config.ErrorMessage = ""

	return nil
}

// 輔助方法

func (s *ArgoCDService) createHTTPClient(insecure bool) *http.Client {
	if insecure {
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- ArgoCD 常使用自簽名證書，insecure 由使用者配置控制
			},
		}
	}
	return s.httpClient
}

func (s *ArgoCDService) setAuthHeader(req *http.Request, config *models.ArgoCDConfig) {
	// 優先使用 Token 認證
	if config.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Token))
		return
	}

	// 如果沒有 Token 但有使用者名稱密碼，嘗試獲取 session token
	if config.Username != "" && config.Password != "" {
		token, err := s.getSessionToken(config)
		if err != nil {
			logger.Error("獲取 ArgoCD session token 失敗", "error", err)
			return
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
}

// getSessionToken 使用使用者名稱密碼獲取 ArgoCD session token
func (s *ArgoCDService) getSessionToken(config *models.ArgoCDConfig) (string, error) {
	client := s.createHTTPClient(config.Insecure)

	// 構建登入請求
	loginReq := map[string]string{
		"username": config.Username,
		"password": config.Password,
	}
	body, err := json.Marshal(loginReq)
	if err != nil {
		return "", fmt.Errorf("marshal login request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/session", config.ServerURL), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("建立登入請求失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("登入請求失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("登入失敗 (狀態碼 %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析響應獲取 token
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析登入響應失敗: %w", err)
	}

	if result.Token == "" {
		return "", fmt.Errorf("登入成功但未返回 token")
	}

	logger.Info("ArgoCD 登入成功，獲取到 session token")
	return result.Token, nil
}
