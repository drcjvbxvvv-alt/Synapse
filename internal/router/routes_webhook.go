package router

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
)

// registerWebhookRoutes registers public webhook endpoints under /api/v1/webhooks.
// These routes do NOT require JWT auth — authentication is via HMAC signature.
// Rate limit is applied per-IP to prevent abuse.
func registerWebhookRoutes(api *gin.RouterGroup, d *routeDeps) {
	pipelineSvc := services.NewPipelineService(d.db)
	secretSvc := services.NewPipelineSecretService(d.db)
	logSvc := services.NewPipelineLogService(d.db)
	jobBuilder := services.NewJobBuilder()
	watcherCfg := services.DefaultJobWatcherConfig()
	watcher := services.NewJobWatcher(d.db, d.k8sMgr, watcherCfg)
	watcher.SetLogService(logSvc)
	scheduler := services.NewPipelineScheduler(d.db, jobBuilder, secretSvc, d.k8sMgr, watcher, services.DefaultSchedulerConfig())

	webhookHandler := handlers.NewPipelineWebhookHandler(pipelineSvc, scheduler, secretSvc)

	webhooks := api.Group("/webhooks")
	webhooks.Use(middleware.APIRateLimit("webhook", 60)) // 60 req/min per IP
	{
		webhooks.POST("/pipelines/:pipelineID/trigger", webhookHandler.TriggerWebhook)
	}
}
