package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
)

// registerClusterChaosRoutes registers Chaos Mesh experiment endpoints.
func registerClusterChaosRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	chaosSvc := services.NewChaosService()
	h := handlers.NewChaosHandler(d.clusterSvc, d.k8sMgr, chaosSvc)

	chaos := cluster.Group("/chaos")
	{
		chaos.GET("/status", h.GetChaosStatus)
		chaos.GET("/experiments", h.ListExperiments)
		chaos.POST("/experiments", h.CreateExperiment)
		chaos.GET("/experiments/:namespace/:kind/:name", h.GetExperiment)
		chaos.DELETE("/experiments/:namespace/:kind/:name", h.DeleteExperiment)
		chaos.GET("/schedules", h.ListSchedules)
		chaos.POST("/schedules", h.CreateSchedule)
		chaos.DELETE("/schedules/:namespace/:name", h.DeleteSchedule)
	}
}
