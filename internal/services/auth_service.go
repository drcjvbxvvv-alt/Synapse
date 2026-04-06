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

// LoginResult 登入結果
type LoginResult struct {
	Token       string                         `json:"token"`
	User        models.User                    `json:"user"`
	ExpiresAt   int64                          `json:"expires_at"`
	Permissions []models.MyPermissionsResponse `json:"permissions,omitempty"`
}

// AuthService 認證服務
type AuthService struct {
	db            *gorm.DB
	ldapService   *LDAPService
	permissionSvc *PermissionService
	jwtSecret     string
	jwtExpireTime int // 小時
}

// NewAuthService 建立認證服務
func NewAuthService(db *gorm.DB, jwtSecret string, jwtExpireTime int) *AuthService {
	return &AuthService{
		db:            db,
		ldapService:   NewLDAPService(db),
		permissionSvc: NewPermissionService(db),
		jwtSecret:     jwtSecret,
		jwtExpireTime: jwtExpireTime,
	}
}

// Login 使用者登入，支援 local 和 ldap 兩種認證方式
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

	// 更新最後登入時間和IP
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = clientIP
	s.db.Save(user)

	// 獲取使用者權限資訊
	permissions := s.buildPermissions(user.ID)

	logger.Info("使用者登入成功: %s (認證型別: %s)", user.Username, user.AuthType)

	return &LoginResult{
		Token:       tokenString,
		User:        *user,
		ExpiresAt:   expiresAt,
		Permissions: permissions,
	}, nil
}

// ChangePassword 修改密碼（僅限本地使用者）
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return apierrors.ErrUserNotFound()
	}

	if user.AuthType == "ldap" {
		return apierrors.ErrAuthLDAPReadonly()
	}

	// 驗證舊密碼
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword+user.Salt)); err != nil {
		return apierrors.ErrAuthWrongPassword()
	}

	// 生成新密碼雜湊
	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword+user.Salt), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密碼加密失敗: %w", err)
	}

	user.PasswordHash = string(newHashedPassword)
	if err := s.db.Save(&user).Error; err != nil {
		return fmt.Errorf("密碼更新失敗: %w", err)
	}

	logger.Info("使用者修改密碼成功: %s", user.Username)
	return nil
}

// GetProfile 獲取使用者資訊
func (s *AuthService) GetProfile(userID uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}
	return &user, nil
}

// GetAuthStatus 獲取LDAP認證是否啟用
func (s *AuthService) GetAuthStatus() (bool, error) {
	ldapConfig, err := s.ldapService.GetLDAPConfig()
	if err != nil {
		return false, nil
	}
	return ldapConfig.Enabled, nil
}

// authenticateLocal 本地密碼認證
func (s *AuthService) authenticateLocal(username, password string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, apierrors.ErrAuthInvalidCredentials()
	}

	passwordWithSalt := password + user.Salt
	logger.Info("驗證密碼 - 使用者: %s, Salt: %s", username, user.Salt)

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(passwordWithSalt)); err != nil {
		logger.Warn("密碼驗證失敗 - 使用者: %s, 錯誤: %v", username, err)
		return nil, apierrors.ErrAuthInvalidCredentials()
	}

	return &user, nil
}

// authenticateLDAP LDAP認證
func (s *AuthService) authenticateLDAP(username, password string) (*models.User, error) {
	ldapConfig, err := s.ldapService.GetLDAPConfig()
	if err != nil {
		return nil, fmt.Errorf("獲取LDAP配置失敗")
	}

	if !ldapConfig.Enabled {
		return nil, apierrors.ErrAuthLDAPNotEnabled()
	}

	ldapUser, err := s.ldapService.Authenticate(username, password)
	if err != nil {
		return nil, fmt.Errorf("LDAP認證失敗: %v", err)
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
			return nil, fmt.Errorf("建立使用者記錄失敗")
		}
		logger.Info("LDAP使用者首次登入，已建立本地記錄: %s", username)
	} else if result.Error != nil {
		return nil, fmt.Errorf("查詢使用者失敗")
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
		return "", 0, fmt.Errorf("簽名token失敗: %w", err)
	}

	return tokenString, expiresAt.Unix(), nil
}

// buildPermissions 構建使用者權限響應
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
