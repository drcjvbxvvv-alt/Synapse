package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/features"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
	"github.com/shaia/Synapse/pkg/logger"
)

// PermissionService 權限服務
//
// P0-4b migration status: dual-path. When features.FlagRepositoryLayer is
// enabled the service routes reads/writes through the Repository layer;
// when disabled it falls back to the legacy *gorm.DB path so the flag can
// be flipped off if a production regression shows up.
//
// Note: all method signatures intentionally stay ctx-less. PermissionService
// is called from 30+ handler sites plus the auth middleware on every
// request; pushing ctx through every caller would explode P0-4b scope.
// Internally the repo calls run with a background context — request-scoped
// tracing is deferred to P0-4c together with ClusterService.GetCluster.
type PermissionService struct {
	db   *gorm.DB
	repo repositories.PermissionRepository
}

// NewPermissionService 建立權限服務
func NewPermissionService(db *gorm.DB, repo repositories.PermissionRepository) *PermissionService {
	return &PermissionService{db: db, repo: repo}
}

// useRepo reports whether the service should dispatch to the Repository layer.
func (s *PermissionService) useRepo() bool {
	return s.repo != nil && features.IsEnabled(features.FlagRepositoryLayer)
}

// ========== 使用者組管理 ==========

// CreateUserGroup 建立使用者組
func (s *PermissionService) CreateUserGroup(name, description string) (*models.UserGroup, error) {
	ctx := context.Background()
	group := &models.UserGroup{
		Name:        name,
		Description: description,
	}

	if s.useRepo() {
		if err := s.repo.CreateUserGroup(ctx, group); err != nil {
			if errors.Is(err, repositories.ErrAlreadyExists) {
				return nil, apierrors.ErrGroupDuplicateName()
			}
			return nil, fmt.Errorf("建立使用者組失敗: %w", err)
		}
		return group, nil
	}

	if err := s.db.Create(group).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, apierrors.ErrGroupDuplicateName()
		}
		return nil, fmt.Errorf("建立使用者組失敗: %w", err)
	}
	return group, nil
}

// UpdateUserGroup 更新使用者組
func (s *PermissionService) UpdateUserGroup(id uint, name, description string) (*models.UserGroup, error) {
	ctx := context.Background()
	if s.useRepo() {
		group, err := s.repo.GetUserGroup(ctx, id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) ||
				errors.Is(err, repositories.ErrInvalidArgument) {
				return nil, apierrors.ErrGroupNotFound()
			}
			return nil, fmt.Errorf("獲取使用者組失敗: %w", err)
		}
		group.Name = name
		group.Description = description
		if err := s.repo.UpdateUserGroup(ctx, group); err != nil {
			return nil, fmt.Errorf("更新使用者組失敗: %w", err)
		}
		return group, nil
	}

	var group models.UserGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return nil, apierrors.ErrGroupNotFound()
	}

	group.Name = name
	group.Description = description
	if err := s.db.Save(&group).Error; err != nil {
		return nil, fmt.Errorf("更新使用者組失敗: %w", err)
	}
	return &group, nil
}

// DeleteUserGroup 刪除使用者組
func (s *PermissionService) DeleteUserGroup(id uint) error {
	ctx := context.Background()
	if s.useRepo() {
		// 前置檢查：有關聯權限配置時拒絕刪除
		count, err := s.repo.CountPermissionsForGroup(ctx, id)
		if err != nil {
			return fmt.Errorf("查詢使用者組權限失敗: %w", err)
		}
		if count > 0 {
			return apierrors.ErrGroupHasPermissions()
		}
		if err := s.repo.DeleteUserGroupTx(ctx, id); err != nil {
			return fmt.Errorf("刪除使用者組失敗: %w", err)
		}
		return nil
	}

	// 前置檢查：有關聯權限配置時拒絕刪除（在事務外檢查即可，失敗只是拒絕，不涉及資料修改）
	var count int64
	if err := s.db.Model(&models.ClusterPermission{}).Where("user_group_id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("查詢使用者組權限失敗: %w", err)
	}
	if count > 0 {
		return apierrors.ErrGroupHasPermissions()
	}

	// 使用事務確保成員關聯與使用者組本體同時刪除，失敗時自動回滾
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_group_id = ?", id).Delete(&models.UserGroupMember{}).Error; err != nil {
			return fmt.Errorf("刪除使用者組成員關聯失敗: %w", err)
		}
		if err := tx.Delete(&models.UserGroup{}, id).Error; err != nil {
			return fmt.Errorf("刪除使用者組失敗: %w", err)
		}
		return nil
	})
}

// GetUserGroup 獲取使用者組詳情
func (s *PermissionService) GetUserGroup(id uint) (*models.UserGroup, error) {
	ctx := context.Background()
	if s.useRepo() {
		group, err := s.repo.GetUserGroupWithUsers(ctx, id)
		if err != nil {
			return nil, apierrors.ErrGroupNotFound()
		}
		return group, nil
	}

	var group models.UserGroup
	if err := s.db.Preload("Users").First(&group, id).Error; err != nil {
		return nil, apierrors.ErrGroupNotFound()
	}
	return &group, nil
}

// ListUserGroups 獲取使用者組列表（只 Preload 使用者基本欄位，避免拉取多餘資料）
func (s *PermissionService) ListUserGroups() ([]models.UserGroup, error) {
	ctx := context.Background()
	if s.useRepo() {
		ptrs, err := s.repo.ListUserGroupsWithUsers(ctx)
		if err != nil {
			return nil, fmt.Errorf("獲取使用者組列表失敗: %w", err)
		}
		groups := make([]models.UserGroup, len(ptrs))
		for i, g := range ptrs {
			groups[i] = *g
		}
		return groups, nil
	}

	var groups []models.UserGroup
	if err := s.db.Preload("Users", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, email, display_name")
	}).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("獲取使用者組列表失敗: %w", err)
	}
	return groups, nil
}

// AddUserToGroup 新增使用者到使用者組
func (s *PermissionService) AddUserToGroup(userID, groupID uint) error {
	ctx := context.Background()

	// 檢查使用者是否存在（兩路徑共用的預檢）
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return apierrors.ErrUserNotFound()
	}

	if s.useRepo() {
		if _, err := s.repo.GetUserGroup(ctx, groupID); err != nil {
			return apierrors.ErrGroupNotFound()
		}
		return s.repo.AddUserToGroup(ctx, userID, groupID)
	}

	// 檢查使用者組是否存在
	var group models.UserGroup
	if err := s.db.First(&group, groupID).Error; err != nil {
		return apierrors.ErrGroupNotFound()
	}

	// 檢查是否已在組中
	var count int64
	s.db.Model(&models.UserGroupMember{}).Where("user_id = ? AND user_group_id = ?", userID, groupID).Count(&count)
	if count > 0 {
		return nil // 已存在，跳過
	}

	// 新增關聯
	member := &models.UserGroupMember{
		UserID:      userID,
		UserGroupID: groupID,
	}
	return s.db.Create(member).Error
}

// RemoveUserFromGroup 從使用者組移除使用者
func (s *PermissionService) RemoveUserFromGroup(userID, groupID uint) error {
	ctx := context.Background()
	if s.useRepo() {
		return s.repo.RemoveUserFromGroup(ctx, userID, groupID)
	}
	return s.db.Where("user_id = ? AND user_group_id = ?", userID, groupID).Delete(&models.UserGroupMember{}).Error
}

// ========== 叢集權限管理 ==========

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
	} else if err := s.db.Create(permission).Error; err != nil {
		return nil, fmt.Errorf("建立權限配置失敗: %w", err)
	}

	// 預載入關聯資料（兩路徑都用 repo 或 db 直接取 relations）
	if s.useRepo() {
		if loaded, err := s.repo.GetWithRelations(ctx, permission.ID); err == nil {
			permission = loaded
		}
	} else {
		s.db.Preload("User").Preload("UserGroup").Preload("Cluster").First(permission, permission.ID)
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

// CreateClusterPermissionRequest 建立叢集權限請求
type CreateClusterPermissionRequest struct {
	ClusterID      uint     `json:"cluster_id" binding:"required"`
	UserID         *uint    `json:"user_id"`
	UserGroupID    *uint    `json:"user_group_id"`
	PermissionType string   `json:"permission_type" binding:"required"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
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
		if err := s.db.First(&p, id).Error; err != nil {
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

	if err := s.db.Save(permission).Error; err != nil {
		return nil, fmt.Errorf("更新權限配置失敗: %w", err)
	}

	// 預載入關聯資料
	s.db.Preload("User").Preload("UserGroup").Preload("Cluster").First(permission, permission.ID)

	return permission, nil
}

// UpdateClusterPermissionRequest 更新叢集權限請求
type UpdateClusterPermissionRequest struct {
	PermissionType string   `json:"permission_type"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
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
	query := s.db.Preload("User").Preload("UserGroup")
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
	if err := s.db.Preload("User").Preload("UserGroup").Preload("Cluster").Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("獲取權限列表失敗: %w", err)
	}
	return permissions, nil
}

// ========== 權限查詢 ==========

// GetUserClusterPermission 獲取使用者在指定叢集的權限
// 權限優先順序：使用者直接權限 > 使用者組權限 > 預設權限
func (s *PermissionService) GetUserClusterPermission(userID, clusterID uint) (*models.ClusterPermission, error) {
	ctx := context.Background()
	if s.useRepo() {
		// 1. 直接權限
		if direct, err := s.repo.FindByClusterUser(ctx, clusterID, userID); err == nil {
			return direct, nil
		} else if !errors.Is(err, repositories.ErrNotFound) {
			return nil, fmt.Errorf("查詢使用者直接權限失敗: %w", err)
		}

		// 2. 使用者組權限
		groupIDs, err := s.repo.ListGroupIDsForUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("查詢使用者組失敗: %w", err)
		}
		if len(groupIDs) > 0 {
			if gp, err := s.repo.FindByClusterGroups(ctx, clusterID, groupIDs); err == nil {
				return gp, nil
			} else if !errors.Is(err, repositories.ErrNotFound) {
				return nil, fmt.Errorf("查詢使用者組權限失敗: %w", err)
			}
		}

		// 3. 預設權限
		return s.getDefaultPermission(userID, clusterID)
	}

	// 1. 先查詢使用者直接權限
	var directPermission models.ClusterPermission
	err := s.db.Where("cluster_id = ? AND user_id = ?", clusterID, userID).First(&directPermission).Error
	if err == nil {
		return &directPermission, nil
	}

	// 2. 查詢使用者組權限
	var userGroups []models.UserGroupMember
	s.db.Where("user_id = ?", userID).Find(&userGroups)

	if len(userGroups) > 0 {
		groupIDs := make([]uint, len(userGroups))
		for i, ug := range userGroups {
			groupIDs[i] = ug.UserGroupID
		}

		var groupPermission models.ClusterPermission
		err = s.db.Where("cluster_id = ? AND user_group_id IN ?", clusterID, groupIDs).
			Order("FIELD(permission_type, 'admin', 'ops', 'dev', 'readonly', 'custom')"). // 優先返回權限最大的
			First(&groupPermission).Error
		if err == nil {
			return &groupPermission, nil
		}
	}

	// 3. 返回預設權限
	return s.getDefaultPermission(userID, clusterID)
}

// getDefaultPermission 獲取使用者的預設權限
// 平台管理員預設為管理員權限，其他使用者預設為只讀權限
func (s *PermissionService) getDefaultPermission(userID, clusterID uint) (*models.ClusterPermission, error) {
	// 查詢使用者資訊
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}

	// 確定預設權限型別
	permissionType := models.PermissionTypeReadonly // 預設只讀
	if user.IsPlatformAdmin() {
		permissionType = models.PermissionTypeAdmin // 平台管理員預設擁有所有叢集 admin 權限
	}

	// 返回虛擬權限物件（不儲存到資料庫，僅用於權限檢查）
	defaultPermission := &models.ClusterPermission{
		ClusterID:      clusterID,
		UserID:         &userID,
		PermissionType: permissionType,
		Namespaces:     `["*"]`, // 預設全部命名空間
	}

	logger.Debug("使用預設權限", "user_id", userID, "cluster_id", clusterID, "type", permissionType)

	return defaultPermission, nil
}

// GetUserAllClusterPermissions 獲取使用者在所有叢集的權限（包括預設權限）
func (s *PermissionService) GetUserAllClusterPermissions(userID uint) ([]models.ClusterPermission, error) {
	ctx := context.Background()

	var permissions []models.ClusterPermission
	if s.useRepo() {
		groupIDs, err := s.repo.ListGroupIDsForUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("查詢使用者組失敗: %w", err)
		}
		ptrs, err := s.repo.ListUserDirectAndGroupWithCluster(ctx, userID, groupIDs)
		if err != nil {
			return nil, fmt.Errorf("獲取使用者權限失敗: %w", err)
		}
		permissions = make([]models.ClusterPermission, len(ptrs))
		for i, p := range ptrs {
			permissions[i] = *p
		}
	} else {
		// P2-11: 使用 GORM subquery 將「查 userGroups」與「查 permissions」合併為單一查詢，
		// 避免原本 4 條 DB round-trip 的 N+1 問題。
		groupSubquery := s.db.WithContext(ctx).
			Model(&models.UserGroupMember{}).
			Where("user_id = ?", userID).
			Select("user_group_id")

		if err := s.db.WithContext(ctx).
			Preload("Cluster").
			Where("user_id = ? OR user_group_id IN (?)", userID, groupSubquery).
			Find(&permissions).Error; err != nil {
			return nil, fmt.Errorf("獲取使用者權限失敗: %w", err)
		}
	}

	// 獲取已配置權限的叢集ID
	configuredClusterIDs := make(map[uint]bool)
	for _, p := range permissions {
		configuredClusterIDs[p.ClusterID] = true
	}

	// 獲取所有叢集，為未配置權限的叢集新增預設權限。
	// Select 僅取必要欄位以減少資料傳輸（P2-11）。
	var allClusters []models.Cluster
	if err := s.db.WithContext(ctx).
		Select("id, name, version, status, created_at, updated_at").
		Find(&allClusters).Error; err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	// 查詢使用者資訊（用於確定預設權限型別）
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("使用者不存在: %w", err)
	}

	// 確定預設權限型別
	defaultPermissionType := models.PermissionTypeReadonly
	if user.IsPlatformAdmin() {
		defaultPermissionType = models.PermissionTypeAdmin
	}

	// 為未配置權限的叢集新增預設權限
	for _, cluster := range allClusters {
		if !configuredClusterIDs[cluster.ID] {
			defaultPerm := models.ClusterPermission{
				ClusterID:      cluster.ID,
				UserID:         &userID,
				PermissionType: defaultPermissionType,
				Namespaces:     `["*"]`,
				Cluster:        &cluster,
			}
			permissions = append(permissions, defaultPerm)
		}
	}

	return permissions, nil
}

// UpdateFeaturePolicy 更新指定叢集權限記錄的功能開關策略。
// Keys that exceed the permission type's ceiling are silently dropped.
func (s *PermissionService) UpdateFeaturePolicy(id uint, policy map[string]bool) (*models.ClusterPermission, error) {
	permission, err := s.GetClusterPermission(id)
	if err != nil {
		return nil, err
	}

	// Build ceiling set for fast lookup.
	ceiling := models.FeatureCeilings[permission.PermissionType]
	inCeiling := make(map[string]struct{}, len(ceiling))
	for _, key := range ceiling {
		inCeiling[key] = struct{}{}
	}

	// Drop keys that exceed the ceiling.
	filtered := make(map[string]bool, len(policy))
	for k, v := range policy {
		if _, ok := inCeiling[k]; ok {
			filtered[k] = v
		}
	}

	if err := permission.SetFeaturePolicyMap(filtered); err != nil {
		return nil, fmt.Errorf("encode feature policy: %w", err)
	}

	ctx := context.Background()
	if err := s.db.WithContext(ctx).
		Model(permission).
		Update("feature_policy", permission.FeaturePolicy).Error; err != nil {
		return nil, fmt.Errorf("update feature policy: %w", err)
	}
	return permission, nil
}

// HasClusterAccess 檢查使用者是否有叢集訪問權限
func (s *PermissionService) HasClusterAccess(userID, clusterID uint) bool {
	_, err := s.GetUserClusterPermission(userID, clusterID)
	return err == nil
}

// CanPerformUserAction 檢查使用者是否可以執行指定操作
func (s *PermissionService) CanPerformUserAction(userID, clusterID uint, action string, namespace string) bool {
	permission, err := s.GetUserClusterPermission(userID, clusterID)
	if err != nil {
		return false
	}

	if namespace != "" && !HasNamespaceAccess(permission, namespace) {
		return false
	}

	return CanPerformAction(permission, action)
}

// HasNamespaceAccess 檢查權限是否包含指定命名空間的訪問
func HasNamespaceAccess(cp *models.ClusterPermission, namespace string) bool {
	namespaces := cp.GetNamespaceList()
	for _, ns := range namespaces {
		if ns == "*" || ns == namespace {
			return true
		}
		if len(ns) > 1 && ns[len(ns)-1] == '*' {
			prefix := ns[:len(ns)-1]
			if len(namespace) >= len(prefix) && namespace[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// HasAllNamespaceAccess 檢查是否有全部命名空間的訪問權限
func HasAllNamespaceAccess(cp *models.ClusterPermission) bool {
	namespaces := cp.GetNamespaceList()
	for _, ns := range namespaces {
		if ns == "*" {
			return true
		}
	}
	return false
}

// CanPerformAction 檢查權限型別是否允許執行指定操作
func CanPerformAction(cp *models.ClusterPermission, action string) bool {
	switch cp.PermissionType {
	case models.PermissionTypeAdmin:
		return true
	case models.PermissionTypeOps:
		restrictedActions := map[string]bool{
			"node:cordon": true, "node:uncordon": true, "node:drain": true,
			"pv:create": true, "pv:delete": true,
			"storageclass:create": true, "storageclass:delete": true,
			"quota:create": true, "quota:update": true, "quota:delete": true,
		}
		return !restrictedActions[action]
	case models.PermissionTypeDev:
		allowedPrefixes := []string{
			"pod:", "deployment:", "statefulset:", "daemonset:",
			"job:", "cronjob:", "service:", "ingress:",
			"configmap:", "secret:", "rollout:",
		}
		for _, prefix := range allowedPrefixes {
			if len(action) >= len(prefix) && action[:len(prefix)] == prefix {
				return true
			}
		}
		return false
	case models.PermissionTypeReadonly:
		return action == "view" || action == "list" || action == "get"
	case models.PermissionTypeCustom:
		return true
	default:
		return false
	}
}

// ========== 使用者查詢 ==========
//
// NOTE: ListUsers / GetUser query the User aggregate root and logically belong
// to UserService / UserRepository, not PermissionRepository. They remain here
// on the legacy *gorm.DB path for historical reasons (callers in handlers have
// accumulated over time). Moving them is deferred to a later refactor; the
// Repository pattern pilot (P0-4b) keeps them unchanged on purpose.

// ListUsers 獲取使用者列表
func (s *PermissionService) ListUsers() ([]models.User, error) {
	var users []models.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("獲取使用者列表失敗: %w", err)
	}
	return users, nil
}

// GetUser 獲取使用者詳情
func (s *PermissionService) GetUser(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.Preload("Roles").First(&user, id).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}
	return &user, nil
}

// GetUserAccessibleClusterIDs 獲取使用者可訪問的叢集 ID 列表
// 返回值: clusterIDs, isAllAccess（平臺管理員）, error
//
// The User lookup (for IsPlatformAdmin) stays on s.db in both paths — it's a
// cross-aggregate query that doesn't belong in PermissionRepository.
func (s *PermissionService) GetUserAccessibleClusterIDs(userID uint) ([]uint, bool, error) {
	ctx := context.Background()

	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, false, apierrors.ErrUserNotFound()
	}

	// 平台管理員擁有全部叢集權限
	if user.IsPlatformAdmin() {
		return nil, true, nil
	}

	if s.useRepo() {
		// 使用者直接的 admin 權限
		adminCount, err := s.repo.CountAdminByUser(ctx, userID)
		if err != nil {
			return nil, false, fmt.Errorf("查詢使用者 admin 權限失敗: %w", err)
		}
		if adminCount > 0 {
			return nil, true, nil
		}

		// 使用者組
		groupIDs, err := s.repo.ListGroupIDsForUser(ctx, userID)
		if err != nil {
			return nil, false, fmt.Errorf("查詢使用者組失敗: %w", err)
		}

		// 使用者組的 admin 權限
		if len(groupIDs) > 0 {
			groupAdminCount, err := s.repo.CountAdminByGroups(ctx, groupIDs)
			if err != nil {
				return nil, false, fmt.Errorf("查詢使用者組 admin 權限失敗: %w", err)
			}
			if groupAdminCount > 0 {
				return nil, true, nil
			}
		}

		// 收集使用者有明確權限的叢集 ID
		clusterIDs, err := s.repo.DistinctClusterIDsByUser(ctx, userID, groupIDs)
		if err != nil {
			return nil, false, fmt.Errorf("查詢可訪問叢集失敗: %w", err)
		}

		// 沒有任何明確權限記錄的使用者，擁有所有叢集的預設只讀權限
		if len(clusterIDs) == 0 {
			return nil, true, nil
		}
		return clusterIDs, false, nil
	}

	// Legacy *gorm.DB path
	var adminCount int64
	s.db.Model(&models.ClusterPermission{}).
		Where("user_id = ? AND permission_type = ?", userID, models.PermissionTypeAdmin).
		Count(&adminCount)
	if adminCount > 0 {
		return nil, true, nil
	}

	var groupIDs []uint
	s.db.Model(&models.UserGroupMember{}).Where("user_id = ?", userID).Pluck("user_group_id", &groupIDs)

	if len(groupIDs) > 0 {
		s.db.Model(&models.ClusterPermission{}).
			Where("user_group_id IN ? AND permission_type = ?", groupIDs, models.PermissionTypeAdmin).
			Count(&adminCount)
		if adminCount > 0 {
			return nil, true, nil
		}
	}

	var clusterIDs []uint
	query := s.db.Model(&models.ClusterPermission{}).Where("user_id = ?", userID)
	if len(groupIDs) > 0 {
		query = s.db.Model(&models.ClusterPermission{}).Where("user_id = ? OR user_group_id IN ?", userID, groupIDs)
	}
	query.Distinct().Pluck("cluster_id", &clusterIDs)

	if len(clusterIDs) == 0 {
		return nil, true, nil
	}
	return clusterIDs, false, nil
}

// BatchDeleteClusterPermissions 批次刪除叢集權限
func (s *PermissionService) BatchDeleteClusterPermissions(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	ctx := context.Background()
	if s.useRepo() {
		if _, err := s.repo.BatchDeletePermissions(ctx, ids); err != nil {
			return fmt.Errorf("批次刪除權限配置失敗: %w", err)
		}
		return nil
	}

	result := s.db.Delete(&models.ClusterPermission{}, ids)
	if result.Error != nil {
		return fmt.Errorf("批次刪除權限配置失敗: %w", result.Error)
	}
	return nil
}
