package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// GitOpsService — GitOps Application CRUD + 邊界管理（CICD_ARCHITECTURE §12, P2-5/P2-6）
//
// 設計原則：
//   - GitOpsApp CRUD，支援 native / argocd 兩種來源
//   - 互斥規則：同一 Application 不可同時被兩種來源管理
//   - Native source 需提供 Git Provider 連線資訊
//   - ArgoCD source 為代理模式，不需 Git 資訊
// ---------------------------------------------------------------------------

// GitOpsService 管理 GitOps 應用。
type GitOpsService struct {
	db *gorm.DB
}

// NewGitOpsService 建立 GitOpsService。
func NewGitOpsService(db *gorm.DB) *GitOpsService {
	return &GitOpsService{db: db}
}

// CreateApp 建立新的 GitOps 應用。
func (s *GitOpsService) CreateApp(ctx context.Context, app *models.GitOpsApp) error {
	if err := validateGitOpsApp(app); err != nil {
		return err
	}

	// 互斥規則：檢查同名 + 同 cluster 是否已存在不同 source 的 App
	if err := s.checkSourceExclusion(ctx, app); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Create(app).Error; err != nil {
		return fmt.Errorf("create gitops app: %w", err)
	}

	logger.Info("gitops app created",
		"app_id", app.ID,
		"name", app.Name,
		"source", app.Source,
		"cluster_id", app.ClusterID,
	)
	return nil
}

// GetApp 取得單一 GitOps 應用。
func (s *GitOpsService) GetApp(ctx context.Context, id uint) (*models.GitOpsApp, error) {
	var app models.GitOpsApp
	if err := s.db.WithContext(ctx).First(&app, id).Error; err != nil {
		return nil, fmt.Errorf("get gitops app %d: %w", id, err)
	}
	return &app, nil
}

// ListApps 列出指定叢集的所有 GitOps 應用（支援 source 過濾）。
func (s *GitOpsService) ListApps(ctx context.Context, clusterID uint, source string) ([]models.GitOpsApp, error) {
	query := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID)
	if source != "" {
		query = query.Where("source = ?", source)
	}

	var apps []models.GitOpsApp
	if err := query.Order("name ASC").Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("list gitops apps: %w", err)
	}
	return apps, nil
}

// ListAllApps 列出所有 GitOps 應用（合併列表，前端 §12.1 需求）。
func (s *GitOpsService) ListAllApps(ctx context.Context) ([]models.GitOpsApp, error) {
	var apps []models.GitOpsApp
	if err := s.db.WithContext(ctx).
		Order("source ASC, name ASC").
		Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("list all gitops apps: %w", err)
	}
	return apps, nil
}

// UpdateApp 更新 GitOps 應用。
func (s *GitOpsService) UpdateApp(ctx context.Context, id uint, updates map[string]interface{}) error {
	// 不允許修改 source（防止繞過互斥規則）
	if _, ok := updates["source"]; ok {
		return fmt.Errorf("cannot change gitops app source after creation")
	}

	result := s.db.WithContext(ctx).Model(&models.GitOpsApp{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update gitops app %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gitops app %d not found", id)
	}

	logger.Info("gitops app updated", "app_id", id)
	return nil
}

// DeleteApp 刪除 GitOps 應用（soft delete）。
func (s *GitOpsService) DeleteApp(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.GitOpsApp{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete gitops app %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("gitops app %d not found", id)
	}

	logger.Info("gitops app deleted", "app_id", id)
	return nil
}

// UpdateSyncStatus 更新 GitOps 應用的同步狀態。
func (s *GitOpsService) UpdateSyncStatus(ctx context.Context, id uint, status, statusMessage string) error {
	updates := map[string]interface{}{
		"status":         status,
		"status_message": statusMessage,
	}
	result := s.db.WithContext(ctx).Model(&models.GitOpsApp{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update gitops sync status %d: %w", id, result.Error)
	}
	return nil
}

// GetAppsForSync 取得需要同步的原生 GitOps 應用（sync_policy = auto）。
func (s *GitOpsService) GetAppsForSync(ctx context.Context) ([]models.GitOpsApp, error) {
	var apps []models.GitOpsApp
	if err := s.db.WithContext(ctx).
		Where("source = ? AND sync_policy = ?", models.GitOpsSourceNative, models.GitOpsSyncPolicyAuto).
		Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("get apps for sync: %w", err)
	}
	return apps, nil
}

// ---------------------------------------------------------------------------
// §12.1 互斥規則 — ArgoCD / Native GitOps 邊界（P2-6）
// ---------------------------------------------------------------------------

// checkSourceExclusion 檢查互斥規則：同一 cluster + namespace 不可有不同 source 的同名 App。
func (s *GitOpsService) checkSourceExclusion(ctx context.Context, app *models.GitOpsApp) error {
	var existing models.GitOpsApp
	err := s.db.WithContext(ctx).
		Where("name = ? AND cluster_id = ? AND source != ?", app.Name, app.ClusterID, app.Source).
		First(&existing).Error
	if err == nil {
		return fmt.Errorf(
			"gitops app %q on cluster %d already managed by %q source (互斥規則: §12.1)",
			app.Name, app.ClusterID, existing.Source,
		)
	}
	return nil // not found = no conflict
}

// ValidateDeployStepExclusion 驗證 Pipeline 中 deploy-argocd-sync 和 gitops-sync
// 不可對同一 App 同時使用（§12.1 互斥規則）。
func ValidateDeployStepExclusion(steps []StepDeployTarget) []string {
	argoApps := make(map[string]bool)
	gitopsApps := make(map[string]bool)

	for _, step := range steps {
		key := fmt.Sprintf("%s/%s", step.Namespace, step.AppName)
		switch step.StepType {
		case "deploy-argocd-sync":
			argoApps[key] = true
		case "gitops-sync":
			gitopsApps[key] = true
		}
	}

	var errors []string
	for app := range argoApps {
		if gitopsApps[app] {
			errors = append(errors, fmt.Sprintf(
				"app %q cannot use both deploy-argocd-sync and gitops-sync in the same pipeline (§12.1)",
				app,
			))
		}
	}
	return errors
}

// StepDeployTarget 代表 Pipeline 中 deploy Step 的目標。
type StepDeployTarget struct {
	StepType  string
	AppName   string
	Namespace string
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func validateGitOpsApp(app *models.GitOpsApp) error {
	if app.Name == "" {
		return fmt.Errorf("gitops app name is required")
	}
	if app.ClusterID == 0 {
		return fmt.Errorf("gitops app cluster_id is required")
	}
	if app.Namespace == "" {
		return fmt.Errorf("gitops app namespace is required")
	}

	// Source validation
	if err := validateGitOpsSource(app.Source); err != nil {
		return err
	}

	// Render type validation
	if err := validateGitOpsRenderType(app.RenderType); err != nil {
		return err
	}

	// Sync policy validation
	if err := validateGitOpsSyncPolicy(app.SyncPolicy); err != nil {
		return err
	}

	// Native source requires Git info
	if app.Source == models.GitOpsSourceNative {
		if app.RepoURL == "" {
			return fmt.Errorf("native gitops app requires repo_url")
		}
		if app.Branch == "" {
			return fmt.Errorf("native gitops app requires branch")
		}
	}

	// Sync interval bounds
	if app.SyncInterval < 30 {
		return fmt.Errorf("sync_interval must be at least 30 seconds, got %d", app.SyncInterval)
	}
	if app.SyncInterval > 86400 {
		return fmt.Errorf("sync_interval must be at most 86400 seconds (24h), got %d", app.SyncInterval)
	}

	return nil
}

func validateGitOpsSource(source string) error {
	valid := map[string]bool{
		models.GitOpsSourceNative: true,
		models.GitOpsSourceArgoCD: true,
	}
	if !valid[source] {
		return fmt.Errorf("invalid gitops source %q, must be native|argocd", source)
	}
	return nil
}

func validateGitOpsRenderType(renderType string) error {
	valid := map[string]bool{
		models.GitOpsRenderRaw:       true,
		models.GitOpsRenderKustomize: true,
		models.GitOpsRenderHelm:      true,
	}
	if !valid[renderType] {
		return fmt.Errorf("invalid render_type %q, must be raw|kustomize|helm", renderType)
	}
	return nil
}

func validateGitOpsSyncPolicy(policy string) error {
	valid := map[string]bool{
		models.GitOpsSyncPolicyAuto:   true,
		models.GitOpsSyncPolicyManual: true,
	}
	if !valid[policy] {
		return fmt.Errorf("invalid sync_policy %q, must be auto|manual", policy)
	}
	return nil
}
