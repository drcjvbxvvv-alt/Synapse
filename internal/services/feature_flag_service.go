package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"gorm.io/gorm"
)

// FeatureFlagService handles CRUD operations for the feature_flags table.
// It intentionally does NOT import internal/features to avoid circular imports:
// the features package already imports models, so services → features would
// create a cycle (services → features → models → back).
type FeatureFlagService struct {
	db *gorm.DB
}

func NewFeatureFlagService(db *gorm.DB) *FeatureFlagService {
	return &FeatureFlagService{db: db}
}

// ListFlags returns all feature flags ordered by key.
func (s *FeatureFlagService) ListFlags(ctx context.Context) ([]models.FeatureFlag, error) {
	var flags []models.FeatureFlag
	if err := s.db.WithContext(ctx).Order("`key` ASC").Find(&flags).Error; err != nil {
		return nil, fmt.Errorf("list feature flags: %w", err)
	}
	return flags, nil
}

// SetFlag upserts a feature flag. If the row does not exist it is created;
// otherwise Enabled, Description, and UpdatedBy are updated.
func (s *FeatureFlagService) SetFlag(
	ctx context.Context,
	key string,
	enabled bool,
	description string,
	updatedBy string,
) error {
	var flag models.FeatureFlag
	result := s.db.WithContext(ctx).Where("`key` = ?", key).First(&flag)

	if result.Error == gorm.ErrRecordNotFound {
		flag = models.FeatureFlag{
			Key:         key,
			Enabled:     enabled,
			Description: description,
			UpdatedBy:   updatedBy,
		}
		if err := s.db.WithContext(ctx).Create(&flag).Error; err != nil {
			return fmt.Errorf("create feature flag %s: %w", key, err)
		}
		return nil
	}
	if result.Error != nil {
		return fmt.Errorf("query feature flag %s: %w", key, result.Error)
	}

	flag.Enabled = enabled
	if description != "" {
		flag.Description = description
	}
	flag.UpdatedBy = updatedBy
	if err := s.db.WithContext(ctx).Save(&flag).Error; err != nil {
		return fmt.Errorf("update feature flag %s: %w", key, err)
	}
	return nil
}
