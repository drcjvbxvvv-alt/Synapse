package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
)

// NotifyChannelService wraps NotifyChannel CRUD, keeping raw DB access out of
// the handler layer.
type NotifyChannelService struct {
	db *gorm.DB
}

func NewNotifyChannelService(db *gorm.DB) *NotifyChannelService {
	return &NotifyChannelService{db: db}
}

// List returns all notify channels ordered by creation time.
func (s *NotifyChannelService) List(ctx context.Context) ([]models.NotifyChannel, error) {
	var channels []models.NotifyChannel
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list notify channels: %w", err)
	}
	return channels, nil
}

// Get returns a single notify channel by ID.
func (s *NotifyChannelService) Get(ctx context.Context, id uint) (*models.NotifyChannel, error) {
	var ch models.NotifyChannel
	if err := s.db.WithContext(ctx).First(&ch, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repositories.ErrNotFound
		}
		return nil, fmt.Errorf("get notify channel %d: %w", id, err)
	}
	return &ch, nil
}

// Create inserts a new notify channel.
func (s *NotifyChannelService) Create(ctx context.Context, ch *models.NotifyChannel) error {
	if err := s.db.WithContext(ctx).Create(ch).Error; err != nil {
		return fmt.Errorf("create notify channel: %w", err)
	}
	return nil
}

// Save upserts a notify channel (used for updates where the record already exists).
func (s *NotifyChannelService) Save(ctx context.Context, ch *models.NotifyChannel) error {
	if err := s.db.WithContext(ctx).Save(ch).Error; err != nil {
		return fmt.Errorf("save notify channel %d: %w", ch.ID, err)
	}
	return nil
}

// Delete removes a notify channel by ID.
func (s *NotifyChannelService) Delete(ctx context.Context, id uint) error {
	if err := s.db.WithContext(ctx).Delete(&models.NotifyChannel{}, id).Error; err != nil {
		return fmt.Errorf("delete notify channel %d: %w", id, err)
	}
	return nil
}
