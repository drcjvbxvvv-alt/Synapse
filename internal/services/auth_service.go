package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
	"github.com/shaia/Synapse/pkg/logger"
)

// JWT 標準 claims 常數
const (
	JWTIssuer   = "synapse"
	JWTAudience = "synapse-api"

	// RefreshTokenCookieName 是 httpOnly cookie 的名稱
	RefreshTokenCookieName = "synapse_refresh_token"
	// RefreshTokenExpireDays refresh token 有效天數
	RefreshTokenExpireDays = 7
)

// LoginResult 登入結果
type LoginResult struct {
	Token       string                         `json:"token"`
	JTI         string                         `json:"-"` // 不回傳給前端，用於伺服端稽核
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
//
// permRepo may be nil — AuthService will then fall back to the legacy
// *gorm.DB path inside its private PermissionService. When the router
// wires a real repo, the dual-path feature flag decides which path runs.
func NewAuthService(
	db *gorm.DB,
	jwtSecret string,
	jwtExpireTime int,
	permRepo repositories.PermissionRepository,
) *AuthService {
	return &AuthService{
		db:            db,
		ldapService:   NewLDAPService(db),
		permissionSvc: NewPermissionService(db, permRepo),
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
	tokenString, jti, expiresAt, err := s.generateJWT(user)
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

	logger.Info("使用者登入成功", "username", user.Username, "auth_type", user.AuthType)

	return &LoginResult{
		Token:       tokenString,
		JTI:         jti,
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
	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword+user.Salt), 12)
	if err != nil {
		return fmt.Errorf("密碼加密失敗: %w", err)
	}

	user.PasswordHash = string(newHashedPassword)
	if err := s.db.Save(&user).Error; err != nil {
		return fmt.Errorf("密碼更新失敗: %w", err)
	}

	logger.Info("使用者修改密碼成功", "username", user.Username)
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
	logger.Debug("authenticating local user", "username", username)

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(passwordWithSalt)); err != nil {
		logger.Warn("密碼驗證失敗", "username", username, "error", err)
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
		// 僅記錄詳細錯誤於 server log，避免 LDAP 伺服器資訊洩露給客戶端
		logger.Warn("LDAP認證失敗", "username", username, "error", err)
		return nil, apierrors.ErrAuthInvalidCredentials()
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
		logger.Info("LDAP使用者首次登入，已建立本地記錄", "username", username)
	} else if result.Error != nil {
		return nil, fmt.Errorf("查詢使用者失敗")
	} else {
		user.Email = ldapUser.Email
		user.DisplayName = ldapUser.DisplayName
		s.db.Save(&user)
	}

	return &user, nil
}

// IssueAccessToken 以 refresh token 換新 access token。
// 流程：驗證 refresh token JWT → 確認 token_type == "refresh" → 從 DB 載入最新使用者資料 → 簽發新 access token。
func (s *AuthService) IssueAccessToken(refreshTokenStr string) (*LoginResult, error) {
	token, err := jwt.Parse(
		refreshTokenStr,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(s.jwtSecret), nil
		},
		jwt.WithIssuer(JWTIssuer),
		jwt.WithAudience(JWTAudience),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !token.Valid {
		return nil, apierrors.ErrAuthTokenFailed()
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, apierrors.ErrAuthTokenFailed()
	}

	if tokenType, _ := claims["token_type"].(string); tokenType != "refresh" {
		return nil, apierrors.ErrAuthTokenFailed()
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return nil, apierrors.ErrAuthTokenFailed()
	}
	userID := uint(userIDFloat)

	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, apierrors.ErrUserNotFound()
	}
	if user.Status != "active" {
		return nil, apierrors.ErrAuthAccountDisabled()
	}

	tokenString, jti, expiresAt, err := s.generateJWT(&user)
	if err != nil {
		return nil, apierrors.ErrAuthTokenFailed()
	}

	permissions := s.buildPermissions(userID)

	return &LoginResult{
		Token:       tokenString,
		JTI:         jti,
		User:        user,
		ExpiresAt:   expiresAt,
		Permissions: permissions,
	}, nil
}

// GenerateRefreshToken 生成 refresh token 字串（不儲存到 DB，由 cookie 管理生命週期）
func (s *AuthService) GenerateRefreshToken(user *models.User) (string, string, error) {
	now := time.Now()
	expiresAt := now.Add(RefreshTokenExpireDays * 24 * time.Hour)
	jti := uuid.NewString()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        jti,
		"iss":        JWTIssuer,
		"aud":        JWTAudience,
		"nbf":        now.Unix(),
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"user_id":    user.ID,
		"token_type": "refresh",
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", fmt.Errorf("簽名 refresh token 失敗: %w", err)
	}
	return tokenString, jti, nil
}

// generateJWT 生成 JWT token（含 jti/iss/aud/nbf 標準 claims）
// 回傳 (token 字串, jti, exp Unix 時間戳, error)
//
// claim 說明：
//   - jti  JWT ID，由 uuid v4 產生，用於支援黑名單撤銷（P0-5）
//   - iss  簽發者，固定為 synapse，供消費端驗證 token 來源
//   - aud  受眾，固定為 synapse-api，跨服務時區隔 token 用途
//   - nbf  Not Before，現在起生效（防止時鐘回退攻擊）
//   - exp  過期時間，由 config.JWT.ExpireTime 決定
func (s *AuthService) generateJWT(user *models.User) (string, string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(s.jwtExpireTime) * time.Hour)
	jti := uuid.NewString()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":         jti,
		"iss":         JWTIssuer,
		"aud":         JWTAudience,
		"nbf":         now.Unix(),
		"iat":         now.Unix(),
		"exp":         expiresAt.Unix(),
		"user_id":     user.ID,
		"username":    user.Username,
		"auth_type":   user.AuthType,
		"system_role": user.SystemRole,
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", 0, fmt.Errorf("簽名token失敗: %w", err)
	}

	return tokenString, jti, expiresAt.Unix(), nil
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
