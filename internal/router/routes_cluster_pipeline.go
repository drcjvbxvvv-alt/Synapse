package router

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
)

// registerClusterPipelineRoutes registers Pipeline + Secret + Log routes under /:clusterID.
func registerClusterPipelineRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	pipelineSvc := services.NewPipelineService(d.db)
	secretSvc := services.NewPipelineSecretService(d.db)
	logSvc := services.NewPipelineLogService(d.db)

	pipelineHandler := handlers.NewPipelineHandler(pipelineSvc)
	secretHandler := handlers.NewPipelineSecretHandler(secretSvc)
	logHandler := handlers.NewPipelineLogHandler(logSvc, pipelineSvc)

	// ── Pipelines ──────────────────────────────────────────────────────
	pipelines := cluster.Group("/pipelines")
	{
		pipelines.GET("", pipelineHandler.ListPipelines)
		pipelines.POST("", pipelineHandler.CreatePipeline)

		pipeline := pipelines.Group("/:pipelineID")
		{
			pipeline.GET("", pipelineHandler.GetPipeline)
			pipeline.PUT("", pipelineHandler.UpdatePipeline)
			pipeline.DELETE("", pipelineHandler.DeletePipeline)

			// Versions
			versions := pipeline.Group("/versions")
			{
				versions.GET("", pipelineHandler.ListVersions)
				versions.POST("", pipelineHandler.CreateVersion)
				versions.GET("/:version", pipelineHandler.GetVersion)
			}

			// Runs → Step Logs
			// GET /clusters/:clusterID/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/logs?follow=true|false
			runs := pipeline.Group("/runs")
			{
				runs.GET("/:runID/steps/:stepRunID/logs", logHandler.GetStepLogs)
			}
		}
	}

	// ── Pipeline Secrets ───────────────────────────────────────────────
	secrets := cluster.Group("/pipeline-secrets")
	{
		secrets.GET("", secretHandler.ListSecrets)
		secrets.POST("", secretHandler.CreateSecret)
		secrets.GET("/:secretID", secretHandler.GetSecret)
		secrets.PUT("/:secretID", secretHandler.UpdateSecret)
		secrets.DELETE("/:secretID", secretHandler.DeleteSecret)
	}
}
