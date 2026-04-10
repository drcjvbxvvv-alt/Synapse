package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
)

// PortForwardService manages port-forward session persistence.
type PortForwardService struct {
	db *gorm.DB
}

// NewPortForwardService creates a PortForwardService.
func NewPortForwardService(db *gorm.DB) *PortForwardService {
	return &PortForwardService{db: db}
}

// CreateSession persists a new port-forward session record.
func (s *PortForwardService) CreateSession(ctx context.Context, session *models.PortForwardSession) error {
	if err := s.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create port-forward session: %w", err)
	}
	return nil
}

// MarkStopped sets status=stopped and stopped_at=now on the given session.
func (s *PortForwardService) MarkStopped(sessionID uint) {
	now := time.Now()
	s.db.Model(&models.PortForwardSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{"status": "stopped", "stopped_at": &now})
}

// StopSession updates a session to stopped status (request-scoped).
func (s *PortForwardService) StopSession(ctx context.Context, sessionID uint) error {
	now := time.Now()
	if err := s.db.WithContext(ctx).
		Model(&models.PortForwardSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{"status": "stopped", "stopped_at": &now}).Error; err != nil {
		return fmt.Errorf("stop port-forward session %d: %w", sessionID, err)
	}
	return nil
}

// ListSessions returns sessions for a user, optionally filtered by status.
func (s *PortForwardService) ListSessions(ctx context.Context, userID uint, status string) ([]models.PortForwardSession, error) {
	var sessions []models.PortForwardSession
	q := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("created_at desc").Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("list port-forward sessions for user %d: %w", userID, err)
	}
	return sessions, nil
}
