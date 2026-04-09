package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
)

const securityConfigKey = "security_config"

// SystemSecurityService wraps SystemSecurityConfig and APIToken DB operations.
type SystemSecurityService struct {
	db *gorm.DB
}

func NewSystemSecurityService(db *gorm.DB) *SystemSecurityService {
	return &SystemSecurityService{db: db}
}

// GetSecurityConfig loads the security config from DB; returns defaults when absent.
func (s *SystemSecurityService) GetSecurityConfig(ctx context.Context) (*models.SystemSecurityConfig, error) {
	var setting models.SystemSetting
	err := s.db.WithContext(ctx).Where("config_key = ?", securityConfigKey).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg := models.GetDefaultSystemSecurityConfig()
			return &cfg, nil
		}
		return nil, fmt.Errorf("get security config: %w", err)
	}
	var cfg models.SystemSecurityConfig
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		def := models.GetDefaultSystemSecurityConfig()
		return &def, nil
	}
	return &cfg, nil
}

// SaveSecurityConfig upserts the security config.
func (s *SystemSecurityService) SaveSecurityConfig(ctx context.Context, cfg *models.SystemSecurityConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal security config: %w", err)
	}

	var setting models.SystemSetting
	s.db.WithContext(ctx).Where("config_key = ?", securityConfigKey).First(&setting)
	setting.ConfigKey = securityConfigKey
	setting.Type = "security"
	setting.Value = string(b)

	if setting.ID == 0 {
		if err := s.db.WithContext(ctx).Create(&setting).Error; err != nil {
			return fmt.Errorf("create security config: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(&setting).Error; err != nil {
			return fmt.Errorf("save security config: %w", err)
		}
	}
	return nil
}

// ListAPITokens returns all active API tokens for the given user.
func (s *SystemSecurityService) ListAPITokens(ctx context.Context, userID uint) ([]models.APIToken, error) {
	var tokens []models.APIToken
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	return tokens, nil
}

// CreateAPIToken inserts a new API token.
func (s *SystemSecurityService) CreateAPIToken(ctx context.Context, token *models.APIToken) error {
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create api token: %w", err)
	}
	return nil
}

// DeleteAPIToken removes an API token belonging to the given user.
func (s *SystemSecurityService) DeleteAPIToken(ctx context.Context, id, userID uint) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.APIToken{})
	if result.Error != nil {
		return fmt.Errorf("delete api token %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return repositories.ErrNotFound
	}
	return nil
}
