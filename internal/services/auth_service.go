package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/apierrors"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

// LoginResult 登录结果
type LoginResult struct {
	Token       string                         `json:"token"`
	User        models.User                    `json:"user"`
	ExpiresAt   int64                          `json:"expires_at"`
	Permissions []models.MyPermissionsResponse `json:"permissions,omitempty"`
}

// AuthService 认证服务
type AuthService struct {
	db            *gorm.DB
	ldapService   *LDAPService
	permissionSvc *PermissionService
	jwtSecret     string
	jwtExpireTime int // 小时
}

// NewAuthService 创建认证服务
func NewAuthService(db *gorm.DB, jwtSecret string, jwtExpireTime int) *AuthService {
	return &AuthService{
		db:            db,
		ldapService:   NewLDAPService(db),
		permissionSvc: NewPermissionService(db),
		jwtSecret:     jwtSecret,
		jwtExpireTime: jwtExpireTime,
	}
}

// Login 用户登录，支持 local 和 ldap 两种认证方式
func (s *AuthService) Login(username, password, authType, clientIP string) (*LoginResult, error) {
	if authType == "" {
		authType = "local"
	}

	var user *models.User
	var err error

	switch authType {
	case "ldap":
		user, err = s.authenticateLDAP(username, password)
	case "local":
		user, err = s.authenticateLocal(username, password)
	default:
		return nil, apierrors.ErrAuthUnsupportedType()
	}
	if err != nil {
		return nil, err
	}

	if user.Status != "active" {
		return nil, apierrors.ErrAuthAccountDisabled()
	}

	// 生成JWT token
	tokenString, expiresAt, err := s.generateJWT(user)
	if err != nil {
		return nil, apierrors.ErrAuthTokenFailed()
	}

	// 更新最后登录时间和IP
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = clientIP
	s.db.Save(user)

	// 获取用户权限信息
	permissions := s.buildPermissions(user.ID)

	logger.Info("用户登录成功: %s (认证类型: %s)", user.Username, user.AuthType)

	return &LoginResult{
		Token:       tokenString,
		User:        *user,
		ExpiresAt:   expiresAt,
		Permissions: permissions,
	}, nil
}

// ChangePassword 修改密码（仅限本地用户）
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return apierrors.ErrUserNotFound()
	}

	if user.AuthType == "ldap" {
		return apierrors.ErrAuthLDAPReadonly()
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword+user.Salt)); err != nil {
		return apierrors.ErrAuthWrongPassword()
	}

	// 生成新密码哈希
	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword+user.Salt), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	user.PasswordHash = string(newHashedPassword)
	if err := s.db.Save(&user).Error; err != nil {
		return fmt.Errorf("密码更新失败: %w", err)
	}

	logger.Info("用户修改密码成功: %s", user.Username)
	return nil
}

// GetProfile 获取用户信息
func (s *AuthService) GetProfile(userID uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}
	return &user, nil
}

// GetAuthStatus 获取LDAP认证是否启用
func (s *AuthService) GetAuthStatus() (bool, error) {
	ldapConfig, err := s.ldapService.GetLDAPConfig()
	if err != nil {
		return false, nil
	}
	return ldapConfig.Enabled, nil
}

// authenticateLocal 本地密码认证
func (s *AuthService) authenticateLocal(username, password string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, apierrors.ErrAuthInvalidCredentials()
	}

	passwordWithSalt := password + user.Salt
	logger.Info("验证密码 - 用户: %s, Salt: %s", username, user.Salt)

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(passwordWithSalt)); err != nil {
		logger.Warn("密码验证失败 - 用户: %s, 错误: %v", username, err)
		return nil, apierrors.ErrAuthInvalidCredentials()
	}

	return &user, nil
}

// authenticateLDAP LDAP认证
func (s *AuthService) authenticateLDAP(username, password string) (*models.User, error) {
	ldapConfig, err := s.ldapService.GetLDAPConfig()
	if err != nil {
		return nil, fmt.Errorf("获取LDAP配置失败")
	}

	if !ldapConfig.Enabled {
		return nil, apierrors.ErrAuthLDAPNotEnabled()
	}

	ldapUser, err := s.ldapService.Authenticate(username, password)
	if err != nil {
		return nil, fmt.Errorf("LDAP认证失败: %v", err)
	}

	var user models.User
	result := s.db.Where("username = ? AND auth_type = ?", username, "ldap").First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		user = models.User{
			Username:    ldapUser.Username,
			Email:       ldapUser.Email,
			DisplayName: ldapUser.DisplayName,
			AuthType:    "ldap",
			Status:      "active",
		}
		if err := s.db.Create(&user).Error; err != nil {
			return nil, fmt.Errorf("创建用户记录失败")
		}
		logger.Info("LDAP用户首次登录，已创建本地记录: %s", username)
	} else if result.Error != nil {
		return nil, fmt.Errorf("查询用户失败")
	} else {
		user.Email = ldapUser.Email
		user.DisplayName = ldapUser.DisplayName
		s.db.Save(&user)
	}

	return &user, nil
}

// generateJWT 生成JWT token
func (s *AuthService) generateJWT(user *models.User) (string, int64, error) {
	expiresAt := time.Now().Add(time.Duration(s.jwtExpireTime) * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":   user.ID,
		"username":  user.Username,
		"auth_type": user.AuthType,
		"exp":       expiresAt.Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", 0, fmt.Errorf("签名token失败: %w", err)
	}

	return tokenString, expiresAt.Unix(), nil
}

// buildPermissions 构建用户权限响应
func (s *AuthService) buildPermissions(userID uint) []models.MyPermissionsResponse {
	clusterPermissions, _ := s.permissionSvc.GetUserAllClusterPermissions(userID)

	permissionResponses := make([]models.MyPermissionsResponse, 0, len(clusterPermissions))
	for _, p := range clusterPermissions {
		permissionName := ""
		for _, pt := range models.GetPermissionTypes() {
			if pt.Type == p.PermissionType {
				permissionName = pt.Name
				break
			}
		}

		clusterName := ""
		if p.Cluster != nil {
			clusterName = p.Cluster.Name
		}

		permissionResponses = append(permissionResponses, models.MyPermissionsResponse{
			ClusterID:      p.ClusterID,
			ClusterName:    clusterName,
			PermissionType: p.PermissionType,
			PermissionName: permissionName,
			Namespaces:     p.GetNamespaceList(),
			CustomRoleRef:  p.CustomRoleRef,
		})
	}

	return permissionResponses
}
