package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

// ConfigVersionService persists and retrieves version snapshots for
// ConfigMap and Secret resources.
type ConfigVersionService struct {
	db *gorm.DB
}

// NewConfigVersionService creates a ConfigVersionService.
func NewConfigVersionService(db *gorm.DB) *ConfigVersionService {
	return &ConfigVersionService{db: db}
}

// SaveConfigMapVersion saves a ConfigMap data snapshot.
func (s *ConfigVersionService) SaveConfigMapVersion(ctx context.Context, clusterID uint, namespace, name, changedBy string, data map[string]string) {
	contentBytes, _ := json.Marshal(data)
	s.saveVersion(ctx, clusterID, "configmap", namespace, name, changedBy, string(contentBytes))
}

// SaveSecretVersion saves a Secret key-list snapshot (values are NOT stored).
func (s *ConfigVersionService) SaveSecretVersion(ctx context.Context, clusterID uint, namespace, name, changedBy string, data map[string][]byte) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	contentBytes, _ := json.Marshal(map[string]interface{}{
		"keys": keys,
		"note": "Secret value not stored for security",
	})
	s.saveVersion(ctx, clusterID, "secret", namespace, name, changedBy, string(contentBytes))
}

func (s *ConfigVersionService) saveVersion(ctx context.Context, clusterID uint, resourceType, namespace, name, changedBy, contentJSON string) {
	// Transaction 保證 SELECT MAX(version) + CREATE 的原子性，
	// 防止並發寫入時產生重複版本號。
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var nextVer int
		tx.Model(&models.ConfigVersion{}).
			Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ?",
				clusterID, resourceType, namespace, name).
			Select("COALESCE(MAX(version),0) + 1").Scan(&nextVer)
		if nextVer == 0 {
			nextVer = 1
		}
		ver := &models.ConfigVersion{
			ClusterID:    clusterID,
			ResourceType: resourceType,
			Namespace:    namespace,
			Name:         name,
			Version:      nextVer,
			ContentJSON:  contentJSON,
			ChangedBy:    changedBy,
			ChangedAt:    time.Now(),
		}
		return tx.Create(ver).Error
	}); err != nil {
		logger.Warn("儲存版本快照失敗", "resourceType", resourceType, "name", name, "error", err)
	}
}

// ListVersions returns all version snapshots for a resource, newest first.
func (s *ConfigVersionService) ListVersions(ctx context.Context, clusterID uint, resourceType, namespace, name string) ([]models.ConfigVersion, error) {
	var versions []models.ConfigVersion
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ?", clusterID, resourceType, namespace, name).
		Order("version DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list versions for %s/%s/%s: %w", resourceType, namespace, name, err)
	}
	return versions, nil
}

// GetVersion fetches a specific version snapshot.
func (s *ConfigVersionService) GetVersion(ctx context.Context, clusterID uint, resourceType, namespace, name string, version int) (*models.ConfigVersion, error) {
	var ver models.ConfigVersion
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ? AND version = ?",
			clusterID, resourceType, namespace, name, version).
		First(&ver).Error; err != nil {
		return nil, fmt.Errorf("get version %d for %s/%s/%s: %w", version, resourceType, namespace, name, err)
	}
	return &ver, nil
}
