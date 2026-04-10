package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// SystemSettingHandler 系統設定處理器
type SystemSettingHandler struct {
	clusterService        *services.ClusterService
	ldapService           *services.LDAPService
	sshSettingService     *services.SSHSettingService
	grafanaSettingService *services.GrafanaSettingService
	grafanaService        *services.GrafanaService
}

// NewSystemSettingHandler 建立系統設定處理器
func NewSystemSettingHandler(
	clusterSvc *services.ClusterService,
	ldapSvc *services.LDAPService,
	sshSvc *services.SSHSettingService,
	grafanaSettingSvc *services.GrafanaSettingService,
	grafanaSvc *services.GrafanaService,
) *SystemSettingHandler {
	return &SystemSettingHandler{
		clusterService:        clusterSvc,
		ldapService:           ldapSvc,
		sshSettingService:     sshSvc,
		grafanaSettingService: grafanaSettingSvc,
		grafanaService:        grafanaSvc,
	}
}

// ==================== LDAP 配置相關介面 ====================

// UpdateLDAPConfigRequest LDAP配置更新請求
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

// GetLDAPConfig 獲取LDAP配置
func (h *SystemSettingHandler) GetLDAPConfig(c *gin.Context) {
	config, err := h.ldapService.GetLDAPConfig()
	if err != nil {
		logger.Error("獲取LDAP配置失敗: %v", err)
		response.InternalError(c, "獲取LDAP配置失敗")
		return
	}

	// 返回配置時隱藏敏感資訊
	safeConfig := *config
	if safeConfig.BindPassword != "" {
		safeConfig.BindPassword = "******"
	}

	response.OK(c, safeConfig)
}

// UpdateLDAPConfig 更新LDAP配置
func (h *SystemSettingHandler) UpdateLDAPConfig(c *gin.Context) {
	var req UpdateLDAPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	// 獲取現有配置
	existingConfig, err := h.ldapService.GetLDAPConfig()
	if err != nil {
		logger.Error("獲取現有LDAP配置失敗: %v", err)
		response.InternalError(c, "更新LDAP配置失敗")
		return
	}

	// 構建新配置
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

	// 如果密碼是佔位符或空，保留原密碼
	if req.BindPassword != "" && req.BindPassword != "******" {
		config.BindPassword = req.BindPassword
	} else {
		config.BindPassword = existingConfig.BindPassword
	}

	// 儲存配置
	if err := h.ldapService.SaveLDAPConfig(config); err != nil {
		logger.Error("儲存LDAP配置失敗: %v", err)
		response.InternalError(c, "儲存LDAP配置失敗")
		return
	}

	logger.Info("LDAP配置更新成功")

	response.OK(c, gin.H{"message": "LDAP配置更新成功"})
}

// TestLDAPConnection 測試LDAP連線
func (h *SystemSettingHandler) TestLDAPConnection(c *gin.Context) {
	var req UpdateLDAPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	// 獲取現有配置以獲取可能未更新的密碼
	existingConfig, _ := h.ldapService.GetLDAPConfig()

	// 構建測試配置
	config := &models.LDAPConfig{
		Enabled:         true, // 測試時始終啟用
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

	// 處理密碼
	if req.BindPassword != "" && req.BindPassword != "******" {
		config.BindPassword = req.BindPassword
	} else if existingConfig != nil {
		config.BindPassword = existingConfig.BindPassword
	}

	// 測試連線
	if err := h.ldapService.TestConnection(config); err != nil {
		logger.Warn("LDAP連線測試失敗: %v", err)
		response.OK(c, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.Info("LDAP連線測試成功")

	response.OK(c, gin.H{
		"success": true,
	})
}

// TestLDAPAuthRequest LDAP認證測試請求
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

// TestLDAPAuth 測試LDAP使用者認證
func (h *SystemSettingHandler) TestLDAPAuth(c *gin.Context) {
	var req TestLDAPAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	// 獲取現有配置以獲取可能未更新的密碼
	existingConfig, _ := h.ldapService.GetLDAPConfig()

	// 構建測試配置
	config := &models.LDAPConfig{
		Enabled:         true, // 測試時始終啟用
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

	// 處理繫結密碼
	if req.BindPassword != "" && req.BindPassword != "******" {
		config.BindPassword = req.BindPassword
	} else if existingConfig != nil {
		config.BindPassword = existingConfig.BindPassword
	}

	// 嘗試認證
	ldapUser, err := h.ldapService.AuthenticateWithConfig(req.Username, req.Password, config)
	if err != nil {
		logger.Warn("LDAP使用者認證測試失敗: %v", err)
		response.OK(c, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.Info("LDAP使用者認證測試成功: %s", req.Username)

	response.OK(c, gin.H{
		"success":      true,
		"username":     ldapUser.Username,
		"email":        ldapUser.Email,
		"display_name": ldapUser.DisplayName,
		"groups":       ldapUser.Groups,
	})
}

// ==================== SSH 配置相關介面 ====================

// UpdateSSHConfigRequest SSH配置更新請求
type UpdateSSHConfigRequest struct {
	Enabled    bool   `json:"enabled"`
	Username   string `json:"username"`
	Port       int    `json:"port"`
	AuthType   string `json:"auth_type"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

// GetSSHConfig 獲取SSH配置
func (h *SystemSettingHandler) GetSSHConfig(c *gin.Context) {
	config, err := h.sshSettingService.GetSSHConfig()
	if err != nil {
		logger.Error("獲取SSH配置失敗: %v", err)
		response.InternalError(c, "獲取SSH配置失敗")
		return
	}

	// 返回配置時隱藏敏感資訊
	safeConfig := *config
	if safeConfig.Password != "" {
		safeConfig.Password = "******"
	}
	if safeConfig.PrivateKey != "" {
		safeConfig.PrivateKey = "******"
	}

	response.OK(c, safeConfig)
}

// UpdateSSHConfig 更新SSH配置
func (h *SystemSettingHandler) UpdateSSHConfig(c *gin.Context) {
	var req UpdateSSHConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	// 獲取現有配置
	existingConfig, err := h.sshSettingService.GetSSHConfig()
	if err != nil {
		logger.Error("獲取現有SSH配置失敗: %v", err)
		response.InternalError(c, "更新SSH配置失敗")
		return
	}

	// 構建新配置
	config := &models.SSHConfig{
		Enabled:  req.Enabled,
		Username: req.Username,
		Port:     req.Port,
		AuthType: req.AuthType,
	}

	// 設定預設值
	if config.Username == "" {
		config.Username = "root"
	}
	if config.Port == 0 {
		config.Port = 22
	}
	if config.AuthType == "" {
		config.AuthType = "password"
	}

	// 如果密碼是佔位符或空，保留原密碼
	if req.Password != "" && req.Password != "******" {
		config.Password = req.Password
	} else {
		config.Password = existingConfig.Password
	}

	// 如果私鑰是佔位符或空，保留原私鑰
	if req.PrivateKey != "" && req.PrivateKey != "******" {
		config.PrivateKey = req.PrivateKey
	} else {
		config.PrivateKey = existingConfig.PrivateKey
	}

	// 儲存配置
	if err := h.sshSettingService.SaveSSHConfig(config); err != nil {
		logger.Error("儲存SSH配置失敗: %v", err)
		response.InternalError(c, "儲存SSH配置失敗")
		return
	}

	logger.Info("SSH配置更新成功")

	response.OK(c, gin.H{"message": "SSH配置更新成功"})
}

// GetSSHCredentials 獲取SSH憑據（用於自動連線，返回完整憑據）
func (h *SystemSettingHandler) GetSSHCredentials(c *gin.Context) {
	config, err := h.sshSettingService.GetSSHConfig()
	if err != nil {
		logger.Error("獲取SSH憑據失敗: %v", err)
		response.InternalError(c, "獲取SSH憑據失敗")
		return
	}

	// 檢查是否啟用
	if !config.Enabled {
		response.OK(c, gin.H{
			"enabled": false,
		})
		return
	}

	// 返回完整憑據（用於自動連線）
	response.OK(c, config)
}
