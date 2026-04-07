package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// PermissionService 權限服務
type PermissionService struct {
	db *gorm.DB
}

// NewPermissionService 建立權限服務
func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

// ========== 使用者組管理 ==========

// CreateUserGroup 建立使用者組
func (s *PermissionService) CreateUserGroup(name, description string) (*models.UserGroup, error) {
	group := &models.UserGroup{
		Name:        name,
		Description: description,
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
	// 檢查是否有關聯的權限配置
	var count int64
	s.db.Model(&models.ClusterPermission{}).Where("user_group_id = ?", id).Count(&count)
	if count > 0 {
		return apierrors.ErrGroupHasPermissions()
	}

	// 刪除使用者組成員關聯
	s.db.Where("user_group_id = ?", id).Delete(&models.UserGroupMember{})

	// 刪除使用者組
	if err := s.db.Delete(&models.UserGroup{}, id).Error; err != nil {
		return fmt.Errorf("刪除使用者組失敗: %w", err)
	}
	return nil
}

// GetUserGroup 獲取使用者組詳情
func (s *PermissionService) GetUserGroup(id uint) (*models.UserGroup, error) {
	var group models.UserGroup
	if err := s.db.Preload("Users").First(&group, id).Error; err != nil {
		return nil, apierrors.ErrGroupNotFound()
	}
	return &group, nil
}

// ListUserGroups 獲取使用者組列表
func (s *PermissionService) ListUserGroups() ([]models.UserGroup, error) {
	var groups []models.UserGroup
	if err := s.db.Preload("Users").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("獲取使用者組列表失敗: %w", err)
	}
	return groups, nil
}

// AddUserToGroup 新增使用者到使用者組
func (s *PermissionService) AddUserToGroup(userID, groupID uint) error {
	// 檢查使用者是否存在
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return apierrors.ErrUserNotFound()
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
	return s.db.Where("user_id = ? AND user_group_id = ?", userID, groupID).Delete(&models.UserGroupMember{}).Error
}

// ========== 叢集權限管理 ==========

// CreateClusterPermission 建立叢集權限
func (s *PermissionService) CreateClusterPermission(req *CreateClusterPermissionRequest) (*models.ClusterPermission, error) {
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
	query := s.db.Model(&models.ClusterPermission{}).Where("cluster_id = ?", req.ClusterID)
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	} else {
		query = query.Where("user_group_id = ?", *req.UserGroupID)
	}
	var count int64
	query.Count(&count)
	if count > 0 {
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

	if err := s.db.Create(permission).Error; err != nil {
		return nil, fmt.Errorf("建立權限配置失敗: %w", err)
	}

	// 預載入關聯資料
	s.db.Preload("User").Preload("UserGroup").Preload("Cluster").First(permission, permission.ID)

	logger.Info("建立叢集權限: clusterID=%d, userID=%v, userGroupID=%v, type=%s",
		req.ClusterID, req.UserID, req.UserGroupID, req.PermissionType)

	return permission, nil
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
	var permission models.ClusterPermission
	if err := s.db.First(&permission, id).Error; err != nil {
		return nil, apierrors.ErrPermissionNotFound()
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

	if err := s.db.Save(&permission).Error; err != nil {
		return nil, fmt.Errorf("更新權限配置失敗: %w", err)
	}

	// 預載入關聯資料
	s.db.Preload("User").Preload("UserGroup").Preload("Cluster").First(&permission, permission.ID)

	return &permission, nil
}

// UpdateClusterPermissionRequest 更新叢集權限請求
type UpdateClusterPermissionRequest struct {
	PermissionType string   `json:"permission_type"`
	Namespaces     []string `json:"namespaces"`
	CustomRoleRef  string   `json:"custom_role_ref"`
}

// DeleteClusterPermission 刪除叢集權限
func (s *PermissionService) DeleteClusterPermission(id uint) error {
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
	var permission models.ClusterPermission
	if err := s.db.Preload("User").Preload("UserGroup").Preload("Cluster").First(&permission, id).Error; err != nil {
		return nil, apierrors.ErrPermissionNotFound()
	}
	return &permission, nil
}

// ListClusterPermissions 獲取叢集的權限列表
func (s *PermissionService) ListClusterPermissions(clusterID uint) ([]models.ClusterPermission, error) {
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
// admin 使用者預設為管理員權限，其他使用者預設為只讀權限
func (s *PermissionService) getDefaultPermission(userID, clusterID uint) (*models.ClusterPermission, error) {
	// 查詢使用者資訊
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}

	// 確定預設權限型別
	permissionType := models.PermissionTypeReadonly // 預設只讀
	if user.Username == "admin" {
		permissionType = models.PermissionTypeAdmin // admin 使用者預設管理員權限
	}

	// 返回虛擬權限物件（不儲存到資料庫，僅用於權限檢查）
	defaultPermission := &models.ClusterPermission{
		ClusterID:      clusterID,
		UserID:         &userID,
		PermissionType: permissionType,
		Namespaces:     `["*"]`, // 預設全部命名空間
	}

	logger.Info("使用預設權限: userID=%d, clusterID=%d, type=%s", userID, clusterID, permissionType)

	return defaultPermission, nil
}

// GetUserAllClusterPermissions 獲取使用者在所有叢集的權限（包括預設權限）
func (s *PermissionService) GetUserAllClusterPermissions(userID uint) ([]models.ClusterPermission, error) {
	var permissions []models.ClusterPermission

	// 獲取使用者所在的使用者組
	var userGroups []models.UserGroupMember
	s.db.Where("user_id = ?", userID).Find(&userGroups)

	groupIDs := make([]uint, len(userGroups))
	for i, ug := range userGroups {
		groupIDs[i] = ug.UserGroupID
	}

	// 查詢使用者直接權限和使用者組權限
	query := s.db.Preload("Cluster").Where("user_id = ?", userID)
	if len(groupIDs) > 0 {
		query = s.db.Preload("Cluster").Where("user_id = ? OR user_group_id IN ?", userID, groupIDs)
	}

	if err := query.Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("獲取使用者權限失敗: %w", err)
	}

	// 獲取已配置權限的叢集ID
	configuredClusterIDs := make(map[uint]bool)
	for _, p := range permissions {
		configuredClusterIDs[p.ClusterID] = true
	}

	// 獲取所有叢集，為未配置權限的叢集新增預設權限
	var allClusters []models.Cluster
	if err := s.db.Find(&allClusters).Error; err != nil {
		return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
	}

	// 查詢使用者資訊（用於確定預設權限型別）
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("使用者不存在: %w", err)
	}

	// 確定預設權限型別
	defaultPermissionType := models.PermissionTypeReadonly
	if user.Username == "admin" {
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
func (s *PermissionService) GetUserAccessibleClusterIDs(userID uint) ([]uint, bool, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, false, apierrors.ErrUserNotFound()
	}

	// admin 使用者擁有全部叢集權限
	if user.Username == "admin" {
		return nil, true, nil
	}

	// 檢查使用者是否直接擁有 admin 權限（即為平臺管理員）
	var adminCount int64
	s.db.Model(&models.ClusterPermission{}).
		Where("user_id = ? AND permission_type = ?", userID, models.PermissionTypeAdmin).
		Count(&adminCount)
	if adminCount > 0 {
		return nil, true, nil
	}

	// 獲取使用者所在的使用者組
	var groupIDs []uint
	s.db.Model(&models.UserGroupMember{}).Where("user_id = ?", userID).Pluck("user_group_id", &groupIDs)

	// 檢查使用者組是否有 admin 權限
	if len(groupIDs) > 0 {
		s.db.Model(&models.ClusterPermission{}).
			Where("user_group_id IN ? AND permission_type = ?", groupIDs, models.PermissionTypeAdmin).
			Count(&adminCount)
		if adminCount > 0 {
			return nil, true, nil
		}
	}

	// 收集使用者有明確權限的叢集 ID（直接權限 + 使用者組權限）
	var clusterIDs []uint
	query := s.db.Model(&models.ClusterPermission{}).Where("user_id = ?", userID)
	if len(groupIDs) > 0 {
		query = s.db.Model(&models.ClusterPermission{}).Where("user_id = ? OR user_group_id IN ?", userID, groupIDs)
	}
	query.Distinct().Pluck("cluster_id", &clusterIDs)

	// 沒有任何明確權限記錄的使用者，擁有所有叢集的預設只讀權限
	// 與 GetUserAllClusterPermissions / GetUserClusterPermission 保持一致
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
	result := s.db.Delete(&models.ClusterPermission{}, ids)
	if result.Error != nil {
		return fmt.Errorf("批次刪除權限配置失敗: %w", result.Error)
	}
	return nil
}
