package services

import (
	"errors"
	"fmt"

	"github.com/clay-wangzhi/KubePolaris/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserService 用户管理服务
type UserService struct {
	db *gorm.DB
}

// NewUserService 创建用户管理服务
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required,min=6"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	Phone       *string `json:"phone"`
}

// ListUsersParams 用户列表查询参数
type ListUsersParams struct {
	Page     int
	PageSize int
	Search   string
	Status   string
	AuthType string
}

// CreateUser 创建本地用户
func (s *UserService) CreateUser(req *CreateUserRequest) (*models.User, error) {
	var count int64
	s.db.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	salt := fmt.Sprintf("kp_%s_salt", req.Username)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password+salt), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
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
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// UpdateUser 更新用户信息
func (s *UserService) UpdateUser(id uint, req *UpdateUserRequest) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, errors.New("用户不存在")
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
		return nil, fmt.Errorf("更新用户失败: %w", err)
	}

	return &user, nil
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(id uint) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return errors.New("用户不存在")
	}

	if user.Username == "admin" {
		return errors.New("不能删除 admin 用户")
	}

	// 清除用户组关联
	s.db.Where("user_id = ?", id).Delete(&models.UserGroupMember{})
	// 清除集群权限
	s.db.Where("user_id = ?", id).Delete(&models.ClusterPermission{})

	if err := s.db.Delete(&user).Error; err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	return nil
}

// GetUser 获取用户详情
func (s *UserService) GetUser(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, errors.New("用户不存在")
	}
	return &user, nil
}

// ListUsers 获取用户列表（分页、搜索、过滤）
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
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}

	return users, total, nil
}

// UpdateUserStatus 更新用户状态
func (s *UserService) UpdateUserStatus(id uint, status string) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return errors.New("用户不存在")
	}

	if user.Username == "admin" {
		return errors.New("不能修改 admin 用户状态")
	}

	if status != "active" && status != "inactive" {
		return errors.New("无效的状态值")
	}

	user.Status = status
	return s.db.Save(&user).Error
}

// ResetPassword 重置用户密码
func (s *UserService) ResetPassword(id uint, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return errors.New("用户不存在")
	}

	if user.AuthType == "ldap" {
		return errors.New("LDAP 用户不能重置密码")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword+user.Salt), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	user.PasswordHash = string(hashedPassword)
	return s.db.Save(&user).Error
}
