package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/gin-gonic/gin"
)

// registerClusterConfigRoutes registers configmap and secret routes.
func registerClusterConfigRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	resourceYAMLHandler := handlers.NewResourceYAMLHandler(d.cfg, d.clusterSvc, d.k8sMgr)

	// configmaps
	configMapHandler := handlers.NewConfigMapHandler(d.cfgVerSvc, d.cfg, d.clusterSvc, d.k8sMgr)
	configmaps := cluster.Group("/configmaps")
	{
		configmaps.GET("", configMapHandler.GetConfigMaps)
		configmaps.GET("/namespaces", configMapHandler.GetConfigMapNamespaces)
		configmaps.GET("/:namespace/:name", configMapHandler.GetConfigMap)
		configmaps.POST("", configMapHandler.CreateConfigMap)
		configmaps.PUT("/:namespace/:name", configMapHandler.UpdateConfigMap)
		configmaps.DELETE("/:namespace/:name", configMapHandler.DeleteConfigMap)
		configmaps.POST("/yaml/apply", resourceYAMLHandler.ApplyConfigMapYAML)
		configmaps.GET("/:namespace/:name/versions", configMapHandler.GetConfigMapVersions)
		configmaps.POST("/:namespace/:name/versions/:version/rollback", configMapHandler.RollbackConfigMap)
	}

	// secrets
	secretHandler := handlers.NewSecretHandler(d.cfgVerSvc, d.cfg, d.clusterSvc, d.k8sMgr)
	secrets := cluster.Group("/secrets")
	{
		secrets.GET("", secretHandler.GetSecrets)
		secrets.GET("/namespaces", secretHandler.GetSecretNamespaces)
		secrets.GET("/:namespace/:name", secretHandler.GetSecret)
		secrets.POST("", secretHandler.CreateSecret)
		secrets.PUT("/:namespace/:name", secretHandler.UpdateSecret)
		secrets.DELETE("/:namespace/:name", secretHandler.DeleteSecret)
		secrets.POST("/yaml/apply", resourceYAMLHandler.ApplySecretYAML)
		secrets.GET("/:namespace/:name/versions", secretHandler.GetSecretVersions)
	}
}
