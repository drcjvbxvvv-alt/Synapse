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
	secretSvc := services.NewPipelineSecretService(d.db)

	webhookHandler := handlers.NewPipelineWebhookHandler(d.pipelineSvc, d.pipelineScheduler, secretSvc)

	webhooks := api.Group("/webhooks")
	webhooks.Use(middleware.APIRateLimit("webhook", 60)) // 60 req/min per IP
	{
		webhooks.POST("/pipelines/:pipelineID/trigger", webhookHandler.TriggerWebhook)

		// Git Provider Webhook 接收端點（M14）
		// POST /webhooks/git/:token — token 為 GitProvider.WebhookToken
		gitWebhookHandler := handlers.NewGitWebhookHandler(d.gitProviderSvc, d.pipelineSvc, d.pipelineScheduler)
		webhooks.POST("/git/:token", gitWebhookHandler.IngestWebhook)
	}
}
