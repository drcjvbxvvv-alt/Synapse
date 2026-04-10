package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/gin-gonic/gin"
)

// registerClusterStorageRoutes registers PVC, PV, and StorageClass routes.
func registerClusterStorageRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	resourceYAMLHandler := handlers.NewResourceYAMLHandler(d.cfg, d.clusterSvc, d.k8sMgr)
	storageHandler := handlers.NewStorageHandler(d.cfg, d.clusterSvc, d.k8sMgr)

	pvcs := cluster.Group("/pvcs")
	{
		pvcs.GET("", storageHandler.ListPVCs)
		pvcs.GET("/namespaces", storageHandler.GetPVCNamespaces)
		pvcs.GET("/:namespace/:name", storageHandler.GetPVC)
		pvcs.GET("/:namespace/:name/yaml", storageHandler.GetPVCYAML)
		pvcs.DELETE("/:namespace/:name", storageHandler.DeletePVC)
		pvcs.POST("/yaml/apply", resourceYAMLHandler.ApplyPVCYAML)
	}

	pvs := cluster.Group("/pvs")
	{
		pvs.GET("", storageHandler.ListPVs)
		pvs.GET("/:name", storageHandler.GetPV)
		pvs.GET("/:name/yaml", storageHandler.GetPVYAML)
		pvs.DELETE("/:name", storageHandler.DeletePV)
		pvs.POST("/yaml/apply", resourceYAMLHandler.ApplyPVYAML)
	}

	storageclasses := cluster.Group("/storageclasses")
	{
		storageclasses.GET("", storageHandler.ListStorageClasses)
		storageclasses.GET("/:name", storageHandler.GetStorageClass)
		storageclasses.GET("/:name/yaml", storageHandler.GetStorageClassYAML)
		storageclasses.DELETE("/:name", storageHandler.DeleteStorageClass)
		storageclasses.POST("/yaml/apply", resourceYAMLHandler.ApplyStorageClassYAML)
	}
}
