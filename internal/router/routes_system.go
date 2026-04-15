package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerSystemRoutes registers audit, system-settings, permissions, AI, search,
// overview, cross-cluster workload, image, multicluster, and WebSocket-adjacent routes.
func registerSystemRoutes(protected *gin.RouterGroup, clusters *gin.RouterGroup, d *routeDeps) {
	// 資源治理全局視角（跨叢集彙總）
	globalResourceSvc := services.NewResourceService(d.db, d.k8sMgr, d.clusterSvc, d.prometheusSvc, d.monitoringCfgSvc)
	globalResourceHandler := handlers.NewResourceHandler(globalResourceSvc, d.clusterSvc)
	protected.GET("/resources/global/overview", globalResourceHandler.GetGlobalOverview)

	// 審批工作流（全域）（§8.3 Phase C）
	globalApprovalHandler := handlers.NewApprovalHandler(d.approvalSvc, d.clusterSvc)
	approvals := protected.Group("/approvals")
	{
		approvals.GET("", globalApprovalHandler.ListApprovalRequests)
		approvals.GET("/pending-count", globalApprovalHandler.GetPendingCount)
		approvals.PUT("/:id/approve", globalApprovalHandler.ApproveRequest)
		approvals.PUT("/:id/reject", globalApprovalHandler.RejectRequest)
	}

	// 跨叢集統一工作負載檢視（§8.3 Phase C）
	crossClusterHandler := handlers.NewCrossClusterHandler(d.clusterSvc, d.permissionSvc, d.k8sMgr)
	workloads := protected.Group("/workloads")
	{
		workloads.GET("", crossClusterHandler.ListCrossClusterWorkloads)
		workloads.GET("/stats", crossClusterHandler.GetCrossClusterStats)
	}

	// 映像索引與搜尋（§8.3 Phase C）
	imageHandler := handlers.NewImageHandler(d.imageIndexSvc, d.clusterSvc, d.permissionSvc, d.k8sMgr)
	images := protected.Group("/images")
	{
		images.GET("/search", imageHandler.SearchImages)
		images.GET("/status", imageHandler.GetImageSyncStatus)
		images.POST("/sync", imageHandler.SyncImages)
	}

	// 多叢集工作流程
	mcHandler := handlers.NewMultiClusterHandler(d.syncPolicySvc, d.clusterSvc, d.k8sMgr)
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
	helmGlobalHandler := handlers.NewHelmHandler(d.clusterSvc, d.helmSvc)
	helmGroup := protected.Group("/helm")
	{
		helmGroup.GET("/repos", helmGlobalHandler.ListRepos)
		helmGroup.POST("/repos", helmGlobalHandler.AddRepo)
		helmGroup.DELETE("/repos/:name", helmGlobalHandler.RemoveRepo)
		helmGroup.GET("/repos/charts", helmGlobalHandler.SearchCharts)
	}

	// overview - 總覽大盤
	overview := protected.Group("/overview")
	{
		alertManagerCfgSvc := services.NewAlertManagerConfigService(d.db)
		alertManagerSvc := services.NewAlertManagerService()
		overviewSvc := services.NewOverviewService(d.db, d.clusterSvc, d.k8sMgr, d.prometheusSvc, d.monitoringCfgSvc, alertManagerCfgSvc, alertManagerSvc)
		overviewHandler := handlers.NewOverviewHandler(overviewSvc, d.permissionSvc)
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
		searchHandler := handlers.NewSearchHandler(d.cfg, d.k8sMgr, d.clusterSvc, d.permissionSvc)
		search.GET("", searchHandler.GlobalSearch)
		search.GET("/quick", searchHandler.QuickSearch)
	}

	// audit - 審計管理（僅平臺管理員）
	audit := protected.Group("/audit")
	audit.Use(middleware.PlatformAdminRequired(d.db))
	{
		// 終端會話審計（保持不變）
		terminalAuditHandler := handlers.NewAuditHandler(d.cfg, d.auditSvc, d.opLogSvc)
		audit.GET("/terminal/sessions", terminalAuditHandler.GetTerminalSessions)
		audit.GET("/terminal/sessions/:sessionId", terminalAuditHandler.GetTerminalSession)
		audit.GET("/terminal/sessions/:sessionId/commands", terminalAuditHandler.GetTerminalCommands)
		audit.GET("/terminal/stats", terminalAuditHandler.GetTerminalStats)

		// 操作日誌審計（新增）
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

	// system settings - 系統設定（僅平臺管理員）
	systemSettings := protected.Group("/system")
	systemSettings.Use(middleware.PlatformAdminRequired(d.db))
	{
		ldapSvc := services.NewLDAPService(d.db)
		sshSvc := services.NewSSHSettingService(d.db)
		grafanaSettingSvc := services.NewGrafanaSettingService(d.db)
		systemSettingHandler := handlers.NewSystemSettingHandler(d.clusterSvc, ldapSvc, sshSvc, grafanaSettingSvc, d.grafanaSvc)
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
		siemSvc := services.NewSIEMService(d.db)
		siemHandler := handlers.NewSIEMHandler(siemSvc)
		systemSettings.GET("/siem/config", siemHandler.GetSIEMConfig)
		systemSettings.PUT("/siem/config", siemHandler.UpdateSIEMConfig)
		systemSettings.POST("/siem/test", siemHandler.TestSIEMWebhook)
		// 安全設定（登入安全參數）
		sysSecuritySvc := services.NewSystemSecurityService(d.db)
		sysSecurityHandler := handlers.NewSystemSecurityHandler(sysSecuritySvc)
		systemSettings.GET("/security/config", sysSecurityHandler.GetSecurityConfig)
		systemSettings.PUT("/security/config", sysSecurityHandler.UpdateSecurityConfig)
		// 通知渠道管理
		notifyChannelSvc := services.NewNotifyChannelService(d.db)
		notifyChannelHandler := handlers.NewNotifyChannelHandler(notifyChannelSvc)
		systemSettings.GET("/notify-channels", notifyChannelHandler.ListNotifyChannels)
		systemSettings.POST("/notify-channels", notifyChannelHandler.CreateNotifyChannel)
		systemSettings.PUT("/notify-channels/:id", notifyChannelHandler.UpdateNotifyChannel)
		systemSettings.DELETE("/notify-channels/:id", notifyChannelHandler.DeleteNotifyChannel)
		systemSettings.POST("/notify-channels/:id/test", notifyChannelHandler.TestNotifyChannel)
		// Feature Flag 管理（P2-6）
		ffHandler := handlers.NewFeatureFlagHandler(d.featureFlagSvc, d.featureDBStore)
		systemSettings.GET("/feature-flags", ffHandler.List)
		systemSettings.PUT("/feature-flags/:key", ffHandler.Update)

		// Git Provider 管理（M14）
		gitProviderHandler := handlers.NewGitProviderHandler(d.gitProviderSvc)
		gitProviders := systemSettings.Group("/git-providers")
		{
			gitProviders.GET("", gitProviderHandler.List)
			gitProviders.GET("/:id", gitProviderHandler.Get)
			gitProviders.POST("", gitProviderHandler.Create)
			gitProviders.PUT("/:id", gitProviderHandler.Update)
			gitProviders.DELETE("/:id", gitProviderHandler.Delete)
			gitProviders.POST("/:id/regenerate-token", gitProviderHandler.RegenerateToken)
		}
	}

	// 消息通知（全域，跨叢集 Event Alert History）
	notificationSvc := services.NewNotificationService(d.db)
	notifyHandler := handlers.NewNotificationHandler(notificationSvc)
	notifications := protected.Group("/notifications")
	{
		notifications.GET("", notifyHandler.ListNotifications)
		notifications.GET("/unread-count", notifyHandler.UnreadCount)
		notifications.PUT("/read-all", notifyHandler.MarkAllRead)
		notifications.PUT("/:id/read", notifyHandler.MarkRead)
	}

	// API Token 管理（任意已登入使用者，非 PlatformAdmin Only）
	sysSecuritySvc := services.NewSystemSecurityService(d.db)
	sysSecurityHandler := handlers.NewSystemSecurityHandler(sysSecuritySvc)
	userTokens := protected.Group("/users/me/tokens")
	{
		userTokens.GET("", sysSecurityHandler.ListAPITokens)
		userTokens.POST("", sysSecurityHandler.CreateAPIToken)
		userTokens.DELETE("/:id", sysSecurityHandler.DeleteAPIToken)
	}

	// Port-Forward 會話管理（全域，§8.3 Phase D）
	pfGlobalHandler := handlers.NewPortForwardHandler(d.portForwardSvc, d.clusterSvc, d.k8sMgr)
	portforwards := protected.Group("/portforwards")
	{
		portforwards.GET("", pfGlobalHandler.ListPortForwards)
		portforwards.DELETE("/:sessionId", pfGlobalHandler.StopPortForward)
	}

	// 稽核日誌 JSON 匯出（§8.3 Phase D）
	siemExportHandler := handlers.NewSIEMHandler(services.NewSIEMService(d.db))
	protected.GET("/audit/export", siemExportHandler.ExportAuditLogs)

	// permissions - 權限管理
	globalRbacSvc := services.NewRBACService()
	permissionHandler := handlers.NewPermissionHandler(d.permissionSvc, d.clusterSvc, globalRbacSvc)
	globalRbacHandler := handlers.NewRBACHandler(d.clusterSvc, globalRbacSvc, d.k8sMgr)
	permissions := protected.Group("/permissions")
	{
		// 當前使用者權限查詢（任意登入使用者可訪問）
		permissions.GET("/my-permissions", permissionHandler.GetMyPermissions)
		permissions.GET("/types", permissionHandler.GetPermissionTypes)

		// 以下介面需要平臺管理員權限
		permAdmin := permissions.Group("")
		permAdmin.Use(middleware.PlatformAdminRequired(d.db))
		{
			// Synapse 預定義 ClusterRole 資訊
			permAdmin.GET("/synapse-roles", globalRbacHandler.GetSynapseClusterRoles)

			// 使用者列表（用於權限分配）
			permAdmin.GET("/users", permissionHandler.ListUsers)

			// 使用者組管理
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

			// 叢集權限管理
			clusterPerms := permAdmin.Group("/cluster-permissions")
			{
				clusterPerms.GET("", permissionHandler.ListAllClusterPermissions)
				clusterPerms.POST("", permissionHandler.CreateClusterPermission)
				clusterPerms.GET("/:id", permissionHandler.GetClusterPermission)
				clusterPerms.PUT("/:id", permissionHandler.UpdateClusterPermission)
				clusterPerms.DELETE("/:id", permissionHandler.DeleteClusterPermission)
				clusterPerms.POST("/batch-delete", permissionHandler.BatchDeleteClusterPermissions)

				// 功能開關策略
				clusterPerms.GET("/:id/features", permissionHandler.GetFeaturePolicy)
				clusterPerms.PATCH("/:id/features", permissionHandler.UpdateFeaturePolicy)
			}
		}
	}

	// 叢集級權限查詢
	protected.GET("/clusters/:clusterID/my-permissions", permissionHandler.GetMyClusterPermission)

	registerSystemAIRoutes(protected, clusters, d)
}
