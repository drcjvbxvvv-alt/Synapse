package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// ProjectService — Project CRUD（CICD_ARCHITECTURE §M14.1）
//
// Project = Git Provider 下的程式碼倉庫。
// Pipeline 可選擇性關聯 Project，讓 Webhook 觸發精確比對 repo URL。
// ---------------------------------------------------------------------------

// ProjectService 管理 Project 資源的 CRUD 操作。
type ProjectService struct {
	db *gorm.DB
}

// NewProjectService 建立 ProjectService。
func NewProjectService(db *gorm.DB) *ProjectService {
	return &ProjectService{db: db}
}

// ─── Read ──────────────────────────────────────────────────────────────────

// ListProjects 列出某 Git Provider 下的所有 Projects。
func (s *ProjectService) ListProjects(ctx context.Context, providerID uint) ([]models.Project, error) {
	var projects []models.Project
	if err := s.db.WithContext(ctx).
		Where("git_provider_id = ?", providerID).
		Order("name ASC").
		Find(&projects).Error; err != nil {
		return nil, fmt.Errorf("list projects for provider %d: %w", providerID, err)
	}
	return projects, nil
}

// GetProject 取得單一 Project。
func (s *ProjectService) GetProject(ctx context.Context, id uint) (*models.Project, error) {
	var project models.Project
	if err := s.db.WithContext(ctx).First(&project, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("project %d not found: %w", id, err)
		}
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	return &project, nil
}

// GetProjectByRepoURL 以 RepoURL 查找 Project（Webhook 觸發比對用）。
func (s *ProjectService) GetProjectByRepoURL(ctx context.Context, repoURL string) (*models.Project, error) {
	var project models.Project
	if err := s.db.WithContext(ctx).
		Where("repo_url = ?", repoURL).
		First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("project with repo_url %q not found: %w", repoURL, err)
		}
		return nil, fmt.Errorf("get project by repo_url: %w", err)
	}
	return &project, nil
}

// ─── Write ─────────────────────────────────────────────────────────────────

// CreateProject 建立新 Project。
func (s *ProjectService) CreateProject(ctx context.Context, project *models.Project) error {
	logger.Info("creating project",
		"provider_id", project.GitProviderID,
		"name", project.Name,
		"repo_url", project.RepoURL,
	)
	if err := s.db.WithContext(ctx).Create(project).Error; err != nil {
		return fmt.Errorf("create project %q: %w", project.Name, err)
	}
	return nil
}

// UpdateProject 更新 Project 欄位。
func (s *ProjectService) UpdateProject(ctx context.Context, id uint, updates map[string]any) (*models.Project, error) {
	var project models.Project
	if err := s.db.WithContext(ctx).First(&project, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("project %d not found: %w", id, err)
		}
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}

	logger.Info("updating project", "id", id)
	if err := s.db.WithContext(ctx).Model(&project).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update project %d: %w", id, err)
	}
	return &project, nil
}

// DeleteProject 硬刪除 Project。
func (s *ProjectService) DeleteProject(ctx context.Context, id uint) error {
	logger.Info("deleting project (hard)", "id", id)
	if err := s.db.WithContext(ctx).Unscoped().Delete(&models.Project{}, id).Error; err != nil {
		return fmt.Errorf("delete project %d: %w", id, err)
	}
	return nil
}
