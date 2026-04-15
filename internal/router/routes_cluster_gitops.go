package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerClusterGitopsRoutes registers ArgoCD, GitOps, RBAC, and Helm release routes.
func registerClusterGitopsRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	// ArgoCD
	argoCDSvc := services.NewArgoCDService(d.db)
	argoCDHandler := handlers.NewArgoCDHandler(argoCDSvc)
	argocd := cluster.Group("/argocd")
	{
		argocd.GET("/config", argoCDHandler.GetConfig)
		argocd.PUT("/config", argoCDHandler.SaveConfig)
		argocd.POST("/test-connection", argoCDHandler.TestConnection)
		argocd.GET("/applications", argoCDHandler.ListApplications)
		argocd.GET("/applications/:appName", argoCDHandler.GetApplication)
		argocd.POST("/applications", argoCDHandler.CreateApplication)
		argocd.PUT("/applications/:appName", argoCDHandler.UpdateApplication)
		argocd.DELETE("/applications/:appName", argoCDHandler.DeleteApplication)
		argocd.POST("/applications/:appName/sync", argoCDHandler.SyncApplication)
		argocd.POST("/applications/:appName/rollback", argoCDHandler.RollbackApplication)
		argocd.GET("/applications/:appName/resources", argoCDHandler.GetApplicationResources)
	}

	// GitOps（M16 — native + ArgoCD 合併列表）
	gitopsSvc := services.NewGitOpsService(d.db)
	gitopsHandler := handlers.NewGitOpsHandler(gitopsSvc, argoCDSvc)
	gitops := cluster.Group("/gitops")
	{
		gitops.GET("/apps", gitopsHandler.ListMerged)
		gitops.GET("/apps/:id", gitopsHandler.Get)
		gitops.POST("/apps", gitopsHandler.Create)
		gitops.PUT("/apps/:id", gitopsHandler.Update)
		gitops.DELETE("/apps/:id", gitopsHandler.Delete)
		gitops.GET("/apps/:id/diff", gitopsHandler.GetDiff)
		gitops.POST("/apps/:id/sync", gitopsHandler.TriggerSync)
	}

	// RBAC
	rbacSvc := services.NewRBACService()
	rbacHandler := handlers.NewRBACHandler(d.clusterSvc, rbacSvc, d.k8sMgr)
	rbacGroup := cluster.Group("/rbac")
	{
		rbacGroup.GET("/status", rbacHandler.GetSyncStatus)
		rbacGroup.POST("/sync", rbacHandler.SyncPermissions)
		rbacGroup.GET("/clusterroles", rbacHandler.ListClusterRoles)
		rbacGroup.POST("/clusterroles", rbacHandler.CreateCustomClusterRole)
		rbacGroup.DELETE("/clusterroles/:name", rbacHandler.DeleteClusterRole)
	}

	// Helm releases (cluster-scoped)
	helmHandler := handlers.NewHelmHandler(d.clusterSvc, d.helmSvc)
	helmReleases := cluster.Group("/helm/releases")
	{
		helmReleases.GET("", helmHandler.ListReleases)
		helmReleases.POST("", helmHandler.InstallRelease)
		helmReleases.GET("/:namespace/:name", helmHandler.GetRelease)
		helmReleases.PUT("/:namespace/:name", helmHandler.UpgradeRelease)
		helmReleases.DELETE("/:namespace/:name", helmHandler.UninstallRelease)
		helmReleases.GET("/:namespace/:name/history", helmHandler.GetReleaseHistory)
		helmReleases.GET("/:namespace/:name/values", helmHandler.GetReleaseValues)
		helmReleases.POST("/:namespace/:name/rollback", helmHandler.RollbackRelease)
	}
}
