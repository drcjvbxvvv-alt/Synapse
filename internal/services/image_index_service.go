package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
)

// ImageIndexService manages the cross-cluster image index.
type ImageIndexService struct {
	db *gorm.DB
}

// NewImageIndexService creates an ImageIndexService.
func NewImageIndexService(db *gorm.DB) *ImageIndexService {
	return &ImageIndexService{db: db}
}

// DeleteByCluster removes all image index entries for a cluster.
func (s *ImageIndexService) DeleteByCluster(ctx context.Context, clusterID uint) {
	s.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Delete(&models.ImageIndex{})
}

// BulkCreate inserts image index entries in batches of 100.
func (s *ImageIndexService) BulkCreate(ctx context.Context, entries []models.ImageIndex) error {
	if len(entries) == 0 {
		return nil
	}
	if err := s.db.WithContext(ctx).CreateInBatches(entries, 100).Error; err != nil {
		return fmt.Errorf("bulk create image index entries: %w", err)
	}
	return nil
}

// SearchImagesParams holds filter criteria for SearchImages.
type SearchImagesParams struct {
	Query     string
	Tag       string
	Namespace string
	ClusterID string
	Page      int
	Limit     int
}

// SearchImages returns paginated image index results matching the given criteria.
func (s *ImageIndexService) SearchImages(ctx context.Context, p SearchImagesParams) ([]models.ImageIndex, int64, error) {
	q := s.db.WithContext(ctx).Model(&models.ImageIndex{})
	if p.Query != "" {
		q = q.Where("image_name LIKE ? OR image LIKE ?", "%"+p.Query+"%", "%"+p.Query+"%")
	}
	if p.Tag != "" {
		q = q.Where("image_tag LIKE ?", "%"+p.Tag+"%")
	}
	if p.Namespace != "" {
		q = q.Where("namespace = ?", p.Namespace)
	}
	if p.ClusterID != "" {
		q = q.Where("cluster_id = ?", p.ClusterID)
	}

	var total int64
	q.Count(&total)

	var items []models.ImageIndex
	if err := q.Offset((p.Page - 1) * p.Limit).Limit(p.Limit).
		Order("cluster_name, namespace, workload_name").
		Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("search image index: %w", err)
	}
	return items, total, nil
}

// GetSyncStatus returns the total indexed count and last sync timestamp.
func (s *ImageIndexService) GetSyncStatus(ctx context.Context) (count int64, lastSyncAt time.Time) {
	s.db.WithContext(ctx).Model(&models.ImageIndex{}).Count(&count)
	var lastSync models.ImageIndex
	s.db.WithContext(ctx).Model(&models.ImageIndex{}).Order("last_sync_at desc").First(&lastSync)
	return count, lastSync.LastSyncAt
}
