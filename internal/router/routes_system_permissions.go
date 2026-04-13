package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
)

// registerSystemPermissionsRoutes registers permissions/RBAC management routes.
// protected is the auth-required group.
func registerSystemPermissionsRoutes(protected *gin.RouterGroup, d *routeDeps) {
	// permissions - 權限管理
	globalRbacSvc := services.NewRBACService()
	permissionHandler := handlers.NewPermissionHandler(d.permissionSvc, d.clusterSvc, globalRbacSvc)
	globalRbacHandler := handlers.NewRBACHandler(d.clusterSvc, globalRbacSvc, d.k8sMgr)
	permissions := protected.Group("/permissions")
	{
		// 當前使用者權限查詢（任意登入使用者可訪問）
		permissions.GET("/my-permissions", permissionHandler.GetMyPermissions)
		permissions.GET("/types", permissionHandler.GetPermissionTypes)

		// 以下介面需要平臺管理員權限
		permAdmin := permissions.Group("")
		permAdmin.Use(middleware.PlatformAdminRequired(d.db))
		{
			// Synapse 預定義 ClusterRole 資訊
			permAdmin.GET("/synapse-roles", globalRbacHandler.GetSynapseClusterRoles)

			// 使用者列表（用於權限分配）
			permAdmin.GET("/users", permissionHandler.ListUsers)

			// 使用者組管理
			userGroups := permAdmin.Group("/user-groups")
			{
				userGroups.GET("", permissionHandler.ListUserGroups)
				userGroups.POST("", permissionHandler.CreateUserGroup)
				userGroups.GET("/:id", permissionHandler.GetUserGroup)
				userGroups.PUT("/:id", permissionHandler.UpdateUserGroup)
				userGroups.DELETE("/:id", permissionHandler.DeleteUserGroup)
				userGroups.POST("/:id/users", permissionHandler.AddUserToGroup)
				userGroups.DELETE("/:id/users/:userId", permissionHandler.RemoveUserFromGroup)
			}

			// 叢集權限管理
			clusterPerms := permAdmin.Group("/cluster-permissions")
			{
				clusterPerms.GET("", permissionHandler.ListAllClusterPermissions)
				clusterPerms.POST("", permissionHandler.CreateClusterPermission)
				clusterPerms.GET("/:id", permissionHandler.GetClusterPermission)
				clusterPerms.PUT("/:id", permissionHandler.UpdateClusterPermission)
				clusterPerms.DELETE("/:id", permissionHandler.DeleteClusterPermission)
				clusterPerms.POST("/batch-delete", permissionHandler.BatchDeleteClusterPermissions)

				// 功能開關策略
				clusterPerms.GET("/:id/features", permissionHandler.GetFeaturePolicy)
				clusterPerms.PATCH("/:id/features", permissionHandler.UpdateFeaturePolicy)
			}
		}
	}

	// 叢集級權限查詢
	protected.GET("/clusters/:clusterID/my-permissions", permissionHandler.GetMyClusterPermission)
}
