package services

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/features"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
)

// UserService 使用者管理服務
//
// P0-4b migration status: dual-path. When features.FlagRepositoryLayer is
// enabled the service routes reads/writes through the Repository layer;
// when disabled it keeps the legacy *gorm.DB path intact so the flag can
// be flipped off safely.
type UserService struct {
	db   *gorm.DB
	repo repositories.UserRepository
}

// NewUserService 建立使用者管理服務
func NewUserService(db *gorm.DB, repo repositories.UserRepository) *UserService {
	return &UserService{db: db, repo: repo}
}

func (s *UserService) useRepo() bool {
	return s.repo != nil && features.IsEnabled(features.FlagRepositoryLayer)
}

// CreateUserRequest 建立使用者請求
type CreateUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required,min=6"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
}

// UpdateUserRequest 更新使用者請求
type UpdateUserRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	Phone       *string `json:"phone"`
}

// ListUsersParams 使用者列表查詢參數
type ListUsersParams struct {
	Page     int
	PageSize int
	Search   string
	Status   string
	AuthType string
}

// CreateUser 建立本地使用者
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*models.User, error) {
	exists, err := s.usernameExists(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apierrors.ErrUserDuplicateUsername()
	}

	salt := fmt.Sprintf("kp_%s_salt", req.Username)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password+salt), 12)
	if err != nil {
		return nil, fmt.Errorf("密碼加密失敗: %w", err)
	}

	user := &models.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		Salt:         salt,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		Phone:        req.Phone,
		AuthType:     "local",
		Status:       "active",
	}

	if s.useRepo() {
		if err := s.repo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("建立使用者失敗: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
			return nil, fmt.Errorf("建立使用者失敗: %w", err)
		}
	}

	return user, nil
}

// usernameExists is a tiny helper so CreateUser does not branch twice.
func (s *UserService) usernameExists(ctx context.Context, username string) (bool, error) {
	if s.useRepo() {
		ok, err := s.repo.Exists(ctx, "username = ?", username)
		if err != nil {
			return false, fmt.Errorf("檢查使用者名稱失敗: %w", err)
		}
		return ok, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("username = ?", username).Count(&count).Error; err != nil {
		return false, fmt.Errorf("檢查使用者名稱失敗: %w", err)
	}
	return count > 0, nil
}

// UpdateUser 更新使用者資訊
func (s *UserService) UpdateUser(ctx context.Context, id uint, req *UpdateUserRequest) (*models.User, error) {
	user, err := s.fetchUser(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}

	if s.useRepo() {
		if err := s.repo.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("更新使用者失敗: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(user).Error; err != nil {
			return nil, fmt.Errorf("更新使用者失敗: %w", err)
		}
	}

	return user, nil
}

// DeleteUser 刪除使用者
//
// Cross-table cleanup stays on the legacy *gorm.DB path because there is no
// dedicated UserGroupMember / ClusterPermission repository method wired into
// UserService. The Permission side of the cleanup is covered by
// PermissionRepository.DeleteMembershipsByUser / DeletePermissionsByUser —
// once those are injected into UserService, this method can be collapsed
// into a single Transaction block.
func (s *UserService) DeleteUser(ctx context.Context, id uint) error {
	user, err := s.fetchUser(ctx, id)
	if err != nil {
		return err
	}

	if user.IsPlatformAdmin() {
		return apierrors.ErrUserAdminProtected()
	}

	db := s.db.WithContext(ctx)
	// Best-effort cleanup of related rows — failures here should not prevent
	// the user delete itself.
	db.Where("user_id = ?", id).Delete(&models.UserGroupMember{})
	db.Where("user_id = ?", id).Delete(&models.ClusterPermission{})

	if s.useRepo() {
		if err := s.repo.Delete(ctx, id); err != nil {
			return fmt.Errorf("刪除使用者失敗: %w", err)
		}
		return nil
	}
	if err := db.Delete(user).Error; err != nil {
		return fmt.Errorf("刪除使用者失敗: %w", err)
	}
	return nil
}

// GetUser 獲取使用者詳情
func (s *UserService) GetUser(ctx context.Context, id uint) (*models.User, error) {
	return s.fetchUser(ctx, id)
}

// fetchUser is the shared read path used by Get/Update/Delete so all three
// observe the same feature-flag branching and the same error translation.
func (s *UserService) fetchUser(ctx context.Context, id uint) (*models.User, error) {
	if s.useRepo() {
		u, err := s.repo.Get(ctx, id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) ||
				errors.Is(err, repositories.ErrInvalidArgument) {
				return nil, apierrors.ErrUserNotFound()
			}
			return nil, fmt.Errorf("獲取使用者失敗: %w", err)
		}
		return u, nil
	}
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.ErrUserNotFound()
		}
		return nil, fmt.Errorf("獲取使用者失敗: %w", err)
	}
	return &user, nil
}

// ListUsers 獲取使用者列表（分頁、搜尋、過濾）
func (s *UserService) ListUsers(ctx context.Context, params *ListUsersParams) ([]models.User, int64, error) {
	if s.useRepo() {
		ptrs, total, err := s.repo.ListPaged(ctx, repositories.ListUsersFilter{
			Page:     params.Page,
			PageSize: params.PageSize,
			Search:   params.Search,
			Status:   params.Status,
			AuthType: params.AuthType,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("查詢使用者列表失敗: %w", err)
		}
		// Caller expects a slice of values, not pointers — deref here rather
		// than forcing every handler to change.
		users := make([]models.User, len(ptrs))
		for i, u := range ptrs {
			users[i] = *u
		}
		return users, total, nil
	}

	query := s.db.WithContext(ctx).Model(&models.User{})

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?", search, search, search)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.AuthType != "" {
		query = query.Where("auth_type = ?", params.AuthType)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("id ASC").
		Offset(offset).Limit(params.PageSize).
		Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("查詢使用者列表失敗: %w", err)
	}

	return users, total, nil
}

// UpdateUserStatus 更新使用者狀態
func (s *UserService) UpdateUserStatus(ctx context.Context, id uint, status string) error {
	user, err := s.fetchUser(ctx, id)
	if err != nil {
		return err
	}

	if user.IsPlatformAdmin() {
		return apierrors.ErrUserAdminProtected()
	}

	if status != "active" && status != "inactive" {
		return apierrors.ErrUserInvalidStatus()
	}

	user.Status = status
	if s.useRepo() {
		return s.repo.Update(ctx, user)
	}
	return s.db.WithContext(ctx).Save(user).Error
}

// ResetPassword 重置使用者密碼
func (s *UserService) ResetPassword(ctx context.Context, id uint, newPassword string) error {
	user, err := s.fetchUser(ctx, id)
	if err != nil {
		return err
	}

	if user.AuthType == "ldap" {
		return apierrors.ErrAuthLDAPReadonly()
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword+user.Salt), 12)
	if err != nil {
		return fmt.Errorf("密碼加密失敗: %w", err)
	}

	user.PasswordHash = string(hashedPassword)
	if s.useRepo() {
		return s.repo.Update(ctx, user)
	}
	return s.db.WithContext(ctx).Save(user).Error
}
