package router

import (
	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
)

// registerPipelineRoutes registers top-level Pipeline routes under /pipelines.
// Pipeline is a cluster-independent entity; execution targets are bound via Environments.
func registerPipelineRoutes(protected *gin.RouterGroup, d *routeDeps) {
	secretSvc := services.NewPipelineSecretService(d.db)
	logSvc := services.NewPipelineLogService(d.db)
	envSvc := services.NewEnvironmentService(d.db)

	pipelineHandler := handlers.NewPipelineHandler(d.pipelineSvc, d.auditSvc)
	secretHandler := handlers.NewPipelineSecretHandler(secretSvc)
	logHandler := handlers.NewPipelineLogHandler(logSvc, d.pipelineSvc)
	envHandler := handlers.NewEnvironmentHandler(envSvc)

	runHandler := handlers.NewPipelineRunHandler(d.pipelineSvc, d.pipelineScheduler, d.auditSvc)
	runHandler.SetEnvironmentService(envSvc)

	// ── Pipelines ──────────────────────────────────────────────────────────
	pipelines := protected.Group("/pipelines")
	{
		pipelines.GET("", pipelineHandler.ListPipelines)
		pipelines.POST("", pipelineHandler.CreatePipeline)

		// Single pipeline group — PipelineAccessRequired validates existence
		pipeline := pipelines.Group("/:pipelineID")
		pipeline.Use(middleware.PipelineAccessRequired(d.db))
		{
			pipeline.GET("", pipelineHandler.GetPipeline)
			pipeline.PUT("", pipelineHandler.UpdatePipeline)
			pipeline.DELETE("", pipelineHandler.DeletePipeline)

			// Versions (immutable snapshots)
			versions := pipeline.Group("/versions")
			{
				versions.GET("", pipelineHandler.ListVersions)
				versions.POST("", pipelineHandler.CreateVersion)
				versions.GET("/:version", pipelineHandler.GetVersion)
			}

			// Environments (cluster binding)
			envs := pipeline.Group("/environments")
			{
				envs.GET("", envHandler.ListEnvironments)
				envs.POST("", envHandler.CreateEnvironment)
				envs.GET("/:envID", envHandler.GetEnvironment)
				envs.PUT("/:envID", envHandler.UpdateEnvironment)
				envs.DELETE("/:envID", envHandler.DeleteEnvironment)

				// Env-based run trigger
				envs.POST("/:envID/runs", runHandler.TriggerRunInEnvironment)

				// Env-scoped secrets
				envs.GET("/:envID/secrets", secretHandler.ListEnvSecrets)
				envs.POST("/:envID/secrets", secretHandler.CreateEnvSecret)
			}

			// Runs (cross-environment view)
			runs := pipeline.Group("/runs")
			{
				runs.GET("", runHandler.ListRuns)
				// Legacy direct trigger (cluster_id + namespace in body)
				runs.POST("", runHandler.TriggerRun)

				run := runs.Group("/:runID")
				{
					run.GET("", runHandler.GetRun)
					run.POST("/cancel", runHandler.CancelRun)
					run.POST("/rerun", runHandler.RerunPipeline)
					run.POST("/rollback", runHandler.RollbackRun)
					run.POST("/promote", runHandler.PromoteRun)

					// Step operations
					run.POST("/steps/:stepRunID/approve", runHandler.ApproveStep)
					run.POST("/steps/:stepRunID/reject", runHandler.RejectStep)

					// Step Logs (SSE)
					run.GET("/steps/:stepRunID/logs", logHandler.GetStepLogs)
				}
			}

			// Pipeline-scoped secrets
			pipeline.GET("/secrets", secretHandler.ListPipelineSecrets)
			pipeline.POST("/secrets", secretHandler.CreatePipelineSecret)
		}
	}

	// ── Pipeline Secrets (global scope) ────────────────────────────────────
	secrets := protected.Group("/pipeline-secrets")
	{
		secrets.GET("", secretHandler.ListSecrets)
		secrets.POST("", secretHandler.CreateSecret)
		secrets.GET("/:secretID", secretHandler.GetSecret)
		secrets.PUT("/:secretID", secretHandler.UpdateSecret)
		secrets.DELETE("/:secretID", secretHandler.DeleteSecret)
	}

	// ── Step type registry ──────────────────────────────────────────────────
	protected.GET("/pipeline-step-types", runHandler.ListStepTypes)
}
