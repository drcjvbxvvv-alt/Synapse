package router

import (
	"github.com/clay-wangzhi/Synapse/internal/handlers"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerSystemRoutes registers audit, system-settings, permissions, AI, search,
// overview, cross-cluster workload, image, multicluster, and WebSocket-adjacent routes.
func registerSystemRoutes(protected *gin.RouterGroup, clusters *gin.RouterGroup, d *routeDeps) {
	// 審批工作流（全域）（§8.3 Phase C）
	globalApprovalHandler := handlers.NewApprovalHandler(d.db, d.clusterSvc)
	approvals := protected.Group("/approvals")
	{
		approvals.GET("", globalApprovalHandler.ListApprovalRequests)
		approvals.GET("/pending-count", globalApprovalHandler.GetPendingCount)
		approvals.PUT("/:id/approve", globalApprovalHandler.ApproveRequest)
		approvals.PUT("/:id/reject", globalApprovalHandler.RejectRequest)
	}

	// 跨叢集統一工作負載視圖（§8.3 Phase C）
	crossClusterHandler := handlers.NewCrossClusterHandler(d.db, d.clusterSvc, d.permissionSvc, d.k8sMgr)
	workloads := protected.Group("/workloads")
	{
		workloads.GET("", crossClusterHandler.ListCrossClusterWorkloads)
		workloads.GET("/stats", crossClusterHandler.GetCrossClusterStats)
	}

	// 映像索引與搜尋（§8.3 Phase C）
	imageHandler := handlers.NewImageHandler(d.db, d.clusterSvc, d.permissionSvc, d.k8sMgr)
	images := protected.Group("/images")
	{
		images.GET("/search", imageHandler.SearchImages)
		images.GET("/status", imageHandler.GetImageSyncStatus)
		images.POST("/sync", imageHandler.SyncImages)
	}

	// 多叢集工作流程
	mcHandler := handlers.NewMultiClusterHandler(d.db, d.clusterSvc, d.k8sMgr)
	mc := protected.Group("/multicluster")
	{
		mc.POST("/migrate/check", mcHandler.MigrateCheck)
		mc.POST("/migrate", mcHandler.Migrate)
		mc.GET("/sync-policies", mcHandler.ListSyncPolicies)
		mc.POST("/sync-policies", mcHandler.CreateSyncPolicy)
		mc.GET("/sync-policies/:id", mcHandler.GetSyncPolicy)
		mc.PUT("/sync-policies/:id", mcHandler.UpdateSyncPolicy)
		mc.DELETE("/sync-policies/:id", mcHandler.DeleteSyncPolicy)
		mc.POST("/sync-policies/:id/trigger", mcHandler.TriggerSync)
		mc.GET("/sync-policies/:id/history", mcHandler.GetSyncHistory)
	}

	// Helm Repository 管理（global）
	helmGlobalHandler := handlers.NewHelmHandler(d.clusterSvc, d.db)
	helmGroup := protected.Group("/helm")
	{
		helmGroup.GET("/repos", helmGlobalHandler.ListRepos)
		helmGroup.POST("/repos", helmGlobalHandler.AddRepo)
		helmGroup.DELETE("/repos/:name", helmGlobalHandler.RemoveRepo)
		helmGroup.GET("/repos/charts", helmGlobalHandler.SearchCharts)
	}

	// overview - 总览大盘
	overview := protected.Group("/overview")
	{
		alertManagerCfgSvc := services.NewAlertManagerConfigService(d.db)
		alertManagerSvc := services.NewAlertManagerService()
		overviewHandler := handlers.NewOverviewHandler(d.db, d.clusterSvc, d.k8sMgr, d.prometheusSvc, d.monitoringCfgSvc, alertManagerCfgSvc, alertManagerSvc, d.permissionSvc)
		overview.GET("/stats", overviewHandler.GetStats)
		overview.GET("/resource-usage", overviewHandler.GetResourceUsage)
		overview.GET("/distribution", overviewHandler.GetDistribution)
		overview.GET("/trends", overviewHandler.GetTrends)
		overview.GET("/abnormal-workloads", overviewHandler.GetAbnormalWorkloads)
		overview.GET("/alert-stats", overviewHandler.GetAlertStats)
	}

	// search
	search := protected.Group("/search")
	{
		searchHandler := handlers.NewSearchHandler(d.db, d.cfg, d.k8sMgr, d.clusterSvc, d.permissionSvc)
		search.GET("", searchHandler.GlobalSearch)
		search.GET("/quick", searchHandler.QuickSearch)
	}

	// audit - 审计管理（仅平台管理员）
	audit := protected.Group("/audit")
	audit.Use(middleware.PlatformAdminRequired(d.db))
	{
		// 终端会话审计（保持不变）
		terminalAuditHandler := handlers.NewAuditHandler(d.db, d.cfg)
		audit.GET("/terminal/sessions", terminalAuditHandler.GetTerminalSessions)
		audit.GET("/terminal/sessions/:sessionId", terminalAuditHandler.GetTerminalSession)
		audit.GET("/terminal/sessions/:sessionId/commands", terminalAuditHandler.GetTerminalCommands)
		audit.GET("/terminal/stats", terminalAuditHandler.GetTerminalStats)

		// 操作日志审计（新增）
		opLogHandler := handlers.NewOperationLogHandler(d.opLogSvc)
		audit.GET("/operations", opLogHandler.GetOperationLogs)
		audit.GET("/operations/:id", opLogHandler.GetOperationLog)
		audit.GET("/operations/stats", opLogHandler.GetOperationLogStats)
		audit.GET("/modules", opLogHandler.GetModules)
		audit.GET("/actions", opLogHandler.GetActions)
	}

	// monitoring templates
	monitoringHandler := handlers.NewMonitoringHandler(d.monitoringCfgSvc, d.prometheusSvc)
	protected.GET("/monitoring/templates", monitoringHandler.GetMonitoringTemplates)

	// system settings - 系统设置（仅平台管理员）
	systemSettings := protected.Group("/system")
	systemSettings.Use(middleware.PlatformAdminRequired(d.db))
	{
		systemSettingHandler := handlers.NewSystemSettingHandler(d.db, d.grafanaSvc)
		// LDAP 配置
		systemSettings.GET("/ldap/config", systemSettingHandler.GetLDAPConfig)
		systemSettings.PUT("/ldap/config", systemSettingHandler.UpdateLDAPConfig)
		systemSettings.POST("/ldap/test-connection", systemSettingHandler.TestLDAPConnection)
		systemSettings.POST("/ldap/test-auth", systemSettingHandler.TestLDAPAuth)
		// SSH 配置
		systemSettings.GET("/ssh/config", systemSettingHandler.GetSSHConfig)
		systemSettings.PUT("/ssh/config", systemSettingHandler.UpdateSSHConfig)
		systemSettings.GET("/ssh/credentials", systemSettingHandler.GetSSHCredentials)
		// Grafana 配置
		systemSettings.GET("/grafana/config", systemSettingHandler.GetGrafanaConfig)
		systemSettings.PUT("/grafana/config", systemSettingHandler.UpdateGrafanaConfig)
		systemSettings.POST("/grafana/test-connection", systemSettingHandler.TestGrafanaConnection)
		systemSettings.GET("/grafana/dashboard-status", systemSettingHandler.GetGrafanaDashboardStatus)
		systemSettings.POST("/grafana/sync-dashboards", systemSettingHandler.SyncGrafanaDashboards)
		systemSettings.GET("/grafana/datasource-status", systemSettingHandler.GetGrafanaDataSourceStatus)
		systemSettings.POST("/grafana/sync-datasources", systemSettingHandler.SyncGrafanaDataSources)
		// SIEM Webhook 匯出設定（§8.3 Phase D）
		siemHandler := handlers.NewSIEMHandler(d.db)
		systemSettings.GET("/siem/config", siemHandler.GetSIEMConfig)
		systemSettings.PUT("/siem/config", siemHandler.UpdateSIEMConfig)
		systemSettings.POST("/siem/test", siemHandler.TestSIEMWebhook)
		// 安全設定（登入安全參數）
		sysSecurityHandler := handlers.NewSystemSecurityHandler(d.db)
		systemSettings.GET("/security/config", sysSecurityHandler.GetSecurityConfig)
		systemSettings.PUT("/security/config", sysSecurityHandler.UpdateSecurityConfig)
		// 通知渠道管理
		notifyChannelHandler := handlers.NewNotifyChannelHandler(d.db)
		systemSettings.GET("/notify-channels", notifyChannelHandler.ListNotifyChannels)
		systemSettings.POST("/notify-channels", notifyChannelHandler.CreateNotifyChannel)
		systemSettings.PUT("/notify-channels/:id", notifyChannelHandler.UpdateNotifyChannel)
		systemSettings.DELETE("/notify-channels/:id", notifyChannelHandler.DeleteNotifyChannel)
		systemSettings.POST("/notify-channels/:id/test", notifyChannelHandler.TestNotifyChannel)
	}

	// API Token 管理（任意已登入使用者，非 PlatformAdmin Only）
	sysSecurityHandler := handlers.NewSystemSecurityHandler(d.db)
	userTokens := protected.Group("/users/me/tokens")
	{
		userTokens.GET("", sysSecurityHandler.ListAPITokens)
		userTokens.POST("", sysSecurityHandler.CreateAPIToken)
		userTokens.DELETE("/:id", sysSecurityHandler.DeleteAPIToken)
	}

	// Port-Forward 會話管理（全域，§8.3 Phase D）
	pfGlobalHandler := handlers.NewPortForwardHandler(d.db, d.clusterSvc, d.k8sMgr)
	portforwards := protected.Group("/portforwards")
	{
		portforwards.GET("", pfGlobalHandler.ListPortForwards)
		portforwards.DELETE("/:sessionId", pfGlobalHandler.StopPortForward)
	}

	// 稽核日誌 JSON 匯出（§8.3 Phase D）
	siemExportHandler := handlers.NewSIEMHandler(d.db)
	protected.GET("/audit/export", siemExportHandler.ExportAuditLogs)

	// permissions - 权限管理
	globalRbacSvc := services.NewRBACService()
	permissionHandler := handlers.NewPermissionHandler(d.permissionSvc, d.clusterSvc, globalRbacSvc)
	globalRbacHandler := handlers.NewRBACHandler(d.clusterSvc, globalRbacSvc, d.k8sMgr)
	permissions := protected.Group("/permissions")
	{
		// 当前用户权限查询（任意登录用户可访问）
		permissions.GET("/my-permissions", permissionHandler.GetMyPermissions)
		permissions.GET("/types", permissionHandler.GetPermissionTypes)

		// 以下接口需要平台管理员权限
		permAdmin := permissions.Group("")
		permAdmin.Use(middleware.PlatformAdminRequired(d.db))
		{
			// Synapse 预定义 ClusterRole 信息
			permAdmin.GET("/synapse-roles", globalRbacHandler.GetSynapseClusterRoles)

			// 用户列表（用于权限分配）
			permAdmin.GET("/users", permissionHandler.ListUsers)

			// 用户组管理
			userGroups := permAdmin.Group("/user-groups")
			{
				userGroups.GET("", permissionHandler.ListUserGroups)
				userGroups.POST("", permissionHandler.CreateUserGroup)
				userGroups.GET("/:id", permissionHandler.GetUserGroup)
				userGroups.PUT("/:id", permissionHandler.UpdateUserGroup)
				userGroups.DELETE("/:id", permissionHandler.DeleteUserGroup)
				userGroups.POST("/:id/users", permissionHandler.AddUserToGroup)
				userGroups.DELETE("/:id/users/:userId", permissionHandler.RemoveUserFromGroup)
			}

			// 集群权限管理
			clusterPerms := permAdmin.Group("/cluster-permissions")
			{
				clusterPerms.GET("", permissionHandler.ListAllClusterPermissions)
				clusterPerms.POST("", permissionHandler.CreateClusterPermission)
				clusterPerms.GET("/:id", permissionHandler.GetClusterPermission)
				clusterPerms.PUT("/:id", permissionHandler.UpdateClusterPermission)
				clusterPerms.DELETE("/:id", permissionHandler.DeleteClusterPermission)
				clusterPerms.POST("/batch-delete", permissionHandler.BatchDeleteClusterPermissions)
			}
		}
	}

	// 集群级权限查询
	protected.GET("/clusters/:clusterID/my-permissions", permissionHandler.GetMyClusterPermission)

	// AI 配置管理（僅平台管理員）
	aiConfigHandler := handlers.NewAIConfigHandler(d.db)
	aiGroup := protected.Group("/ai")
	{
		aiAdminGroup := aiGroup.Group("")
		aiAdminGroup.Use(middleware.PlatformAdminRequired(d.db))
		aiAdminGroup.GET("/config", aiConfigHandler.GetConfig)
		aiAdminGroup.PUT("/config", aiConfigHandler.UpdateConfig)
		aiAdminGroup.POST("/test-connection", aiConfigHandler.TestConnection)

		// Runbook 知識庫（所有已登入使用者）
		aiRunbookHandler := handlers.NewAIRunbookHandler()
		aiGroup.GET("/runbooks", aiRunbookHandler.GetRunbooks)
	}

	// AI Chat + NL Query（叢集級）
	aiChatHandler := handlers.NewAIChatHandler(d.db, d.clusterSvc, d.k8sMgr)
	aiNLQueryHandler := handlers.NewAINLQueryHandler(d.db, d.clusterSvc, d.k8sMgr)
	aiChat := clusters.Group("/:clusterID/ai")
	aiChat.Use(d.permMiddleware.ClusterAccessRequired())
	{
		aiChat.POST("/chat", aiChatHandler.Chat)
		aiChat.POST("/nl-query", aiNLQueryHandler.NLQuery)
	}

	// Security Scanning（叢集級）
	securityHandler := handlers.NewSecurityHandler(d.db, d.k8sMgr)
	securityGroup := clusters.Group("/:clusterID/security")
	securityGroup.Use(d.permMiddleware.ClusterAccessRequired())
	{
		// Trivy image scanning
		securityGroup.POST("/scans", securityHandler.TriggerScan)
		securityGroup.GET("/scans", securityHandler.GetScanResults)
		securityGroup.GET("/scans/:scanID", securityHandler.GetScanDetail)
		// CIS kube-bench
		securityGroup.POST("/bench", securityHandler.TriggerBenchmark)
		securityGroup.GET("/bench", securityHandler.GetBenchResults)
		securityGroup.GET("/bench/:benchID", securityHandler.GetBenchDetail)
		// Gatekeeper / OPA violations
		securityGroup.GET("/gatekeeper", securityHandler.GetGatekeeperViolations)
	}
}
