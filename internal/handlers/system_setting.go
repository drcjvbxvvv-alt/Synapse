package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/clay-wangzhi/KubePolaris/internal/models"
	"github.com/clay-wangzhi/KubePolaris/internal/services"
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemSettingHandler 系统设置处理器
type SystemSettingHandler struct {
	db                    *gorm.DB
	ldapService           *services.LDAPService
	sshSettingService     *services.SSHSettingService
	grafanaSettingService *services.GrafanaSettingService
	grafanaService        *services.GrafanaService
}

// NewSystemSettingHandler 创建系统设置处理器
func NewSystemSettingHandler(db *gorm.DB, grafanaService *services.GrafanaService) *SystemSettingHandler {
	return &SystemSettingHandler{
		db:                    db,
		ldapService:           services.NewLDAPService(db),
		sshSettingService:     services.NewSSHSettingService(db),
		grafanaSettingService: services.NewGrafanaSettingService(db),
		grafanaService:        grafanaService,
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

// ==================== SSH 配置相关接口 ====================

// GetSSHConfig 获取SSH配置
func (h *SystemSettingHandler) GetSSHConfig(c *gin.Context) {
	config, err := h.sshSettingService.GetSSHConfig()
	if err != nil {
		logger.Error("获取SSH配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取SSH配置失败",
			"data":    nil,
		})
		return
	}

	// 返回配置时隐藏敏感信息
	safeConfig := *config
	if safeConfig.Password != "" {
		safeConfig.Password = "******"
	}
	if safeConfig.PrivateKey != "" {
		safeConfig.PrivateKey = "******"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    safeConfig,
	})
}

// UpdateSSHConfigRequest SSH配置更新请求
type UpdateSSHConfigRequest struct {
	Enabled    bool   `json:"enabled"`
	Username   string `json:"username"`
	Port       int    `json:"port"`
	AuthType   string `json:"auth_type"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

// UpdateSSHConfig 更新SSH配置
func (h *SystemSettingHandler) UpdateSSHConfig(c *gin.Context) {
	var req UpdateSSHConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"data":    nil,
		})
		return
	}

	// 获取现有配置
	existingConfig, err := h.sshSettingService.GetSSHConfig()
	if err != nil {
		logger.Error("获取现有SSH配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新SSH配置失败",
			"data":    nil,
		})
		return
	}

	// 构建新配置
	config := &models.SSHConfig{
		Enabled:  req.Enabled,
		Username: req.Username,
		Port:     req.Port,
		AuthType: req.AuthType,
	}

	// 设置默认值
	if config.Username == "" {
		config.Username = "root"
	}
	if config.Port == 0 {
		config.Port = 22
	}
	if config.AuthType == "" {
		config.AuthType = "password"
	}

	// 如果密码是占位符或空，保留原密码
	if req.Password != "" && req.Password != "******" {
		config.Password = req.Password
	} else {
		config.Password = existingConfig.Password
	}

	// 如果私钥是占位符或空，保留原私钥
	if req.PrivateKey != "" && req.PrivateKey != "******" {
		config.PrivateKey = req.PrivateKey
	} else {
		config.PrivateKey = existingConfig.PrivateKey
	}

	// 保存配置
	if err := h.sshSettingService.SaveSSHConfig(config); err != nil {
		logger.Error("保存SSH配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存SSH配置失败",
			"data":    nil,
		})
		return
	}

	logger.Info("SSH配置更新成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "SSH配置更新成功",
		"data":    nil,
	})
}

// GetSSHCredentials 获取SSH凭据（用于自动连接，返回完整凭据）
func (h *SystemSettingHandler) GetSSHCredentials(c *gin.Context) {
	config, err := h.sshSettingService.GetSSHConfig()
	if err != nil {
		logger.Error("获取SSH凭据失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取SSH凭据失败",
			"data":    nil,
		})
		return
	}

	// 检查是否启用
	if !config.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "SSH全局配置未启用",
			"data": gin.H{
				"enabled": false,
			},
		})
		return
	}

	// 返回完整凭据（用于自动连接）
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    config,
	})
}

// ==================== Grafana 配置相关接口 ====================

// GetGrafanaConfig 获取 Grafana 配置
func (h *SystemSettingHandler) GetGrafanaConfig(c *gin.Context) {
	config, err := h.grafanaSettingService.GetGrafanaConfig()
	if err != nil {
		logger.Error("获取 Grafana 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取 Grafana 配置失败",
			"data":    nil,
		})
		return
	}

	safeConfig := *config
	if safeConfig.APIKey != "" {
		safeConfig.APIKey = "******"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    safeConfig,
	})
}

// UpdateGrafanaConfigRequest Grafana 配置更新请求
type UpdateGrafanaConfigRequest struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// UpdateGrafanaConfig 更新 Grafana 配置
func (h *SystemSettingHandler) UpdateGrafanaConfig(c *gin.Context) {
	var req UpdateGrafanaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"data":    nil,
		})
		return
	}

	existingConfig, err := h.grafanaSettingService.GetGrafanaConfig()
	if err != nil {
		logger.Error("获取现有 Grafana 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新 Grafana 配置失败",
			"data":    nil,
		})
		return
	}

	config := &models.GrafanaSettingConfig{
		URL: req.URL,
	}

	if req.APIKey != "" && req.APIKey != "******" {
		config.APIKey = req.APIKey
	} else {
		config.APIKey = existingConfig.APIKey
	}

	if err := h.grafanaSettingService.SaveGrafanaConfig(config); err != nil {
		logger.Error("保存 Grafana 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存 Grafana 配置失败",
			"data":    nil,
		})
		return
	}

	// 配置更新后，刷新 GrafanaService 的连接参数
	if h.grafanaService != nil {
		h.grafanaService.UpdateConfig(config.URL, config.APIKey)
		logger.Info("Grafana 服务配置已热更新", "url", config.URL)
	}

	logger.Info("Grafana 配置更新成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Grafana 配置更新成功",
		"data":    nil,
	})
}

// TestGrafanaConnection 测试 Grafana 连接
func (h *SystemSettingHandler) TestGrafanaConnection(c *gin.Context) {
	var req UpdateGrafanaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"data":    nil,
		})
		return
	}

	existingConfig, _ := h.grafanaSettingService.GetGrafanaConfig()

	apiKey := req.APIKey
	if (apiKey == "" || apiKey == "******") && existingConfig != nil {
		apiKey = existingConfig.APIKey
	}

	// 创建临时 GrafanaService 用于测试连接
	testSvc := services.NewGrafanaService(req.URL, apiKey)
	if err := testSvc.TestConnection(); err != nil {
		logger.Warn("Grafana 连接测试失败: %v", err)
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

	logger.Info("Grafana 连接测试成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Grafana 连接测试成功",
		"data": gin.H{
			"success": true,
		},
	})
}

// GetGrafanaDashboardStatus 获取 Dashboard 同步状态
func (h *SystemSettingHandler) GetGrafanaDashboardStatus(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "Grafana 未配置",
			"data": gin.H{
				"folder_exists": false,
				"dashboards":    []interface{}{},
				"all_synced":    false,
			},
		})
		return
	}

	status, err := h.grafanaService.GetDashboardSyncStatus()
	if err != nil {
		logger.Error("获取 Dashboard 同步状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取 Dashboard 同步状态失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    status,
	})
}

// GetGrafanaDataSourceStatus 获取数据源同步状态
func (h *SystemSettingHandler) GetGrafanaDataSourceStatus(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "Grafana 未配置",
			"data": gin.H{
				"datasources": []interface{}{},
				"all_synced":  false,
			},
		})
		return
	}

	clusters := h.getMonitoringClusters()
	status, err := h.grafanaService.GetDataSourceSyncStatus(clusters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取数据源同步状态失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    status,
	})
}

// SyncGrafanaDataSources 同步所有数据源到 Grafana
func (h *SystemSettingHandler) SyncGrafanaDataSources(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请先配置 Grafana 连接信息",
			"data":    nil,
		})
		return
	}

	clusters := h.getMonitoringClusters()
	if len(clusters) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "没有已配置监控的集群",
			"data": gin.H{
				"datasources": []interface{}{},
				"all_synced":  false,
			},
		})
		return
	}

	status, err := h.grafanaService.SyncAllDataSources(clusters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "同步数据源失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	msg := "数据源同步成功"
	if !status.AllSynced {
		msg = "部分数据源同步失败，请检查日志"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": msg,
		"data":    status,
	})
}

// getMonitoringClusters 获取所有启用了监控的集群信息
func (h *SystemSettingHandler) getMonitoringClusters() []services.DataSourceClusterInfo {
	var clusters []models.Cluster
	if err := h.db.Select("name, monitoring_config").Where("monitoring_config != '' AND monitoring_config IS NOT NULL").Find(&clusters).Error; err != nil {
		logger.Error("查询集群监控配置失败", "error", err)
		return nil
	}

	var result []services.DataSourceClusterInfo
	for _, cluster := range clusters {
		var config models.MonitoringConfig
		if err := json.Unmarshal([]byte(cluster.MonitoringConfig), &config); err != nil {
			continue
		}
		if config.Type == "disabled" || config.Endpoint == "" {
			continue
		}
		result = append(result, services.DataSourceClusterInfo{
			ClusterName:   cluster.Name,
			PrometheusURL: config.Endpoint,
		})
	}
	return result
}

// SyncGrafanaDashboards 同步 Dashboard 到 Grafana
func (h *SystemSettingHandler) SyncGrafanaDashboards(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请先配置 Grafana 连接信息",
			"data":    nil,
		})
		return
	}

	status, err := h.grafanaService.EnsureDashboards()
	if err != nil {
		logger.Error("同步 Dashboard 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "同步 Dashboard 失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	logger.Info("Dashboard 同步完成", "all_synced", status.AllSynced)

	msg := "Dashboard 同步成功"
	if !status.AllSynced {
		msg = "部分 Dashboard 同步失败，请检查日志"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": msg,
		"data":    status,
	})
}
