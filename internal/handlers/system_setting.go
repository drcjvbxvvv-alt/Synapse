package handlers

import (
	"net/http"

	"kubepolaris/internal/models"
	"kubepolaris/internal/services"
	"kubepolaris/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemSettingHandler 系统设置处理器
type SystemSettingHandler struct {
	db          *gorm.DB
	ldapService *services.LDAPService
}

// NewSystemSettingHandler 创建系统设置处理器
func NewSystemSettingHandler(db *gorm.DB) *SystemSettingHandler {
	return &SystemSettingHandler{
		db:          db,
		ldapService: services.NewLDAPService(db),
	}
}

// GetLDAPConfig 获取LDAP配置
func (h *SystemSettingHandler) GetLDAPConfig(c *gin.Context) {
	config, err := h.ldapService.GetLDAPConfig()
	if err != nil {
		logger.Error("获取LDAP配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取LDAP配置失败",
			"data":    nil,
		})
		return
	}

	// 返回配置时隐藏敏感信息
	safeConfig := *config
	if safeConfig.BindPassword != "" {
		safeConfig.BindPassword = "******"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    safeConfig,
	})
}

// UpdateLDAPConfigRequest LDAP配置更新请求
type UpdateLDAPConfigRequest struct {
	Enabled         bool   `json:"enabled"`
	Server          string `json:"server"`
	Port            int    `json:"port"`
	UseTLS          bool   `json:"use_tls"`
	SkipTLSVerify   bool   `json:"skip_tls_verify"`
	BindDN          string `json:"bind_dn"`
	BindPassword    string `json:"bind_password"`
	BaseDN          string `json:"base_dn"`
	UserFilter      string `json:"user_filter"`
	UsernameAttr    string `json:"username_attr"`
	EmailAttr       string `json:"email_attr"`
	DisplayNameAttr string `json:"display_name_attr"`
	GroupFilter     string `json:"group_filter"`
	GroupAttr       string `json:"group_attr"`
}

// UpdateLDAPConfig 更新LDAP配置
func (h *SystemSettingHandler) UpdateLDAPConfig(c *gin.Context) {
	var req UpdateLDAPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"data":    nil,
		})
		return
	}

	// 获取现有配置
	existingConfig, err := h.ldapService.GetLDAPConfig()
	if err != nil {
		logger.Error("获取现有LDAP配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新LDAP配置失败",
			"data":    nil,
		})
		return
	}

	// 构建新配置
	config := &models.LDAPConfig{
		Enabled:         req.Enabled,
		Server:          req.Server,
		Port:            req.Port,
		UseTLS:          req.UseTLS,
		SkipTLSVerify:   req.SkipTLSVerify,
		BindDN:          req.BindDN,
		BaseDN:          req.BaseDN,
		UserFilter:      req.UserFilter,
		UsernameAttr:    req.UsernameAttr,
		EmailAttr:       req.EmailAttr,
		DisplayNameAttr: req.DisplayNameAttr,
		GroupFilter:     req.GroupFilter,
		GroupAttr:       req.GroupAttr,
	}

	// 如果密码是占位符或空，保留原密码
	if req.BindPassword != "" && req.BindPassword != "******" {
		config.BindPassword = req.BindPassword
	} else {
		config.BindPassword = existingConfig.BindPassword
	}

	// 保存配置
	if err := h.ldapService.SaveLDAPConfig(config); err != nil {
		logger.Error("保存LDAP配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存LDAP配置失败",
			"data":    nil,
		})
		return
	}

	logger.Info("LDAP配置更新成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "LDAP配置更新成功",
		"data":    nil,
	})
}

// TestLDAPConnection 测试LDAP连接
func (h *SystemSettingHandler) TestLDAPConnection(c *gin.Context) {
	var req UpdateLDAPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"data":    nil,
		})
		return
	}

	// 获取现有配置以获取可能未更新的密码
	existingConfig, _ := h.ldapService.GetLDAPConfig()

	// 构建测试配置
	config := &models.LDAPConfig{
		Enabled:         true, // 测试时始终启用
		Server:          req.Server,
		Port:            req.Port,
		UseTLS:          req.UseTLS,
		SkipTLSVerify:   req.SkipTLSVerify,
		BindDN:          req.BindDN,
		BaseDN:          req.BaseDN,
		UserFilter:      req.UserFilter,
		UsernameAttr:    req.UsernameAttr,
		EmailAttr:       req.EmailAttr,
		DisplayNameAttr: req.DisplayNameAttr,
		GroupFilter:     req.GroupFilter,
		GroupAttr:       req.GroupAttr,
	}

	// 处理密码
	if req.BindPassword != "" && req.BindPassword != "******" {
		config.BindPassword = req.BindPassword
	} else if existingConfig != nil {
		config.BindPassword = existingConfig.BindPassword
	}

	// 测试连接
	if err := h.ldapService.TestConnection(config); err != nil {
		logger.Warn("LDAP连接测试失败: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": err.Error(),
			"data": gin.H{
				"success": false,
				"error":   err.Error(),
			},
		})
		return
	}

	logger.Info("LDAP连接测试成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "LDAP连接测试成功",
		"data": gin.H{
			"success": true,
		},
	})
}

// TestLDAPAuthRequest LDAP认证测试请求
type TestLDAPAuthRequest struct {
	Username        string `json:"username" binding:"required"`
	Password        string `json:"password" binding:"required"`
	Server          string `json:"server"`
	Port            int    `json:"port"`
	UseTLS          bool   `json:"use_tls"`
	SkipTLSVerify   bool   `json:"skip_tls_verify"`
	BindDN          string `json:"bind_dn"`
	BindPassword    string `json:"bind_password"`
	BaseDN          string `json:"base_dn"`
	UserFilter      string `json:"user_filter"`
	UsernameAttr    string `json:"username_attr"`
	EmailAttr       string `json:"email_attr"`
	DisplayNameAttr string `json:"display_name_attr"`
	GroupFilter     string `json:"group_filter"`
	GroupAttr       string `json:"group_attr"`
}

// TestLDAPAuth 测试LDAP用户认证
func (h *SystemSettingHandler) TestLDAPAuth(c *gin.Context) {
	var req TestLDAPAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"data":    nil,
		})
		return
	}

	// 获取现有配置以获取可能未更新的密码
	existingConfig, _ := h.ldapService.GetLDAPConfig()

	// 构建测试配置
	config := &models.LDAPConfig{
		Enabled:         true, // 测试时始终启用
		Server:          req.Server,
		Port:            req.Port,
		UseTLS:          req.UseTLS,
		SkipTLSVerify:   req.SkipTLSVerify,
		BindDN:          req.BindDN,
		BaseDN:          req.BaseDN,
		UserFilter:      req.UserFilter,
		UsernameAttr:    req.UsernameAttr,
		EmailAttr:       req.EmailAttr,
		DisplayNameAttr: req.DisplayNameAttr,
		GroupFilter:     req.GroupFilter,
		GroupAttr:       req.GroupAttr,
	}

	// 处理绑定密码
	if req.BindPassword != "" && req.BindPassword != "******" {
		config.BindPassword = req.BindPassword
	} else if existingConfig != nil {
		config.BindPassword = existingConfig.BindPassword
	}

	// 尝试认证
	ldapUser, err := h.ldapService.AuthenticateWithConfig(req.Username, req.Password, config)
	if err != nil {
		logger.Warn("LDAP用户认证测试失败: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": err.Error(),
			"data": gin.H{
				"success": false,
				"error":   err.Error(),
			},
		})
		return
	}

	logger.Info("LDAP用户认证测试成功: %s", req.Username)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "LDAP用户认证测试成功",
		"data": gin.H{
			"success":      true,
			"username":     ldapUser.Username,
			"email":        ldapUser.Email,
			"display_name": ldapUser.DisplayName,
			"groups":       ldapUser.Groups,
		},
	})
}
