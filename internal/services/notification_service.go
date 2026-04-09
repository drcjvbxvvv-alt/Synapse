package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
)

// NotificationService wraps EventAlertHistory DB operations, keeping raw DB
// access out of the handler layer.
type NotificationService struct {
	db *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

// ListRecent returns the most recent `limit` EventAlertHistory rows.
func (s *NotificationService) ListRecent(ctx context.Context, limit int) ([]models.EventAlertHistory, error) {
	var rows []models.EventAlertHistory
	if err := s.db.WithContext(ctx).
		Order("triggered_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return rows, nil
}

// ClusterNames returns a map of id → name for the given cluster IDs.
func (s *NotificationService) ClusterNames(ctx context.Context, ids []uint) (map[uint]string, error) {
	if len(ids) == 0 {
		return map[uint]string{}, nil
	}
	var clusters []models.Cluster
	if err := s.db.WithContext(ctx).
		Select("id, name").
		Where("id IN ?", ids).
		Find(&clusters).Error; err != nil {
		return nil, fmt.Errorf("fetch cluster names: %w", err)
	}
	m := make(map[uint]string, len(clusters))
	for _, cl := range clusters {
		m[cl.ID] = cl.Name
	}
	return m, nil
}

// MarkRead marks a single notification as read.
func (s *NotificationService) MarkRead(ctx context.Context, id uint) error {
	if err := s.db.WithContext(ctx).
		Model(&models.EventAlertHistory{}).
		Where("id = ?", id).
		Update("is_read", true).Error; err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

// MarkAllRead marks all unread notifications as read.
func (s *NotificationService) MarkAllRead(ctx context.Context) error {
	if err := s.db.WithContext(ctx).
		Model(&models.EventAlertHistory{}).
		Where("is_read = ?", false).
		Update("is_read", true).Error; err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// CountUnread returns the number of unread notifications.
func (s *NotificationService) CountUnread(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&models.EventAlertHistory{}).
		Where("is_read = ?", false).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}
