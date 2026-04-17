package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// GitProviderService — Git Provider CRUD（CICD_ARCHITECTURE §10, P2-2）
// ---------------------------------------------------------------------------

// GitProviderService 管理 Git Provider 的 CRUD 操作。
type GitProviderService struct {
	db *gorm.DB
}

// NewGitProviderService 建立 Git Provider 服務。
func NewGitProviderService(db *gorm.DB) *GitProviderService {
	return &GitProviderService{db: db}
}

// CreateProvider 建立新的 Git Provider。
func (s *GitProviderService) CreateProvider(ctx context.Context, provider *models.GitProvider) error {
	if err := validateProviderType(provider.Type); err != nil {
		return err
	}

	// 自動生成 webhook token（如果未提供）
	if provider.WebhookToken == "" {
		token, err := generateWebhookToken()
		if err != nil {
			return fmt.Errorf("generate webhook token: %w", err)
		}
		provider.WebhookToken = token
	}

	if err := s.db.WithContext(ctx).Create(provider).Error; err != nil {
		return fmt.Errorf("create git provider: %w", err)
	}

	logger.Info("git provider created",
		"provider_id", provider.ID,
		"name", provider.Name,
		"type", provider.Type,
	)
	return nil
}

// GetProvider 取得單一 Git Provider。
func (s *GitProviderService) GetProvider(ctx context.Context, id uint) (*models.GitProvider, error) {
	var provider models.GitProvider
	if err := s.db.WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("get git provider %d: %w", id, err)
	}
	return &provider, nil
}

// GetProviderByWebhookToken 透過 webhook token 查詢 provider（webhook 端點用）。
func (s *GitProviderService) GetProviderByWebhookToken(ctx context.Context, token string) (*models.GitProvider, error) {
	var provider models.GitProvider
	if err := s.db.WithContext(ctx).
		Where("webhook_token = ? AND enabled = ?", token, true).
		First(&provider).Error; err != nil {
		return nil, fmt.Errorf("get provider by webhook token: %w", err)
	}
	return &provider, nil
}

// ListProviders 列出所有 Git Provider。
func (s *GitProviderService) ListProviders(ctx context.Context) ([]models.GitProvider, error) {
	var providers []models.GitProvider
	if err := s.db.WithContext(ctx).
		Select("id, name, type, base_url, webhook_token, enabled, created_by, created_at, updated_at").
		Order("name ASC").
		Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("list git providers: %w", err)
	}
	return providers, nil
}

// UpdateProvider 更新 Git Provider。
func (s *GitProviderService) UpdateProvider(ctx context.Context, id uint, updates map[string]interface{}) error {
	if t, ok := updates["type"]; ok {
		if err := validateProviderType(t.(string)); err != nil {
			return err
		}
	}

	result := s.db.WithContext(ctx).Model(&models.GitProvider{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update git provider %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("git provider %d not found", id)
	}

	logger.Info("git provider updated", "provider_id", id)
	return nil
}

// DeleteProvider 刪除 Git Provider（soft delete）。
func (s *GitProviderService) DeleteProvider(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.GitProvider{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete git provider %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("git provider %d not found", id)
	}

	logger.Info("git provider deleted", "provider_id", id)
	return nil
}

// RegenerateWebhookToken 重新生成 Git Provider 的 webhook token。
func (s *GitProviderService) RegenerateWebhookToken(ctx context.Context, id uint) (string, error) {
	newToken, err := generateWebhookToken()
	if err != nil {
		return "", fmt.Errorf("generate webhook token: %w", err)
	}

	result := s.db.WithContext(ctx).Model(&models.GitProvider{}).
		Where("id = ?", id).
		Update("webhook_token", newToken)
	if result.Error != nil {
		return "", fmt.Errorf("regenerate webhook token for provider %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return "", fmt.Errorf("git provider %d not found", id)
	}

	logger.Info("webhook token regenerated", "provider_id", id)
	return newToken, nil
}

// ---------------------------------------------------------------------------
// TestConnection — 驗證 Git Provider 的 Base URL + Token 可連線
// ---------------------------------------------------------------------------

// TestConnection 透過 Git Provider API 驗證 Base URL 和 Access Token 是否有效。
// 回傳 nil 表示連線成功。
func (s *GitProviderService) TestConnection(ctx context.Context, providerType, baseURL, accessToken string) error {
	if baseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if accessToken == "" {
		return fmt.Errorf("access_token is required")
	}

	var apiURL string
	base := strings.TrimSuffix(baseURL, "/")
	switch providerType {
	case models.GitProviderTypeGitLab:
		apiURL = base + "/api/v4/user"
	case models.GitProviderTypeGitHub:
		if base == "https://github.com" {
			base = "https://api.github.com"
		}
		apiURL = base + "/user"
	case models.GitProviderTypeGitea:
		apiURL = base + "/api/v1/user"
	default:
		return fmt.Errorf("unsupported provider type: %s", providerType)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	switch providerType {
	case models.GitProviderTypeGitLab:
		req.Header.Set("PRIVATE-TOKEN", accessToken)
	default:
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", baseURL, err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		logger.Info("git provider connection verified", "type", providerType, "base_url", baseURL)
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: invalid access token")
	case http.StatusForbidden:
		return fmt.Errorf("access denied: token lacks required permissions")
	default:
		return fmt.Errorf("unexpected response from %s: HTTP %d", baseURL, resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// ValidateRepoConnection — 驗證 Git repo URL 可連線
// ---------------------------------------------------------------------------

// ValidateRepoConnection 透過 Git Provider API 驗證 repo URL 是否存在且可存取。
// 回傳 nil 表示連線成功；回傳 error 時包含具體原因（repo 不存在、token 無權限等）。
func (s *GitProviderService) ValidateRepoConnection(ctx context.Context, provider *models.GitProvider, repoURL string) error {
	repoPath, err := extractRepoPath(provider.BaseURL, repoURL)
	if err != nil {
		return fmt.Errorf("parse repo URL: %w", err)
	}

	var apiURL string
	switch provider.Type {
	case models.GitProviderTypeGitLab:
		encoded := url.PathEscape(repoPath)
		apiURL = strings.TrimSuffix(provider.BaseURL, "/") + "/api/v4/projects/" + encoded
	case models.GitProviderTypeGitHub:
		base := strings.TrimSuffix(provider.BaseURL, "/")
		if base == "https://github.com" {
			base = "https://api.github.com"
		}
		apiURL = base + "/repos/" + repoPath
	case models.GitProviderTypeGitea:
		apiURL = strings.TrimSuffix(provider.BaseURL, "/") + "/api/v1/repos/" + repoPath
	default:
		return fmt.Errorf("unsupported provider type: %s", provider.Type)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	// Set auth header
	token := provider.AccessTokenEnc // GORM AfterFind hook already decrypted it
	switch provider.Type {
	case models.GitProviderTypeGitLab:
		req.Header.Set("PRIVATE-TOKEN", token)
	default:
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("git API request failed: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		logger.Info("git repo validated", "repo_url", repoURL, "provider", provider.Name)
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("access denied: token has no permission to access %q", repoPath)
	case http.StatusNotFound:
		return fmt.Errorf("repository %q not found on %s", repoPath, provider.Name)
	default:
		return fmt.Errorf("unexpected response from git API: HTTP %d", resp.StatusCode)
	}
}

// NormalizeRepoURL 清理 repo URL：移除 .git 後綴、/-/tree/... 路徑、尾斜線。
// 回傳標準化的 repo URL（baseURL + / + repoPath）。
func NormalizeRepoURL(baseURL, repoURL string) string {
	path, err := extractRepoPath(baseURL, repoURL)
	if err != nil {
		return repoURL // fallback to original if parsing fails
	}
	return strings.TrimSuffix(baseURL, "/") + "/" + path
}

// extractRepoPath 從 repo URL 中解析出 owner/repo 路徑。
// 支援格式：
//
//	https://gitlab.com/root/my-repo.git  → root/my-repo
//	https://gitlab.com/root/my-repo      → root/my-repo
//	http://localhost:8929/root/my-repo    → root/my-repo
func extractRepoPath(baseURL, repoURL string) (string, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimSuffix(repoURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Try to remove baseURL prefix
	lower := strings.ToLower(repoURL)
	lowerBase := strings.ToLower(baseURL)
	var path string
	if strings.HasPrefix(lower, lowerBase) {
		path = repoURL[len(baseURL):]
		path = strings.TrimPrefix(path, "/")
	} else {
		// Fallback: parse URL and extract path
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", fmt.Errorf("invalid repo URL %q: %w", repoURL, err)
		}
		path = strings.Trim(u.Path, "/")
	}

	if path == "" {
		return "", fmt.Errorf("no repo path found in URL %q", repoURL)
	}

	// Remove GitLab sub-paths like "/-/tree/...", "/-/commits/..."
	if idx := strings.Index(path, "/-/"); idx > 0 {
		path = path[:idx]
	}

	return path, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateProviderType(providerType string) error {
	valid := map[string]bool{
		models.GitProviderTypeGitHub: true,
		models.GitProviderTypeGitLab: true,
		models.GitProviderTypeGitea:  true,
	}
	if !valid[providerType] {
		return fmt.Errorf("invalid git provider type %q, must be github|gitlab|gitea", providerType)
	}
	return nil
}

func generateWebhookToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
