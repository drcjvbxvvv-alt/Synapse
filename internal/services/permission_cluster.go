package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
	"github.com/shaia/Synapse/pkg/logger"
)

// ========== 叢集權限管理 ==========

// CreateClusterPermissionRequest 建立叢集權限請求
type CreateClusterPermissionRequest struct {
	ClusterID      uint     `json:"cluster_id" binding:"required"`
	UserID         *uint    `json:"user_id"`
	UserGroupID    *uint    `json:"user_group_id"`
	PermissionType string   `json:"permission_type" binding:"required"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
}

// UpdateClusterPermissionRequest 更新叢集權限請求
type UpdateClusterPermissionRequest struct {
	PermissionType string   `json:"permission_type"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
}

// CreateClusterPermission 建立叢集權限
func (s *PermissionService) CreateClusterPermission(req *CreateClusterPermissionRequest) (*models.ClusterPermission, error) {
	ctx := context.Background()

	// 驗證參數
	if req.ClusterID == 0 {
		return nil, apierrors.ErrBadRequest("叢集ID不能為空")
	}
	if req.UserID == nil && req.UserGroupID == nil {
		return nil, apierrors.ErrBadRequest("必須指定使用者或使用者組")
	}
	if req.UserID != nil && req.UserGroupID != nil {
		return nil, apierrors.ErrPermissionAmbiguousTarget()
	}

	// 驗證權限型別
	validTypes := map[string]bool{
		models.PermissionTypeAdmin:    true,
		models.PermissionTypeOps:      true,
		models.PermissionTypeDev:      true,
		models.PermissionTypeReadonly: true,
		models.PermissionTypeCustom:   true,
	}
	if !validTypes[req.PermissionType] {
		return nil, apierrors.ErrPermissionInvalidType()
	}

	// 自定義權限必須指定角色
	if req.PermissionType == models.PermissionTypeCustom && req.CustomRoleRef == "" {
		return nil, apierrors.ErrPermissionCustomRoleRequired()
	}

	// 檢查是否已存在相同的權限配置
	duplicate, err := s.checkDuplicatePermission(ctx, req.ClusterID, req.UserID, req.UserGroupID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, apierrors.ErrPermissionDuplicate()
	}

	// 處理命名空間
	namespaces := req.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{"*"}
	}
	namespacesJSON, _ := json.Marshal(namespaces)

	permission := &models.ClusterPermission{
		ClusterID:      req.ClusterID,
		UserID:         req.UserID,
		UserGroupID:    req.UserGroupID,
		PermissionType: req.PermissionType,
		Namespaces:     string(namespacesJSON),
		CustomRoleRef:  req.CustomRoleRef,
	}

	if s.useRepo() {
		if err := s.repo.CreatePermission(ctx, permission); err != nil {
			if errors.Is(err, repositories.ErrAlreadyExists) {
				return nil, apierrors.ErrPermissionDuplicate()
			}
			return nil, fmt.Errorf("建立權限配置失敗: %w", err)
		}
	} else if err := s.db.WithContext(ctx).Create(permission).Error; err != nil {
		return nil, fmt.Errorf("建立權限配置失敗: %w", err)
	}

	// 預載入關聯資料（兩路徑都用 repo 或 db 直接取 relations）
	if s.useRepo() {
		if loaded, err := s.repo.GetWithRelations(ctx, permission.ID); err == nil {
			permission = loaded
		}
	} else {
		s.db.WithContext(ctx).Preload("User").Preload("UserGroup").Preload("Cluster").First(permission, permission.ID)
	}

	logger.Info("建立叢集權限: clusterID=%d, userID=%v, userGroupID=%v, type=%s",
		req.ClusterID, req.UserID, req.UserGroupID, req.PermissionType)

	return permission, nil
}

// checkDuplicatePermission is the shared dedup check used by CreateClusterPermission,
// keeping the dual-path branching in one place so the main method stays readable.
func (s *PermissionService) checkDuplicatePermission(
	ctx context.Context, clusterID uint, userID, groupID *uint,
) (bool, error) {
	if s.useRepo() {
		if userID != nil {
			return s.repo.ExistsForClusterUser(ctx, clusterID, *userID)
		}
		return s.repo.ExistsForClusterGroup(ctx, clusterID, *groupID)
	}

	query := s.db.Model(&models.ClusterPermission{}).Where("cluster_id = ?", clusterID)
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	} else {
		query = query.Where("user_group_id = ?", *groupID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("檢查權限配置失敗: %w", err)
	}
	return count > 0, nil
}

// UpdateClusterPermission 更新叢集權限
func (s *PermissionService) UpdateClusterPermission(id uint, req *UpdateClusterPermissionRequest) (*models.ClusterPermission, error) {
	ctx := context.Background()

	var permission *models.ClusterPermission
	if s.useRepo() {
		p, err := s.repo.Get(ctx, id)
		if err != nil {
			return nil, apierrors.ErrPermissionNotFound()
		}
		permission = p
	} else {
		var p models.ClusterPermission
		if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
			return nil, apierrors.ErrPermissionNotFound()
		}
		permission = &p
	}

	// 驗證權限型別
	if req.PermissionType != "" {
		validTypes := map[string]bool{
			models.PermissionTypeAdmin:    true,
			models.PermissionTypeOps:      true,
			models.PermissionTypeDev:      true,
			models.PermissionTypeReadonly: true,
			models.PermissionTypeCustom:   true,
		}
		if !validTypes[req.PermissionType] {
			return nil, apierrors.ErrPermissionInvalidType()
		}
		// 切換角色時清空舊的 feature_policy，避免舊設定污染新角色的預設狀態
		if req.PermissionType != permission.PermissionType {
			permission.FeaturePolicy = ""
		}
		permission.PermissionType = req.PermissionType
	}

	// 自定義權限必須指定角色
	if permission.PermissionType == models.PermissionTypeCustom {
		if req.CustomRoleRef != "" {
			permission.CustomRoleRef = req.CustomRoleRef
		} else if permission.CustomRoleRef == "" {
			return nil, apierrors.ErrPermissionCustomRoleRequired()
		}
	}

	// 更新命名空間
	if len(req.Namespaces) > 0 {
		namespacesJSON, _ := json.Marshal(req.Namespaces)
		permission.Namespaces = string(namespacesJSON)
	}

	if s.useRepo() {
		if err := s.repo.Update(ctx, permission); err != nil {
			return nil, fmt.Errorf("更新權限配置失敗: %w", err)
		}
		if loaded, err := s.repo.GetWithRelations(ctx, permission.ID); err == nil {
			permission = loaded
		}
		return permission, nil
	}

	if err := s.db.WithContext(ctx).Save(permission).Error; err != nil {
		return nil, fmt.Errorf("更新權限配置失敗: %w", err)
	}

	// 預載入關聯資料
	s.db.WithContext(ctx).Preload("User").Preload("UserGroup").Preload("Cluster").First(permission, permission.ID)

	return permission, nil
}

// DeleteClusterPermission 刪除叢集權限
func (s *PermissionService) DeleteClusterPermission(id uint) error {
	ctx := context.Background()
	if s.useRepo() {
		// Use DeleteWhere to get the affected rowcount so we can return
		// "not found" consistently with the legacy path.
		affected, err := s.repo.DeleteWhere(ctx, "id = ?", id)
		if err != nil {
			return fmt.Errorf("刪除權限配置失敗: %w", err)
		}
		if affected == 0 {
			return apierrors.ErrPermissionNotFound()
		}
		return nil
	}

	result := s.db.Delete(&models.ClusterPermission{}, id)
	if result.Error != nil {
		return fmt.Errorf("刪除權限配置失敗: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apierrors.ErrPermissionNotFound()
	}
	return nil
}

// GetClusterPermission 獲取叢集權限詳情
func (s *PermissionService) GetClusterPermission(id uint) (*models.ClusterPermission, error) {
	ctx := context.Background()
	if s.useRepo() {
		p, err := s.repo.GetWithRelations(ctx, id)
		if err != nil {
			return nil, apierrors.ErrPermissionNotFound()
		}
		return p, nil
	}

	var permission models.ClusterPermission
	if err := s.db.Preload("User").Preload("UserGroup").Preload("Cluster").First(&permission, id).Error; err != nil {
		return nil, apierrors.ErrPermissionNotFound()
	}
	return &permission, nil
}

// ListClusterPermissions 獲取叢集的權限列表
func (s *PermissionService) ListClusterPermissions(clusterID uint) ([]models.ClusterPermission, error) {
	ctx := context.Background()
	if s.useRepo() {
		ptrs, err := s.repo.ListByCluster(ctx, clusterID)
		if err != nil {
			return nil, fmt.Errorf("獲取權限列表失敗: %w", err)
		}
		permissions := make([]models.ClusterPermission, len(ptrs))
		for i, p := range ptrs {
			permissions[i] = *p
		}
		return permissions, nil
	}

	var permissions []models.ClusterPermission
	query := s.db.WithContext(ctx).Preload("User").Preload("UserGroup")
	if clusterID > 0 {
		query = query.Where("cluster_id = ?", clusterID)
	}
	if err := query.Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("獲取權限列表失敗: %w", err)
	}
	return permissions, nil
}

// ListAllClusterPermissions 獲取所有叢集的權限列表
func (s *PermissionService) ListAllClusterPermissions() ([]models.ClusterPermission, error) {
	ctx := context.Background()
	if s.useRepo() {
		ptrs, err := s.repo.ListAllWithRelations(ctx)
		if err != nil {
			return nil, fmt.Errorf("獲取權限列表失敗: %w", err)
		}
		permissions := make([]models.ClusterPermission, len(ptrs))
		for i, p := range ptrs {
			permissions[i] = *p
		}
		return permissions, nil
	}

	var permissions []models.ClusterPermission
	if err := s.db.WithContext(ctx).Preload("User").Preload("UserGroup").Preload("Cluster").Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("獲取權限列表失敗: %w", err)
	}
	return permissions, nil
}
