package services

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"

	"kubepolaris/internal/models"
	"kubepolaris/pkg/logger"

	"github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
)

// LDAPService LDAP服务
type LDAPService struct {
	db *gorm.DB
}

// LDAPUser LDAP用户信息
type LDAPUser struct {
	Username    string
	Email       string
	DisplayName string
	Groups      []string
}

// NewLDAPService 创建LDAP服务
func NewLDAPService(db *gorm.DB) *LDAPService {
	return &LDAPService{db: db}
}

// GetLDAPConfig 从数据库获取LDAP配置
func (s *LDAPService) GetLDAPConfig() (*models.LDAPConfig, error) {
	var setting models.SystemSetting
	if err := s.db.Where("config_key = ?", "ldap_config").First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 返回默认配置
			defaultConfig := models.GetDefaultLDAPConfig()
			return &defaultConfig, nil
		}
		return nil, err
	}

	var config models.LDAPConfig
	if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
		return nil, fmt.Errorf("解析LDAP配置失败: %w", err)
	}

	return &config, nil
}

// SaveLDAPConfig 保存LDAP配置到数据库
func (s *LDAPService) SaveLDAPConfig(config *models.LDAPConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化LDAP配置失败: %w", err)
	}

	var setting models.SystemSetting
	result := s.db.Where("config_key = ?", "ldap_config").First(&setting)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 创建新配置
		setting = models.SystemSetting{
			ConfigKey: "ldap_config",
			Value:     string(configJSON),
			Type:      "ldap",
		}
		return s.db.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	// 更新现有配置
	setting.Value = string(configJSON)
	return s.db.Save(&setting).Error
}

// Authenticate 使用LDAP认证用户
func (s *LDAPService) Authenticate(username, password string) (*LDAPUser, error) {
	config, err := s.GetLDAPConfig()
	if err != nil {
		return nil, fmt.Errorf("获取LDAP配置失败: %w", err)
	}

	if !config.Enabled {
		return nil, errors.New("LDAP未启用")
	}

	return s.AuthenticateWithConfig(username, password, config)
}

// AuthenticateWithConfig 使用指定的LDAP配置认证用户（用于测试）
func (s *LDAPService) AuthenticateWithConfig(username, password string, config *models.LDAPConfig) (*LDAPUser, error) {
	// 连接LDAP服务器
	conn, err := s.connect(config)
	if err != nil {
		return nil, fmt.Errorf("连接LDAP服务器失败: %w", err)
	}
	defer conn.Close()

	// 使用绑定账号进行绑定
	if config.BindDN != "" && config.BindPassword != "" {
		if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP绑定失败: %w", err)
		}
	}

	// 搜索用户
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
		return nil, fmt.Errorf("LDAP搜索失败: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, errors.New("用户不存在")
	}

	if len(result.Entries) > 1 {
		return nil, errors.New("找到多个匹配用户")
	}

	userEntry := result.Entries[0]
	userDN := userEntry.DN

	// 使用用户DN和密码进行绑定验证
	if err := conn.Bind(userDN, password); err != nil {
		return nil, errors.New("密码错误")
	}

	// 构建用户信息
	ldapUser := &LDAPUser{
		Username:    userEntry.GetAttributeValue(config.UsernameAttr),
		Email:       userEntry.GetAttributeValue(config.EmailAttr),
		DisplayName: userEntry.GetAttributeValue(config.DisplayNameAttr),
	}

	// 搜索用户组
	if config.GroupFilter != "" {
		groups, err := s.searchUserGroups(conn, config, username)
		if err != nil {
			logger.Warn("搜索用户组失败: %v", err)
		} else {
			ldapUser.Groups = groups
		}
	}

	return ldapUser, nil
}

// TestConnection 测试LDAP连接
func (s *LDAPService) TestConnection(config *models.LDAPConfig) error {
	conn, err := s.connect(config)
	if err != nil {
		return fmt.Errorf("连接LDAP服务器失败: %w", err)
	}
	defer conn.Close()

	// 如果配置了绑定DN，测试绑定
	if config.BindDN != "" && config.BindPassword != "" {
		if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
			return fmt.Errorf("LDAP绑定失败: %w", err)
		}
	}

	return nil
}

// connect 连接到LDAP服务器
func (s *LDAPService) connect(config *models.LDAPConfig) (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", config.Server, config.Port)

	var conn *ldap.Conn
	var err error

	if config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
		}
		conn, err = ldap.DialTLS("tcp", addr, tlsConfig)
	} else {
		conn, err = ldap.Dial("tcp", addr)
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// searchUserGroups 搜索用户所属的组
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
