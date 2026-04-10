package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
)

// SyncPolicyService manages multi-cluster sync policies and their history.
type SyncPolicyService struct {
	db *gorm.DB
}

// NewSyncPolicyService creates a SyncPolicyService.
func NewSyncPolicyService(db *gorm.DB) *SyncPolicyService {
	return &SyncPolicyService{db: db}
}

// ListPolicies returns all sync policies ordered newest first.
func (s *SyncPolicyService) ListPolicies(ctx context.Context) ([]models.SyncPolicy, error) {
	var policies []models.SyncPolicy
	if err := s.db.WithContext(ctx).Order("id desc").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("list sync policies: %w", err)
	}
	return policies, nil
}

// CreatePolicy persists a new sync policy.
func (s *SyncPolicyService) CreatePolicy(ctx context.Context, policy *models.SyncPolicy) error {
	if err := s.db.WithContext(ctx).Create(policy).Error; err != nil {
		return fmt.Errorf("create sync policy: %w", err)
	}
	return nil
}

// GetPolicy fetches a sync policy by ID.
func (s *SyncPolicyService) GetPolicy(ctx context.Context, id uint) (*models.SyncPolicy, error) {
	var policy models.SyncPolicy
	if err := s.db.WithContext(ctx).First(&policy, id).Error; err != nil {
		return nil, fmt.Errorf("get sync policy %d: %w", id, err)
	}
	return &policy, nil
}

// UpdatePolicy saves changes to an existing sync policy.
func (s *SyncPolicyService) UpdatePolicy(ctx context.Context, policy *models.SyncPolicy) error {
	if err := s.db.WithContext(ctx).Save(policy).Error; err != nil {
		return fmt.Errorf("update sync policy %d: %w", policy.ID, err)
	}
	return nil
}

// DeletePolicy removes a sync policy by ID.
func (s *SyncPolicyService) DeletePolicy(ctx context.Context, id uint) error {
	if err := s.db.WithContext(ctx).Delete(&models.SyncPolicy{}, id).Error; err != nil {
		return fmt.Errorf("delete sync policy %d: %w", id, err)
	}
	return nil
}

// RecordSyncHistory persists a sync execution record.
func (s *SyncPolicyService) RecordSyncHistory(ctx context.Context, hist *models.SyncHistory) {
	s.db.WithContext(ctx).Create(hist)
}

// UpdatePolicySyncStatus updates last_sync_at and last_sync_status on a policy.
func (s *SyncPolicyService) UpdatePolicySyncStatus(ctx context.Context, policy *models.SyncPolicy, status string) {
	s.db.WithContext(ctx).Model(policy).Updates(map[string]interface{}{
		"last_sync_at":     policy.LastSyncAt,
		"last_sync_status": status,
	})
}

// ListSyncHistory returns the most recent sync history records for a policy.
func (s *SyncPolicyService) ListSyncHistory(ctx context.Context, policyID uint, limit int) ([]models.SyncHistory, error) {
	var history []models.SyncHistory
	if err := s.db.WithContext(ctx).
		Where("policy_id = ?", policyID).
		Order("id desc").Limit(limit).
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("list sync history for policy %d: %w", policyID, err)
	}
	return history, nil
}
