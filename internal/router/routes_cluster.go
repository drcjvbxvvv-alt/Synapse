package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerClusterRoutes registers all /clusters routes onto the protected group.
// Domain-specific sub-groups are delegated to register*Routes helpers.
func registerClusterRoutes(protected *gin.RouterGroup, d *routeDeps) {
	clusters := protected.Group("/clusters")
	{
		clusterHandler := handlers.NewClusterHandler(d.cfg, d.k8sMgr, d.clusterSvc, d.prometheusSvc, d.monitoringCfgSvc, d.permissionSvc)

		clusters.GET("", clusterHandler.GetClusters)
		clusters.GET("/stats", clusterHandler.GetClusterStats)
		clusters.POST("/import", middleware.PlatformAdminRequired(d.db), clusterHandler.ImportCluster)
		clusters.POST("/test-connection", middleware.PlatformAdminRequired(d.db), clusterHandler.TestConnection)

		cluster := clusters.Group("/:clusterID")
		cluster.Use(d.permMiddleware.ClusterAccessRequired())
		cluster.Use(d.permMiddleware.AutoWriteCheck())
		{
			cluster.GET("", clusterHandler.GetCluster)
			cluster.GET("/status", clusterHandler.GetClusterStatus)
			cluster.GET("/overview", clusterHandler.GetClusterOverview)
			cluster.GET("/metrics", clusterHandler.GetClusterMetrics)
			cluster.GET("/events", clusterHandler.GetClusterEvents)
			cluster.DELETE("", clusterHandler.DeleteCluster)

			// namespaces
			namespaceHandler := handlers.NewNamespaceHandler(d.clusterSvc, d.k8sMgr)
			namespaces := cluster.Group("/namespaces")
			{
				namespaces.GET("", namespaceHandler.GetNamespaces)
				namespaces.GET("/:namespace", namespaceHandler.GetNamespaceDetail)
				namespaces.POST("", namespaceHandler.CreateNamespace)
				namespaces.DELETE("/:namespace", namespaceHandler.DeleteNamespace)
				namespaces.GET("/:namespace/quotas", namespaceHandler.ListResourceQuotas)
				namespaces.POST("/:namespace/quotas", namespaceHandler.CreateResourceQuota)
				namespaces.PUT("/:namespace/quotas/:name", namespaceHandler.UpdateResourceQuota)
				namespaces.DELETE("/:namespace/quotas/:name", namespaceHandler.DeleteResourceQuota)
				namespaces.GET("/:namespace/limitranges", namespaceHandler.ListLimitRanges)
				namespaces.POST("/:namespace/limitranges", namespaceHandler.CreateLimitRange)
				namespaces.PUT("/:namespace/limitranges/:name", namespaceHandler.UpdateLimitRange)
				namespaces.DELETE("/:namespace/limitranges/:name", namespaceHandler.DeleteLimitRange)
			}

			// monitoring config
			monitoringHandler := handlers.NewMonitoringHandler(d.monitoringCfgSvc, d.prometheusSvc)
			monitoring := cluster.Group("/monitoring")
			{
				monitoring.GET("/config", monitoringHandler.GetMonitoringConfig)
				monitoring.PUT("/config", monitoringHandler.UpdateMonitoringConfig)
				monitoring.POST("/test-connection", monitoringHandler.TestMonitoringConnection)
				monitoring.GET("/metrics", monitoringHandler.GetClusterMetrics)
			}

			// alertmanager
			alertManagerConfigSvc := services.NewAlertManagerConfigService(d.db)
			alertManagerSvc := services.NewAlertManagerService()
			alertHandler := handlers.NewAlertHandler(alertManagerConfigSvc, alertManagerSvc, d.k8sMgr, d.clusterSvc)
			alertmanager := cluster.Group("/alertmanager")
			{
				alertmanager.GET("/config", alertHandler.GetAlertManagerConfig)
				alertmanager.PUT("/config", alertHandler.UpdateAlertManagerConfig)
				alertmanager.POST("/test-connection", alertHandler.TestAlertManagerConnection)
				alertmanager.GET("/status", alertHandler.GetAlertManagerStatus)
				alertmanager.GET("/template", alertHandler.GetAlertManagerConfigTemplate)
			}
			alerts := cluster.Group("/alerts")
			{
				alerts.GET("", alertHandler.GetAlerts)
				alerts.GET("/groups", alertHandler.GetAlertGroups)
				alerts.GET("/stats", alertHandler.GetAlertStats)
			}
			silences := cluster.Group("/silences")
			{
				silences.GET("", alertHandler.GetSilences)
				silences.POST("", alertHandler.CreateSilence)
				silences.DELETE("/:silenceId", alertHandler.DeleteSilence)
			}
			receivers := cluster.Group("/receivers")
			{
				receivers.GET("", alertHandler.GetReceivers)
				receivers.GET("/full", alertHandler.GetFullReceivers)
				receivers.POST("", alertHandler.CreateReceiver)
				receivers.PUT("/:name", alertHandler.UpdateReceiver)
				receivers.DELETE("/:name", alertHandler.DeleteReceiver)
				receivers.POST("/:name/test", alertHandler.TestReceiver)
			}

			// domain-specific sub-groups
			registerClusterWorkloadRoutes(cluster, d)
			registerClusterConfigRoutes(cluster, d)
			registerClusterNetworkingRoutes(cluster, d)
			registerClusterStorageRoutes(cluster, d)
			registerClusterGitopsRoutes(cluster, d)
			registerClusterOpsRoutes(cluster, d)
			registerClusterGovernanceRoutes(cluster, d)
			registerClusterResilienceRoutes(cluster, d)
			registerClusterSLORoutes(cluster, d)
		registerClusterChaosRoutes(cluster, d)
		registerClusterComplianceRoutes(cluster, d)
		// Pipeline routes moved to top-level /pipelines (CICD_PIPELINE_REDESIGN Sprint 4 R3)
		}
	}
}
