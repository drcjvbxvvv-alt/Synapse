package services

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/apierrors"
	"github.com/clay-wangzhi/Synapse/internal/models"
)

// UserService 使用者管理服務
type UserService struct {
	db *gorm.DB
}

// NewUserService 建立使用者管理服務
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
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
func (s *UserService) CreateUser(req *CreateUserRequest) (*models.User, error) {
	var count int64
	s.db.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		return nil, apierrors.ErrUserDuplicateUsername()
	}

	salt := fmt.Sprintf("kp_%s_salt", req.Username)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password+salt), bcrypt.DefaultCost)
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

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("建立使用者失敗: %w", err)
	}

	return user, nil
}

// UpdateUser 更新使用者資訊
func (s *UserService) UpdateUser(id uint, req *UpdateUserRequest) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
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

	if err := s.db.Save(&user).Error; err != nil {
		return nil, fmt.Errorf("更新使用者失敗: %w", err)
	}

	return &user, nil
}

// DeleteUser 刪除使用者
func (s *UserService) DeleteUser(id uint) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return apierrors.ErrUserNotFound()
	}

	if user.Username == "admin" {
		return apierrors.ErrUserAdminProtected()
	}

	// 清除使用者組關聯
	s.db.Where("user_id = ?", id).Delete(&models.UserGroupMember{})
	// 清除叢集權限
	s.db.Where("user_id = ?", id).Delete(&models.ClusterPermission{})

	if err := s.db.Delete(&user).Error; err != nil {
		return fmt.Errorf("刪除使用者失敗: %w", err)
	}
	return nil
}

// GetUser 獲取使用者詳情
func (s *UserService) GetUser(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}
	return &user, nil
}

// ListUsers 獲取使用者列表（分頁、搜尋、過濾）
func (s *UserService) ListUsers(params *ListUsersParams) ([]models.User, int64, error) {
	query := s.db.Model(&models.User{})

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
func (s *UserService) UpdateUserStatus(id uint, status string) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return apierrors.ErrUserNotFound()
	}

	if user.Username == "admin" {
		return apierrors.ErrUserAdminProtected()
	}

	if status != "active" && status != "inactive" {
		return apierrors.ErrUserInvalidStatus()
	}

	user.Status = status
	return s.db.Save(&user).Error
}

// ResetPassword 重置使用者密碼
func (s *UserService) ResetPassword(id uint, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return apierrors.ErrUserNotFound()
	}

	if user.AuthType == "ldap" {
		return apierrors.ErrAuthLDAPReadonly()
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword+user.Salt), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密碼加密失敗: %w", err)
	}

	user.PasswordHash = string(hashedPassword)
	return s.db.Save(&user).Error
}
