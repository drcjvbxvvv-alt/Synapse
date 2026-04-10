package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/gin-gonic/gin"
)

// registerClusterNetworkingRoutes registers service, ingress, gateway API, and network policy routes.
func registerClusterNetworkingRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	resourceYAMLHandler := handlers.NewResourceYAMLHandler(d.cfg, d.clusterSvc, d.k8sMgr)

	// services
	serviceHandler := handlers.NewServiceHandler(d.cfg, d.clusterSvc, d.k8sMgr)
	svcGroup := cluster.Group("/services")
	{
		svcGroup.GET("", serviceHandler.ListServices)
		svcGroup.GET("/namespaces", serviceHandler.GetServiceNamespaces)
		svcGroup.POST("", serviceHandler.CreateService)
		svcGroup.GET("/:namespace/:name", serviceHandler.GetService)
		svcGroup.PUT("/:namespace/:name", serviceHandler.UpdateService)
		svcGroup.GET("/:namespace/:name/yaml", serviceHandler.GetServiceYAML)
		svcGroup.GET("/:namespace/:name/endpoints", serviceHandler.GetServiceEndpoints)
		svcGroup.DELETE("/:namespace/:name", serviceHandler.DeleteService)
		svcGroup.POST("/yaml/apply", resourceYAMLHandler.ApplyServiceYAML)
	}

	// ingresses
	ingressHandler := handlers.NewIngressHandler(d.cfg, d.clusterSvc, d.k8sMgr)
	ingresses := cluster.Group("/ingresses")
	{
		ingresses.GET("", ingressHandler.ListIngresses)
		ingresses.GET("/namespaces", ingressHandler.GetIngressNamespaces)
		ingresses.POST("", ingressHandler.CreateIngress)
		ingresses.GET("/:namespace/:name", ingressHandler.GetIngress)
		ingresses.PUT("/:namespace/:name", ingressHandler.UpdateIngress)
		ingresses.GET("/:namespace/:name/yaml", ingressHandler.GetIngressYAML)
		ingresses.DELETE("/:namespace/:name", ingressHandler.DeleteIngress)
		ingresses.POST("/yaml/apply", resourceYAMLHandler.ApplyIngressYAML)
	}

	// gateway API
	gatewayHandler := handlers.NewGatewayHandler(d.clusterSvc, d.k8sMgr)
	{
		cluster.GET("/gateway/status", gatewayHandler.GetGatewayAPIStatus)
		gatewayclasses := cluster.Group("/gatewayclasses")
		{
			gatewayclasses.GET("", gatewayHandler.ListGatewayClasses)
			gatewayclasses.GET("/:name", gatewayHandler.GetGatewayClass)
		}
		gateways := cluster.Group("/gateways")
		{
			gateways.GET("", gatewayHandler.ListGateways)
			gateways.POST("", gatewayHandler.CreateGateway)
			gateways.GET("/:namespace/:name", gatewayHandler.GetGateway)
			gateways.PUT("/:namespace/:name", gatewayHandler.UpdateGateway)
			gateways.DELETE("/:namespace/:name", gatewayHandler.DeleteGateway)
			gateways.GET("/:namespace/:name/yaml", gatewayHandler.GetGatewayYAML)
		}
		httproutes := cluster.Group("/httproutes")
		{
			httproutes.GET("", gatewayHandler.ListHTTPRoutes)
			httproutes.POST("", gatewayHandler.CreateHTTPRoute)
			httproutes.GET("/:namespace/:name", gatewayHandler.GetHTTPRoute)
			httproutes.PUT("/:namespace/:name", gatewayHandler.UpdateHTTPRoute)
			httproutes.DELETE("/:namespace/:name", gatewayHandler.DeleteHTTPRoute)
			httproutes.GET("/:namespace/:name/yaml", gatewayHandler.GetHTTPRouteYAML)
		}
		grpcroutes := cluster.Group("/grpcroutes")
		{
			grpcroutes.GET("", gatewayHandler.ListGRPCRoutes)
			grpcroutes.POST("", gatewayHandler.CreateGRPCRoute)
			grpcroutes.GET("/:namespace/:name", gatewayHandler.GetGRPCRoute)
			grpcroutes.PUT("/:namespace/:name", gatewayHandler.UpdateGRPCRoute)
			grpcroutes.DELETE("/:namespace/:name", gatewayHandler.DeleteGRPCRoute)
			grpcroutes.GET("/:namespace/:name/yaml", gatewayHandler.GetGRPCRouteYAML)
		}
		referencegrants := cluster.Group("/referencegrants")
		{
			referencegrants.GET("", gatewayHandler.ListReferenceGrants)
			referencegrants.POST("", gatewayHandler.CreateReferenceGrant)
			referencegrants.DELETE("/:namespace/:name", gatewayHandler.DeleteReferenceGrant)
			referencegrants.GET("/:namespace/:name/yaml", gatewayHandler.GetReferenceGrantYAML)
		}
		cluster.GET("/gateway/topology", gatewayHandler.GetTopology)
	}

	// network topology
	netTopoHandler := handlers.NewNetworkTopologyHandler(d.clusterSvc, d.k8sMgr)
	cluster.GET("/network/topology", netTopoHandler.GetClusterTopology)
	cluster.GET("/network/integrations", netTopoHandler.GetIntegrations)

	// network policies
	npHandler := handlers.NewNetworkPolicyHandler(d.clusterSvc, d.k8sMgr)
	nps := cluster.Group("/networkpolicies")
	{
		nps.GET("", npHandler.ListNetworkPolicies)
		nps.POST("", npHandler.CreateNetworkPolicy)
		nps.GET("/topology", npHandler.GetTopology)
		nps.GET("/conflicts", npHandler.GetConflicts)
		nps.POST("/wizard-validate", npHandler.WizardValidate)
		nps.POST("/simulate", npHandler.SimulateNetworkPolicy)
		nps.GET("/:namespace/:name", npHandler.GetNetworkPolicy)
		nps.PUT("/:namespace/:name", npHandler.UpdateNetworkPolicy)
		nps.GET("/:namespace/:name/yaml", npHandler.GetNetworkPolicyYAML)
		nps.DELETE("/:namespace/:name", npHandler.DeleteNetworkPolicy)
	}
}
