package router

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
)

// registerClusterPipelineRoutes registers Pipeline + Run + Secret + Log routes under /:clusterID.
func registerClusterPipelineRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	secretSvc := services.NewPipelineSecretService(d.db)
	logSvc := services.NewPipelineLogService(d.db)

	pipelineHandler := handlers.NewPipelineHandler(d.pipelineSvc)
	secretHandler := handlers.NewPipelineSecretHandler(secretSvc)
	logHandler := handlers.NewPipelineLogHandler(logSvc, d.pipelineSvc)
	runHandler := handlers.NewPipelineRunHandler(d.pipelineSvc, d.pipelineScheduler)

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

			// Runs
			runs := pipeline.Group("/runs")
			{
				runs.GET("", runHandler.ListRuns)
				runs.POST("", runHandler.TriggerRun)

				run := runs.Group("/:runID")
				{
					run.GET("", runHandler.GetRun)
					run.POST("/cancel", runHandler.CancelRun)
					run.POST("/rerun", runHandler.RerunPipeline)

					// Step Approval
					run.POST("/steps/:stepRunID/approve", runHandler.ApproveStep)
					run.POST("/steps/:stepRunID/reject", runHandler.RejectStep)

					// Step Logs
					run.GET("/steps/:stepRunID/logs", logHandler.GetStepLogs)
				}
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
