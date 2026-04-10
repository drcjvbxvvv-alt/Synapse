package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
)

// ApprovalService manages approval requests and namespace protection settings.
type ApprovalService struct {
	db *gorm.DB
}

// NewApprovalService creates an ApprovalService.
func NewApprovalService(db *gorm.DB) *ApprovalService {
	return &ApprovalService{db: db}
}

// CreateApprovalRequest persists a new approval request.
func (s *ApprovalService) CreateApprovalRequest(ctx context.Context, ar *models.ApprovalRequest) error {
	if err := s.db.WithContext(ctx).Create(ar).Error; err != nil {
		return fmt.Errorf("create approval request: %w", err)
	}
	return nil
}

// ExpireStaleRequests marks pending requests past their expiry as expired.
func (s *ApprovalService) ExpireStaleRequests(ctx context.Context) {
	s.db.WithContext(ctx).
		Model(&models.ApprovalRequest{}).
		Where("status = ? AND expires_at < ?", "pending", time.Now()).
		Update("status", "expired")
}

// ListApprovalRequests returns requests filtered by optional status and clusterID.
func (s *ApprovalService) ListApprovalRequests(ctx context.Context, status string, clusterID uint) ([]models.ApprovalRequest, error) {
	q := s.db.WithContext(ctx).Model(&models.ApprovalRequest{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if clusterID > 0 {
		q = q.Where("cluster_id = ?", clusterID)
	}
	var items []models.ApprovalRequest
	if err := q.Order("created_at desc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list approval requests: %w", err)
	}
	return items, nil
}

// GetApprovalRequest fetches a single request by ID.
func (s *ApprovalService) GetApprovalRequest(ctx context.Context, id uint) (*models.ApprovalRequest, error) {
	var ar models.ApprovalRequest
	if err := s.db.WithContext(ctx).First(&ar, id).Error; err != nil {
		return nil, fmt.Errorf("get approval request %d: %w", id, err)
	}
	return &ar, nil
}

// UpdateApprovalRequest applies a partial update map to the request.
func (s *ApprovalService) UpdateApprovalRequest(ctx context.Context, ar *models.ApprovalRequest, updates map[string]interface{}) error {
	if err := s.db.WithContext(ctx).Model(ar).Updates(updates).Error; err != nil {
		return fmt.Errorf("update approval request %d: %w", ar.ID, err)
	}
	return nil
}

// GetPendingCount returns the count of currently pending approval requests.
func (s *ApprovalService) GetPendingCount(ctx context.Context) int64 {
	var count int64
	s.db.WithContext(ctx).Model(&models.ApprovalRequest{}).Where("status = ?", "pending").Count(&count)
	return count
}

// ListNamespaceProtections returns all protection settings for a cluster.
func (s *ApprovalService) ListNamespaceProtections(ctx context.Context, clusterID uint) ([]models.NamespaceProtection, error) {
	var items []models.NamespaceProtection
	if err := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list namespace protections for cluster %d: %w", clusterID, err)
	}
	return items, nil
}

// UpsertNamespaceProtection creates or updates a namespace protection record.
func (s *ApprovalService) UpsertNamespaceProtection(ctx context.Context, clusterID uint, namespace string, requireApproval bool, description string) (*models.NamespaceProtection, error) {
	var np models.NamespaceProtection
	result := s.db.WithContext(ctx).
		Where("cluster_id = ? AND namespace = ?", clusterID, namespace).
		First(&np)
	if result.Error != nil {
		np = models.NamespaceProtection{
			ClusterID:       clusterID,
			Namespace:       namespace,
			RequireApproval: requireApproval,
			Description:     description,
		}
		if err := s.db.WithContext(ctx).Create(&np).Error; err != nil {
			return nil, fmt.Errorf("create namespace protection: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Model(&np).Updates(map[string]interface{}{
			"require_approval": requireApproval,
			"description":      description,
		}).Error; err != nil {
			return nil, fmt.Errorf("update namespace protection: %w", err)
		}
	}
	return &np, nil
}

// GetNamespaceProtection returns the protection record for a specific namespace (nil if not set).
func (s *ApprovalService) GetNamespaceProtection(ctx context.Context, clusterID uint, namespace string) (*models.NamespaceProtection, error) {
	var np models.NamespaceProtection
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND namespace = ?", clusterID, namespace).
		First(&np).Error; err != nil {
		return nil, err // caller checks gorm.ErrRecordNotFound
	}
	return &np, nil
}
