package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerSystemAIRoutes registers AI configuration, chat, NL query, and security scanning routes.
func registerSystemAIRoutes(protected *gin.RouterGroup, clusters *gin.RouterGroup, d *routeDeps) {
	aiConfigSvc := services.NewAIConfigService(d.db)
	aiConfigHandler := handlers.NewAIConfigHandler(aiConfigSvc)
	aiGroup := protected.Group("/ai")
	{
		aiAdminGroup := aiGroup.Group("")
		aiAdminGroup.Use(middleware.PlatformAdminRequired(d.db))
		aiAdminGroup.GET("/config", aiConfigHandler.GetConfig)
		aiAdminGroup.PUT("/config", aiConfigHandler.UpdateConfig)
		aiAdminGroup.POST("/test-connection", aiConfigHandler.TestConnection)

		aiRunbookHandler := handlers.NewAIRunbookHandler()
		aiGroup.GET("/runbooks", aiRunbookHandler.GetRunbooks)
	}

	aiChatHandler := handlers.NewAIChatHandler(d.clusterSvc, aiConfigSvc, d.k8sMgr)
	aiNLQueryHandler := handlers.NewAINLQueryHandler(d.clusterSvc, aiConfigSvc, d.k8sMgr)
	aiChat := clusters.Group("/:clusterID/ai")
	aiChat.Use(d.permMiddleware.ClusterAccessRequired())
	{
		aiChat.POST("/chat", aiChatHandler.Chat)
		aiChat.POST("/nl-query", aiNLQueryHandler.NLQuery)

		// AI Root Cause Analysis (6.2)
		aiRCASvc := services.NewRCAService(aiConfigSvc)
		aiRCAHandler := handlers.NewAIRCAHandler(d.clusterSvc, d.k8sMgr, aiRCASvc)
		aiChat.POST("/rca", aiRCAHandler.AnalyzePod)
	}

	trivySvc := services.NewTrivyService(d.db)
	benchSvc := services.NewBenchService(d.db, d.k8sMgr)
	securityHandler := handlers.NewSecurityHandler(trivySvc, benchSvc, d.k8sMgr)
	securityGroup := clusters.Group("/:clusterID/security")
	securityGroup.Use(d.permMiddleware.ClusterAccessRequired())
	{
		securityGroup.POST("/scans", securityHandler.TriggerScan)
		securityGroup.GET("/scans", securityHandler.GetScanResults)
		securityGroup.GET("/scans/:scanID", securityHandler.GetScanDetail)
		securityGroup.POST("/bench", securityHandler.TriggerBenchmark)
		securityGroup.GET("/bench", securityHandler.GetBenchResults)
		securityGroup.GET("/bench/:benchID", securityHandler.GetBenchDetail)
		securityGroup.GET("/gatekeeper", securityHandler.GetGatekeeperViolations)

		// Secret Sprawl Scanner (6.4)
		secAuditSvc := services.NewSecurityAuditService()
		secAuditHandler := handlers.NewSecurityAuditHandler(d.clusterSvc, d.k8sMgr, secAuditSvc)
		securityGroup.GET("/secret-sprawl", secAuditHandler.ScanSecretSprawl)
	}
}
