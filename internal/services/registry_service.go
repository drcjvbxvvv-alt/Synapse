package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// RegistryService — Registry CRUD（CICD_ARCHITECTURE §11, P2-3）
// ---------------------------------------------------------------------------

// RegistryService 管理 Registry 的 CRUD 操作。
type RegistryService struct {
	db *gorm.DB
}

// NewRegistryService 建立 Registry 服務。
func NewRegistryService(db *gorm.DB) *RegistryService {
	return &RegistryService{db: db}
}

// CreateRegistry 建立新的 Registry。
func (s *RegistryService) CreateRegistry(ctx context.Context, registry *models.Registry) error {
	if err := validateRegistryType(registry.Type); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Create(registry).Error; err != nil {
		return fmt.Errorf("create registry: %w", err)
	}

	logger.Info("registry created",
		"registry_id", registry.ID,
		"name", registry.Name,
		"type", registry.Type,
	)
	return nil
}

// GetRegistry 取得單一 Registry。
func (s *RegistryService) GetRegistry(ctx context.Context, id uint) (*models.Registry, error) {
	var registry models.Registry
	if err := s.db.WithContext(ctx).First(&registry, id).Error; err != nil {
		return nil, fmt.Errorf("get registry %d: %w", id, err)
	}
	return &registry, nil
}

// GetRegistryByName 透過名稱查詢 Registry（Pipeline Step 用）。
func (s *RegistryService) GetRegistryByName(ctx context.Context, name string) (*models.Registry, error) {
	var registry models.Registry
	if err := s.db.WithContext(ctx).
		Where("name = ? AND enabled = ?", name, true).
		First(&registry).Error; err != nil {
		return nil, fmt.Errorf("get registry by name %q: %w", name, err)
	}
	return &registry, nil
}

// ListRegistries 列出所有 Registry（不含加密欄位）。
func (s *RegistryService) ListRegistries(ctx context.Context) ([]models.Registry, error) {
	var registries []models.Registry
	if err := s.db.WithContext(ctx).
		Select("id, name, type, url, username, insecure_tls, default_project, enabled, created_by, created_at, updated_at").
		Order("name ASC").
		Find(&registries).Error; err != nil {
		return nil, fmt.Errorf("list registries: %w", err)
	}
	return registries, nil
}

// UpdateRegistry 更新 Registry。
func (s *RegistryService) UpdateRegistry(ctx context.Context, id uint, updates map[string]interface{}) error {
	if t, ok := updates["type"]; ok {
		if err := validateRegistryType(t.(string)); err != nil {
			return err
		}
	}

	result := s.db.WithContext(ctx).Model(&models.Registry{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update registry %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("registry %d not found", id)
	}

	logger.Info("registry updated", "registry_id", id)
	return nil
}

// DeleteRegistry 刪除 Registry（soft delete）。
func (s *RegistryService) DeleteRegistry(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.Registry{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete registry %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("registry %d not found", id)
	}

	logger.Info("registry deleted", "registry_id", id)
	return nil
}

// TestConnection 測試 Registry 連線。
func (s *RegistryService) TestConnection(ctx context.Context, id uint) error {
	registry, err := s.GetRegistry(ctx, id)
	if err != nil {
		return err
	}

	adapter, err := NewRegistryAdapter(registry)
	if err != nil {
		return err
	}

	return adapter.Ping(ctx)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateRegistryType(registryType string) error {
	valid := map[string]bool{
		models.RegistryTypeHarbor:    true,
		models.RegistryTypeDockerHub: true,
		models.RegistryTypeACR:       true,
		models.RegistryTypeECR:       true,
		models.RegistryTypeGCR:       true,
	}
	if !valid[registryType] {
		return fmt.Errorf("invalid registry type %q, must be harbor|dockerhub|acr|ecr|gcr", registryType)
	}
	return nil
}
