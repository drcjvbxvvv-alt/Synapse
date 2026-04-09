package handlers

import (
	"encoding/json"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemSettingHandler 系統設定處理器
// P0-4c: db retained only for getMonitoringClusters; pending Wave 3 extraction.
type SystemSettingHandler struct {
	db                    *gorm.DB
	ldapService           *services.LDAPService
	sshSettingService     *services.SSHSettingService
	grafanaSettingService *services.GrafanaSettingService
	grafanaService        *services.GrafanaService
}

// NewSystemSettingHandler 建立系統設定處理器
func NewSystemSettingHandler(
	db *gorm.DB,
	ldapSvc *services.LDAPService,
	sshSvc *services.SSHSettingService,
	grafanaSettingSvc *services.GrafanaSettingService,
	grafanaSvc *services.GrafanaService,
) *SystemSettingHandler {
	return &SystemSettingHandler{
		db:                    db,
		ldapService:           ldapSvc,
		sshSettingService:     sshSvc,
		grafanaSettingService: grafanaSettingSvc,
		grafanaService:        grafanaSvc,
	}
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

// UpdateSSHConfigRequest SSH配置更新請求
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

// ==================== Grafana 配置相關介面 ====================

// GetGrafanaConfig 獲取 Grafana 配置
func (h *SystemSettingHandler) GetGrafanaConfig(c *gin.Context) {
	config, err := h.grafanaSettingService.GetGrafanaConfig()
	if err != nil {
		logger.Error("獲取 Grafana 配置失敗: %v", err)
		response.InternalError(c, "獲取 Grafana 配置失敗")
		return
	}

	safeConfig := *config
	if safeConfig.APIKey != "" {
		safeConfig.APIKey = "******"
	}

	response.OK(c, safeConfig)
}

// UpdateGrafanaConfigRequest Grafana 配置更新請求
type UpdateGrafanaConfigRequest struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// UpdateGrafanaConfig 更新 Grafana 配置
func (h *SystemSettingHandler) UpdateGrafanaConfig(c *gin.Context) {
	var req UpdateGrafanaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	existingConfig, err := h.grafanaSettingService.GetGrafanaConfig()
	if err != nil {
		logger.Error("獲取現有 Grafana 配置失敗: %v", err)
		response.InternalError(c, "更新 Grafana 配置失敗")
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
		logger.Error("儲存 Grafana 配置失敗: %v", err)
		response.InternalError(c, "儲存 Grafana 配置失敗")
		return
	}

	// 配置更新後，重新整理 GrafanaService 的連線參數
	if h.grafanaService != nil {
		h.grafanaService.UpdateConfig(config.URL, config.APIKey)
		logger.Info("Grafana 服務配置已熱更新", "url", config.URL)
	}

	logger.Info("Grafana 配置更新成功")

	response.OK(c, gin.H{"message": "Grafana 配置更新成功"})
}

// TestGrafanaConnection 測試 Grafana 連線
func (h *SystemSettingHandler) TestGrafanaConnection(c *gin.Context) {
	var req UpdateGrafanaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤")
		return
	}

	existingConfig, _ := h.grafanaSettingService.GetGrafanaConfig()

	apiKey := req.APIKey
	if (apiKey == "" || apiKey == "******") && existingConfig != nil {
		apiKey = existingConfig.APIKey
	}

	// 建立臨時 GrafanaService 用於測試連線
	testSvc := services.NewGrafanaService(req.URL, apiKey)
	if err := testSvc.TestConnection(); err != nil {
		logger.Warn("Grafana 連線測試失敗: %v", err)
		response.OK(c, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	logger.Info("Grafana 連線測試成功")

	response.OK(c, gin.H{
		"success": true,
	})
}

// GetGrafanaDashboardStatus 獲取 Dashboard 同步狀態
func (h *SystemSettingHandler) GetGrafanaDashboardStatus(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.OK(c, gin.H{
			"folder_exists": false,
			"dashboards":    []interface{}{},
			"all_synced":    false,
		})
		return
	}

	status, err := h.grafanaService.GetDashboardSyncStatus()
	if err != nil {
		logger.Error("獲取 Dashboard 同步狀態失敗: %v", err)
		response.InternalError(c, "獲取 Dashboard 同步狀態失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

// GetGrafanaDataSourceStatus 獲取資料來源同步狀態
func (h *SystemSettingHandler) GetGrafanaDataSourceStatus(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.OK(c, gin.H{
			"datasources": []interface{}{},
			"all_synced":  false,
		})
		return
	}

	clusters := h.getMonitoringClusters()
	status, err := h.grafanaService.GetDataSourceSyncStatus(clusters)
	if err != nil {
		response.InternalError(c, "獲取資料來源同步狀態失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

// SyncGrafanaDataSources 同步所有資料來源到 Grafana
func (h *SystemSettingHandler) SyncGrafanaDataSources(c *gin.Context) {
	if h.grafanaService == nil || !h.grafanaService.IsEnabled() {
		response.BadRequest(c, "請先配置 Grafana 連線資訊")
		return
	}

	clusters := h.getMonitoringClusters()
	if len(clusters) == 0 {
		response.OK(c, gin.H{
			"datasources": []interface{}{},
			"all_synced":  false,
		})
		return
	}

	status, err := h.grafanaService.SyncAllDataSources(clusters)
	if err != nil {
		response.InternalError(c, "同步資料來源失敗: "+err.Error())
		return
	}

	response.OK(c, status)
}

// getMonitoringClusters 獲取所有啟用了監控的叢集資訊
func (h *SystemSettingHandler) getMonitoringClusters() []services.DataSourceClusterInfo {
	var clusters []models.Cluster
	if err := h.db.Select("name, monitoring_config").Where("monitoring_config != '' AND monitoring_config IS NOT NULL").Find(&clusters).Error; err != nil {
		logger.Error("查詢叢集監控配置失敗", "error", err)
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
		response.BadRequest(c, "請先配置 Grafana 連線資訊")
		return
	}

	status, err := h.grafanaService.EnsureDashboards()
	if err != nil {
		logger.Error("同步 Dashboard 失敗: %v", err)
		response.InternalError(c, "同步 Dashboard 失敗: "+err.Error())
		return
	}

	logger.Info("Dashboard 同步完成", "all_synced", status.AllSynced)

	response.OK(c, status)
}
