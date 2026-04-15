package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// PipelineSecretService — Pipeline Secret CRUD
// 加密 / 解密由 PipelineSecret 的 GORM hooks 透明處理。
// ---------------------------------------------------------------------------

// PipelineSecretService 管理 CI/CD Pipeline 使用的敏感憑證。
type PipelineSecretService struct {
	db *gorm.DB
}

// NewPipelineSecretService 建立 PipelineSecretService。
func NewPipelineSecretService(db *gorm.DB) *PipelineSecretService {
	return &PipelineSecretService{db: db}
}

// CreateSecretRequest 建立 Secret 請求。
type CreateSecretRequest struct {
	Scope       string `json:"scope" binding:"required,oneof=global environment pipeline"`
	ScopeRef    *uint  `json:"scope_ref"`
	Name        string `json:"name" binding:"required,max=100"`
	Value       string `json:"value" binding:"required"`
	Description string `json:"description" binding:"max=255"`
}

// UpdateSecretRequest 更新 Secret 請求。
type UpdateSecretRequest struct {
	Value       *string `json:"value"`
	Description *string `json:"description"`
}

// CreateSecret 建立 Pipeline Secret。
func (s *PipelineSecretService) CreateSecret(ctx context.Context, req *CreateSecretRequest, createdBy uint) (*models.PipelineSecret, error) {
	// 檢查同 scope 下名稱是否重複
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.PipelineSecret{}).
		Where("scope = ? AND name = ? AND COALESCE(scope_ref, 0) = COALESCE(?, 0)", req.Scope, req.Name, req.ScopeRef).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("check secret duplicate: %w", err)
	}
	if count > 0 {
		return nil, apierrors.ErrPipelineSecretDuplicateName()
	}

	secret := &models.PipelineSecret{
		Scope:       req.Scope,
		ScopeRef:    req.ScopeRef,
		Name:        req.Name,
		ValueEnc:    req.Value, // BeforeSave hook 會加密
		Description: req.Description,
		CreatedBy:   createdBy,
	}

	if err := s.db.WithContext(ctx).Create(secret).Error; err != nil {
		return nil, fmt.Errorf("create pipeline secret: %w", err)
	}
	return secret, nil
}

// GetSecret 取得單一 Secret（不含解密值）。
func (s *PipelineSecretService) GetSecret(ctx context.Context, id uint) (*models.PipelineSecret, error) {
	var secret models.PipelineSecret
	if err := s.db.WithContext(ctx).First(&secret, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.ErrPipelineSecretNotFound()
		}
		return nil, fmt.Errorf("get pipeline secret %d: %w", id, err)
	}
	return &secret, nil
}

// ListSecrets 列出指定 scope 下的 Secrets。
func (s *PipelineSecretService) ListSecrets(ctx context.Context, scope string, scopeRef *uint) ([]models.PipelineSecret, error) {
	query := s.db.WithContext(ctx).
		Select("id, scope, scope_ref, name, description, created_by, created_at, updated_at").
		Model(&models.PipelineSecret{})

	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	if scopeRef != nil {
		query = query.Where("scope_ref = ?", *scopeRef)
	}

	var secrets []models.PipelineSecret
	if err := query.Order("name ASC").Find(&secrets).Error; err != nil {
		return nil, fmt.Errorf("list pipeline secrets: %w", err)
	}
	return secrets, nil
}

// UpdateSecret 更新 Secret 的值或描述。
func (s *PipelineSecretService) UpdateSecret(ctx context.Context, id uint, req *UpdateSecretRequest) (*models.PipelineSecret, error) {
	secret, err := s.GetSecret(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Value != nil {
		secret.ValueEnc = *req.Value // BeforeSave hook 會加密
	}
	if req.Description != nil {
		secret.Description = *req.Description
	}

	if err := s.db.WithContext(ctx).Save(secret).Error; err != nil {
		return nil, fmt.Errorf("update pipeline secret %d: %w", id, err)
	}
	return secret, nil
}

// DeleteSecret 刪除 Secret（軟刪除）。
func (s *PipelineSecretService) DeleteSecret(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.PipelineSecret{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete pipeline secret %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return apierrors.ErrPipelineSecretNotFound()
	}
	return nil
}
