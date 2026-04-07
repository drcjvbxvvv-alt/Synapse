package services

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
)

// LDAPService LDAP服務
type LDAPService struct {
	db *gorm.DB
}

// LDAPUser LDAP使用者資訊
type LDAPUser struct {
	Username    string
	Email       string
	DisplayName string
	Groups      []string
}

// NewLDAPService 建立LDAP服務
func NewLDAPService(db *gorm.DB) *LDAPService {
	return &LDAPService{db: db}
}

// GetLDAPConfig 從資料庫獲取LDAP配置
func (s *LDAPService) GetLDAPConfig() (*models.LDAPConfig, error) {
	var config models.LDAPConfig
	found, err := GetSystemSetting(s.db, "ldap_config", &config)
	if err != nil {
		return nil, err
	}
	if !found {
		defaultConfig := models.GetDefaultLDAPConfig()
		return &defaultConfig, nil
	}
	return &config, nil
}

// SaveLDAPConfig 儲存LDAP配置到資料庫
func (s *LDAPService) SaveLDAPConfig(config *models.LDAPConfig) error {
	return SaveSystemSetting(s.db, "ldap_config", "ldap", config)
}

// Authenticate 使用LDAP認證使用者
func (s *LDAPService) Authenticate(username, password string) (*LDAPUser, error) {
	config, err := s.GetLDAPConfig()
	if err != nil {
		return nil, fmt.Errorf("獲取LDAP配置失敗: %w", err)
	}

	if !config.Enabled {
		return nil, errors.New("LDAP未啟用")
	}

	return s.AuthenticateWithConfig(username, password, config)
}

// AuthenticateWithConfig 使用指定的LDAP配置認證使用者（用於測試）
func (s *LDAPService) AuthenticateWithConfig(username, password string, config *models.LDAPConfig) (*LDAPUser, error) {
	// 連線LDAP伺服器
	conn, err := s.connect(config)
	if err != nil {
		return nil, fmt.Errorf("連線LDAP伺服器失敗: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// 使用繫結賬號進行繫結
	if config.BindDN != "" && config.BindPassword != "" {
		if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP繫結失敗: %w", err)
		}
	}

	// 搜尋使用者
	userFilter := fmt.Sprintf(config.UserFilter, ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		userFilter,
		[]string{"dn", config.UsernameAttr, config.EmailAttr, config.DisplayNameAttr},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP搜尋失敗: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, errors.New("使用者不存在")
	}

	if len(result.Entries) > 1 {
		return nil, errors.New("找到多個匹配使用者")
	}

	userEntry := result.Entries[0]
	userDN := userEntry.DN

	// 使用使用者DN和密碼進行繫結驗證
	if err := conn.Bind(userDN, password); err != nil {
		return nil, errors.New("密碼錯誤")
	}

	// 構建使用者資訊
	ldapUser := &LDAPUser{
		Username:    userEntry.GetAttributeValue(config.UsernameAttr),
		Email:       userEntry.GetAttributeValue(config.EmailAttr),
		DisplayName: userEntry.GetAttributeValue(config.DisplayNameAttr),
	}

	// 搜尋使用者組
	if config.GroupFilter != "" {
		groups, err := s.searchUserGroups(conn, config, username)
		if err != nil {
			logger.Warn("搜尋使用者組失敗: %v", err)
		} else {
			ldapUser.Groups = groups
		}
	}

	return ldapUser, nil
}

// TestConnection 測試LDAP連線
func (s *LDAPService) TestConnection(config *models.LDAPConfig) error {
	conn, err := s.connect(config)
	if err != nil {
		return fmt.Errorf("連線LDAP伺服器失敗: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// 如果配置了繫結DN，測試繫結
	if config.BindDN != "" && config.BindPassword != "" {
		if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
			return fmt.Errorf("LDAP繫結失敗: %w", err)
		}
	}

	return nil
}

// connect 連線到LDAP伺服器
func (s *LDAPService) connect(config *models.LDAPConfig) (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", config.Server, config.Port)

	var conn *ldap.Conn
	var err error

	if config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify, // #nosec G402 -- LDAP TLS 驗證由使用者配置 SkipTLSVerify 控制
		}
		ldapURL := fmt.Sprintf("ldaps://%s", addr)
		conn, err = ldap.DialURL(ldapURL, ldap.DialWithTLSConfig(tlsConfig))
	} else {
		ldapURL := fmt.Sprintf("ldap://%s", addr)
		conn, err = ldap.DialURL(ldapURL)
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// searchUserGroups 搜尋使用者所屬的組
func (s *LDAPService) searchUserGroups(conn *ldap.Conn, config *models.LDAPConfig, username string) ([]string, error) {
	groupFilter := fmt.Sprintf(config.GroupFilter, ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		groupFilter,
		[]string{config.GroupAttr},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	var groups []string
	for _, entry := range result.Entries {
		groupName := entry.GetAttributeValue(config.GroupAttr)
		if groupName != "" {
			groups = append(groups, groupName)
		}
	}

	return groups, nil
}
