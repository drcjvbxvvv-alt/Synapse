package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerClusterGovernanceRoutes registers resource governance, cost, VPA, and approval routes.
func registerClusterGovernanceRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	// resource governance
	resourceSvc := services.NewResourceService(d.db, d.k8sMgr, d.clusterSvc, d.prometheusSvc, d.monitoringCfgSvc)
	resourceHandler := handlers.NewResourceHandler(resourceSvc, d.clusterSvc)
	resources := cluster.Group("/resources")
	{
		resources.GET("/snapshot", resourceHandler.GetSnapshot)
		resources.GET("/namespaces", resourceHandler.GetNamespaceOccupancy)
		resources.GET("/efficiency", resourceHandler.GetNamespaceEfficiency)
		resources.GET("/workloads", resourceHandler.GetWorkloadEfficiency)
		resources.GET("/waste", resourceHandler.GetWasteWorkloads)
		resources.GET("/waste/export", resourceHandler.ExportWasteCSV)
		resources.GET("/trend", resourceHandler.GetTrend)
		resources.GET("/forecast", resourceHandler.GetForecast)
	}

	// cloud billing
	cloudBillingSvc := services.NewCloudBillingService(d.db)
	cloudBillingHandler := handlers.NewCloudBillingHandler(cloudBillingSvc, d.clusterSvc)
	billing := cluster.Group("/billing")
	{
		billing.GET("/config", cloudBillingHandler.GetBillingConfig)
		billing.PUT("/config", cloudBillingHandler.UpdateBillingConfig)
		billing.POST("/sync", cloudBillingHandler.SyncBilling)
		billing.GET("/overview", cloudBillingHandler.GetBillingOverview)
	}

	// cost analysis
	costSvc := services.NewCostService(d.db)
	costHandler := handlers.NewCostHandler(costSvc)
	cost := cluster.Group("/cost")
	{
		cost.GET("/config", costHandler.GetConfig)
		cost.PUT("/config", costHandler.UpdateConfig)
		cost.GET("/overview", costHandler.GetOverview)
		cost.GET("/namespaces", costHandler.GetNamespaceCosts)
		cost.GET("/workloads", costHandler.GetWorkloadCosts)
		cost.GET("/trend", costHandler.GetTrend)
		cost.GET("/waste", costHandler.GetWaste)
		cost.GET("/export", costHandler.ExportCSV)
	}

	// VPA
	vpaHandler := handlers.NewVPAHandler(d.clusterSvc, d.k8sMgr)
	vpaGroup := cluster.Group("/vpa")
	{
		vpaGroup.GET("/crd-check", vpaHandler.CheckVPACRD)
		vpaGroup.GET("", vpaHandler.ListVPA)
		vpaGroup.POST("", vpaHandler.CreateVPA)
		vpaGroup.PUT("/:namespace/:name", vpaHandler.UpdateVPA)
		vpaGroup.DELETE("/:namespace/:name", vpaHandler.DeleteVPA)
		vpaGroup.GET("/:namespace/:name/workload", vpaHandler.GetWorkloadVPA)
	}

	// approvals + namespace protections
	approvalHandler := handlers.NewApprovalHandler(d.approvalSvc, d.clusterSvc)
	clusterApprovals := cluster.Group("/approvals")
	{
		clusterApprovals.POST("", approvalHandler.CreateApprovalRequest)
	}
	nsProt := cluster.Group("/namespace-protections")
	{
		nsProt.GET("", approvalHandler.GetNamespaceProtections)
		nsProt.PUT("/:namespace", approvalHandler.SetNamespaceProtection)
		nsProt.GET("/:namespace", approvalHandler.GetNamespaceProtectionStatus)
	}
}
