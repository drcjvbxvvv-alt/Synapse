package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
)

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
	if err := s.db.WithContext(ctx).Model(&models.ClusterPermission{}).Where("user_group_id = ?", id).Count(&count).Error; err != nil {
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
	if err := s.db.WithContext(ctx).Preload("Users").First(&group, id).Error; err != nil {
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
	if err := s.db.WithContext(ctx).Preload("Users", func(db *gorm.DB) *gorm.DB {
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
