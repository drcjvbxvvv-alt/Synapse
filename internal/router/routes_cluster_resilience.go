package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/gin-gonic/gin"
)

// registerClusterResilienceRoutes registers PDB, port-forward, volume snapshots, and Velero routes.
func registerClusterResilienceRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	// PDB
	pdbHandler := handlers.NewPDBHandler(d.clusterSvc, d.k8sMgr)
	pdbGroup := cluster.Group("/pdbs")
	{
		pdbGroup.GET("", pdbHandler.ListPDB)
		pdbGroup.GET("/:namespace", pdbHandler.GetWorkloadPDB)
		pdbGroup.POST("", pdbHandler.CreatePDB)
		pdbGroup.PUT("/:namespace/:name", pdbHandler.UpdatePDB)
		pdbGroup.DELETE("/:namespace/:name", pdbHandler.DeletePDB)
	}

	// port-forward (per-pod)
	pfHandler := handlers.NewPortForwardHandler(d.portForwardSvc, d.clusterSvc, d.k8sMgr)
	cluster.POST("/pods/:namespace/:name/portforward", pfHandler.StartPortForward)

	// volume snapshots + velero
	vsHandler := handlers.NewVolumeSnapshotHandler(d.clusterSvc, d.k8sMgr)
	vsGroup := cluster.Group("/volume-snapshots")
	{
		vsGroup.GET("/status", vsHandler.CheckVolumeSnapshotCRD)
		vsGroup.GET("", vsHandler.ListVolumeSnapshots)
		vsGroup.POST("", vsHandler.CreateVolumeSnapshot)
		vsGroup.DELETE("/:namespace/:name", vsHandler.DeleteVolumeSnapshot)
	}
	cluster.GET("/volume-snapshot-classes", vsHandler.ListVolumeSnapshotClasses)
	veleroGroup := cluster.Group("/velero")
	{
		veleroGroup.GET("/status", vsHandler.CheckVelero)
		veleroGroup.GET("/backups", vsHandler.ListVeleroBackups)
		veleroGroup.GET("/restores", vsHandler.ListVeleroRestores)
		veleroGroup.POST("/restores", vsHandler.TriggerRestore)
		veleroGroup.GET("/schedules", vsHandler.ListVeleroSchedules)
		veleroGroup.POST("/schedules", vsHandler.CreateVeleroSchedule)
		veleroGroup.DELETE("/schedules/:name", vsHandler.DeleteVeleroSchedule)
	}
}
