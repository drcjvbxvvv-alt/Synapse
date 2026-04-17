package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
	"github.com/shaia/Synapse/pkg/logger"
)

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
	err := s.db.WithContext(ctx).Where("cluster_id = ? AND user_id = ?", clusterID, userID).First(&directPermission).Error
	if err == nil {
		return &directPermission, nil
	}

	// 2. 查詢使用者組權限
	var userGroups []models.UserGroupMember
	s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&userGroups)

	if len(userGroups) > 0 {
		groupIDs := make([]uint, len(userGroups))
		for i, ug := range userGroups {
			groupIDs[i] = ug.UserGroupID
		}

		var groupPermission models.ClusterPermission
		err = s.db.WithContext(ctx).Where("cluster_id = ? AND user_group_id IN ?", clusterID, groupIDs).
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
	ctx := context.Background()
	// 查詢使用者資訊
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
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
	ctx := context.Background()
	var users []models.User
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("獲取使用者列表失敗: %w", err)
	}
	return users, nil
}

// GetUser 獲取使用者詳情
func (s *PermissionService) GetUser(id uint) (*models.User, error) {
	ctx := context.Background()
	var user models.User
	if err := s.db.WithContext(ctx).Preload("Roles").First(&user, id).Error; err != nil {
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
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
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
	s.db.WithContext(ctx).Model(&models.ClusterPermission{}).
		Where("user_id = ? AND permission_type = ?", userID, models.PermissionTypeAdmin).
		Count(&adminCount)
	if adminCount > 0 {
		return nil, true, nil
	}

	var groupIDs []uint
	s.db.WithContext(ctx).Model(&models.UserGroupMember{}).Where("user_id = ?", userID).Pluck("user_group_id", &groupIDs)

	if len(groupIDs) > 0 {
		s.db.WithContext(ctx).Model(&models.ClusterPermission{}).
			Where("user_group_id IN ? AND permission_type = ?", groupIDs, models.PermissionTypeAdmin).
			Count(&adminCount)
		if adminCount > 0 {
			return nil, true, nil
		}
	}

	var clusterIDs []uint
	query := s.db.WithContext(ctx).Model(&models.ClusterPermission{}).Where("user_id = ?", userID)
	if len(groupIDs) > 0 {
		query = s.db.WithContext(ctx).Model(&models.ClusterPermission{}).Where("user_id = ? OR user_group_id IN ?", userID, groupIDs)
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

	result := s.db.WithContext(ctx).Delete(&models.ClusterPermission{}, ids)
	if result.Error != nil {
		return fmt.Errorf("批次刪除權限配置失敗: %w", result.Error)
	}
	return nil
}
