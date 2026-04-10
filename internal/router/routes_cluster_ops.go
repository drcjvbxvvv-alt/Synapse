package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerClusterOpsRoutes registers logging, O&M, CRDs, event alerts,
// autoscaling (KEDA/Karpenter/CAS), cert-manager, and service mesh routes.
func registerClusterOpsRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	// logs
	logCenterHandler := handlers.NewLogCenterHandler(d.clusterSvc, d.k8sMgr)
	logs := cluster.Group("/logs")
	{
		logs.GET("/containers", logCenterHandler.GetContainerLogs)
		logs.GET("/events", logCenterHandler.GetEventLogs)
		logs.POST("/search", logCenterHandler.SearchLogs)
		logs.GET("/stats", logCenterHandler.GetLogStats)
		logs.GET("/namespaces", logCenterHandler.GetNamespacesForLogs)
		logs.GET("/pods", logCenterHandler.GetPodsForLogs)
		logs.POST("/export", logCenterHandler.ExportLogs)
	}

	// log-sources
	logSrcHandler := handlers.NewLogSourceHandler(d.logSourceSvc)
	logSrcs := cluster.Group("/log-sources")
	{
		logSrcs.GET("", logSrcHandler.ListLogSources)
		logSrcs.POST("", logSrcHandler.CreateLogSource)
		logSrcs.PUT("/:sourceId", logSrcHandler.UpdateLogSource)
		logSrcs.DELETE("/:sourceId", logSrcHandler.DeleteLogSource)
		logSrcs.POST("/:sourceId/search", logSrcHandler.SearchExternalLogs)
	}

	// O&M
	omSvc := services.NewOMService(d.prometheusSvc, d.monitoringCfgSvc)
	omHandler := handlers.NewOMHandler(d.clusterSvc, omSvc, d.k8sMgr)
	om := cluster.Group("/om")
	{
		om.GET("/health-diagnosis", omHandler.GetHealthDiagnosis)
		om.GET("/resource-top", omHandler.GetResourceTop)
		om.GET("/control-plane-status", omHandler.GetControlPlaneStatus)
	}

	// CRDs
	crdHandler := handlers.NewCRDHandler(d.clusterSvc, d.k8sMgr)
	crdGroup := cluster.Group("/crds")
	{
		crdGroup.GET("", crdHandler.ListCRDs)
		crdGroup.GET("/resources", crdHandler.ListCRDResources)
		crdGroup.GET("/resources/:namespace/:name", crdHandler.GetCRDResource)
		crdGroup.DELETE("/resources/:namespace/:name", crdHandler.DeleteCRDResource)
	}

	// event alerts
	eventAlertSvc := services.NewEventAlertService(d.db)
	eventAlertHandler := handlers.NewEventAlertHandler(eventAlertSvc)
	eventAlerts := cluster.Group("/event-alerts")
	{
		eventAlerts.GET("/rules", eventAlertHandler.ListRules)
		eventAlerts.POST("/rules", eventAlertHandler.CreateRule)
		eventAlerts.PUT("/rules/:ruleID", eventAlertHandler.UpdateRule)
		eventAlerts.DELETE("/rules/:ruleID", eventAlertHandler.DeleteRule)
		eventAlerts.PUT("/rules/:ruleID/toggle", eventAlertHandler.ToggleRule)
		eventAlerts.GET("/history", eventAlertHandler.ListHistory)
	}

	// autoscaling: KEDA, Karpenter, CAS
	autoscalingHandler := handlers.NewAutoscalingHandler(d.clusterSvc, d.k8sMgr)
	keda := cluster.Group("/keda")
	{
		keda.GET("/status", autoscalingHandler.CheckKEDA)
		keda.GET("/scaled-objects", autoscalingHandler.ListScaledObjects)
		keda.GET("/scaled-jobs", autoscalingHandler.ListScaledJobs)
	}
	karpenter := cluster.Group("/karpenter")
	{
		karpenter.GET("/status", autoscalingHandler.CheckKarpenter)
		karpenter.GET("/node-pools", autoscalingHandler.ListNodePools)
		karpenter.GET("/node-claims", autoscalingHandler.ListNodeClaims)
	}
	cluster.GET("/cas/status", autoscalingHandler.GetCASStatus)

	// cert-manager
	certMgrHandler := handlers.NewCertManagerHandler(d.clusterSvc, d.k8sMgr)
	cluster.GET("/cert-manager/status", certMgrHandler.CheckCertManagerStatus)
	certGroup := cluster.Group("/cert-manager")
	{
		certGroup.GET("/certificates", certMgrHandler.ListCertificates)
		certGroup.GET("/issuers", certMgrHandler.ListIssuers)
	}

	// service mesh (Istio)
	meshSvc := services.NewMeshService(d.prometheusSvc, d.monitoringCfgSvc)
	meshHandler := handlers.NewMeshHandler(d.clusterSvc, d.k8sMgr, meshSvc)
	mesh := cluster.Group("/service-mesh")
	{
		mesh.GET("/status", meshHandler.GetStatus)
		mesh.GET("/topology", meshHandler.GetTopology)
		mesh.GET("/virtual-services", meshHandler.ListVirtualServices)
		mesh.POST("/virtual-services", meshHandler.CreateVirtualService)
		mesh.GET("/virtual-services/:namespace/:name", meshHandler.GetVirtualService)
		mesh.PUT("/virtual-services/:namespace/:name", meshHandler.UpdateVirtualService)
		mesh.DELETE("/virtual-services/:namespace/:name", meshHandler.DeleteVirtualService)
		mesh.GET("/destination-rules", meshHandler.ListDestinationRules)
		mesh.POST("/destination-rules", meshHandler.CreateDestinationRule)
		mesh.GET("/destination-rules/:namespace/:name", meshHandler.GetDestinationRule)
		mesh.PUT("/destination-rules/:namespace/:name", meshHandler.UpdateDestinationRule)
		mesh.DELETE("/destination-rules/:namespace/:name", meshHandler.DeleteDestinationRule)
	}
}
