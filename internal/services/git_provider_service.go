package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

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
