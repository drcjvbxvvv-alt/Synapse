package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
)

// LogSourceService manages external log source configurations.
type LogSourceService struct {
	db *gorm.DB
}

// NewLogSourceService creates a LogSourceService.
func NewLogSourceService(db *gorm.DB) *LogSourceService {
	return &LogSourceService{db: db}
}

// ListLogSources returns all log sources for a cluster, with credentials masked.
func (s *LogSourceService) ListLogSources(ctx context.Context, clusterID uint) ([]models.LogSourceConfig, error) {
	var sources []models.LogSourceConfig
	if err := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Find(&sources).Error; err != nil {
		return nil, fmt.Errorf("list log sources for cluster %d: %w", clusterID, err)
	}
	for i := range sources {
		sources[i].Password = ""
		sources[i].APIKey = ""
	}
	return sources, nil
}

// CreateLogSource persists a new log source and masks credentials in the returned value.
func (s *LogSourceService) CreateLogSource(ctx context.Context, src *models.LogSourceConfig) error {
	if err := s.db.WithContext(ctx).Create(src).Error; err != nil {
		return fmt.Errorf("create log source: %w", err)
	}
	src.Password = ""
	src.APIKey = ""
	return nil
}

// GetLogSource fetches a single log source owned by the given cluster.
func (s *LogSourceService) GetLogSource(ctx context.Context, id uint, clusterID uint) (*models.LogSourceConfig, error) {
	var src models.LogSourceConfig
	if err := s.db.WithContext(ctx).
		Where("id = ? AND cluster_id = ?", id, clusterID).
		First(&src).Error; err != nil {
		return nil, fmt.Errorf("get log source %d: %w", id, err)
	}
	return &src, nil
}

// GetEnabledLogSource fetches a single enabled log source owned by the given cluster.
func (s *LogSourceService) GetEnabledLogSource(ctx context.Context, id uint, clusterID uint) (*models.LogSourceConfig, error) {
	var src models.LogSourceConfig
	if err := s.db.WithContext(ctx).
		Where("id = ? AND cluster_id = ? AND enabled = ?", id, clusterID, true).
		First(&src).Error; err != nil {
		return nil, fmt.Errorf("get enabled log source %d: %w", id, err)
	}
	return &src, nil
}

// UpdateLogSource applies a partial update map to the log source.
func (s *LogSourceService) UpdateLogSource(ctx context.Context, src *models.LogSourceConfig, updates map[string]interface{}) error {
	if err := s.db.WithContext(ctx).Model(src).Updates(updates).Error; err != nil {
		return fmt.Errorf("update log source %d: %w", src.ID, err)
	}
	return nil
}

// DeleteLogSource removes a log source that belongs to the given cluster.
func (s *LogSourceService) DeleteLogSource(ctx context.Context, id uint, clusterID uint) error {
	if err := s.db.WithContext(ctx).
		Where("id = ? AND cluster_id = ?", id, clusterID).
		Delete(&models.LogSourceConfig{}).Error; err != nil {
		return fmt.Errorf("delete log source %d: %w", id, err)
	}
	return nil
}
