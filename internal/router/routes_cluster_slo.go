package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
)

// registerClusterSLORoutes registers SLO CRUD and status endpoints.
func registerClusterSLORoutes(cluster *gin.RouterGroup, d *routeDeps) {
	sloSvc := services.NewSLOService(d.db, d.prometheusSvc, d.monitoringCfgSvc)
	chaosSvc := services.NewChaosService()
	h := handlers.NewSLOHandler(d.clusterSvc, sloSvc, d.k8sMgr, chaosSvc)

	slos := cluster.Group("/slos")
	{
		slos.GET("", h.ListSLOs)
		slos.POST("", h.CreateSLO)
		slos.GET("/:id", h.GetSLO)
		slos.PUT("/:id", h.UpdateSLO)
		slos.DELETE("/:id", h.DeleteSLO)
		slos.GET("/:id/status", h.GetSLOStatus)
	}
}
