package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
)

// registerClusterComplianceRoutes registers compliance report, evidence, and violation timeline routes.
func registerClusterComplianceRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	complianceSvc := services.NewComplianceService(d.db)
	h := handlers.NewComplianceHandler(complianceSvc)

	compliance := cluster.Group("/compliance")
	{
		// Reports
		reports := compliance.Group("/reports")
		{
			reports.POST("", h.GenerateReport)
			reports.GET("", h.ListReports)
			reports.GET("/:id", h.GetReport)
			reports.GET("/:id/export", h.ExportReport)
			reports.DELETE("/:id", h.DeleteReport)
		}

		// Violations timeline
		violations := compliance.Group("/violations")
		{
			violations.GET("", h.ListViolations)
			violations.GET("/stats", h.GetViolationStats)
			violations.PUT("/:id/resolve", h.ResolveViolation)
		}

		// Evidence
		evidence := compliance.Group("/evidence")
		{
			evidence.POST("", h.CaptureEvidence)
			evidence.GET("", h.ListEvidence)
			evidence.GET("/:id", h.GetEvidence)
		}
	}
}
