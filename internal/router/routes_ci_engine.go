package router

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/argo"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/github"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/gitlab"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/jenkins"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/tekton"
	"github.com/shaia/Synapse/pkg/logger"
)

// registerCIEngineRoutes registers /api/v1/ci-engines endpoints (M18a–M19b).
//
// All mutating operations (POST/PUT/DELETE) and the Create/Update/Delete paths
// are gated by PlatformAdminRequired because engine configs carry credentials
// (Token / Password / WebhookSecret) that grant access to external systems.
//
// The status probe (GET /status) is available to all authenticated users so
// pipeline authors can see which engines are selectable when composing a run.
//
// Adapter registration is best-effort: a duplicate registration (e.g. from a
// unit test that already registered) is logged and ignored so that multi-entry
// initialisation does not crash the server.
func registerCIEngineRoutes(protected *gin.RouterGroup, d *routeDeps) {
	// Register external engine adapters on the default factory. The native
	// engine is registered by main/bootstrap (see M18a); this block covers
	// M18b+ external adapters.
	if err := gitlab.Register(engine.Default()); err != nil {
		// A benign "already registered" is the only expected failure path
		// (tests may have pre-registered). Other errors still degrade the
		// feature rather than fail startup, matching Observer Pattern
		// guidance in CLAUDE.md §8.
		if !errors.Is(err, engine.ErrAlreadyRegistered) {
			logger.Warn("gitlab adapter registration returned non-fatal error, continuing",
				"error", err)
		}
	}
	if err := jenkins.Register(engine.Default()); err != nil {
		if !errors.Is(err, engine.ErrAlreadyRegistered) {
			logger.Warn("jenkins adapter registration returned non-fatal error, continuing",
				"error", err)
		}
	}
	// k8sResolver is shared between Tekton and Argo since both adapters need
	// dynamic + discovery access to a Synapse-managed cluster.
	k8sResolver := newK8sClusterResolver(d.k8sMgr)
	if err := tekton.Register(engine.Default(), k8sResolver); err != nil {
		if !errors.Is(err, engine.ErrAlreadyRegistered) {
			logger.Warn("tekton adapter registration returned non-fatal error, continuing",
				"error", err)
		}
	}
	if err := argo.Register(engine.Default(), k8sResolver); err != nil {
		if !errors.Is(err, engine.ErrAlreadyRegistered) {
			logger.Warn("argo adapter registration returned non-fatal error, continuing",
				"error", err)
		}
	}
	if err := github.Register(engine.Default()); err != nil {
		if !errors.Is(err, engine.ErrAlreadyRegistered) {
			logger.Warn("github adapter registration returned non-fatal error, continuing",
				"error", err)
		}
	}

	svc := services.NewCIEngineService(d.db, engine.Default())
	h := handlers.NewCIEngineHandler(svc)

	// Public-ish (auth-required but not admin-only)
	protected.GET("/ci-engines/status", h.ListAvailable)

	// Admin-only CRUD: credentials require platform-admin rights.
	admin := protected.Group("/ci-engines")
	admin.Use(middleware.PlatformAdminRequired(d.db))
	{
		admin.GET("", h.List)
		admin.GET("/:id", h.Get)
		admin.POST("", h.Create)
		admin.PUT("/:id", h.Update)
		admin.DELETE("/:id", h.Delete)

		// Run operations — trigger, status, cancel, logs, artifacts.
		// Kept under the admin group: accessing run logs or cancelling a run
		// on an externally-configured engine requires the same credential trust
		// as configuring the engine itself.
		runs := admin.Group("/:id/runs")
		{
			runs.POST("", h.TriggerRun)
			runs.GET("/:runId", h.GetRun)
			runs.DELETE("/:runId", h.CancelRun)
			runs.GET("/:runId/logs", h.StreamLogs)
			runs.GET("/:runId/artifacts", h.GetArtifacts)
		}
	}
}
