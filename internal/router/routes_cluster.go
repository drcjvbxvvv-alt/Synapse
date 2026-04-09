package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"github.com/gin-gonic/gin"
)

// registerClusterRoutes registers all /clusters routes onto the protected group.
func registerClusterRoutes(protected *gin.RouterGroup, d *routeDeps) {
	// clusters 根分組
	clusters := protected.Group("/clusters")
	{
		clusterHandler := handlers.NewClusterHandler(d.cfg, d.k8sMgr, d.clusterSvc, d.prometheusSvc, d.monitoringCfgSvc, d.permissionSvc)

		// 叢集列表和統計（按使用者權限過濾）
		clusters.GET("", clusterHandler.GetClusters)
		clusters.GET("/stats", clusterHandler.GetClusterStats)

		// 叢集匯入和測連（僅平臺管理員）
		clusters.POST("/import", middleware.PlatformAdminRequired(d.db), clusterHandler.ImportCluster)
		clusters.POST("/test-connection", middleware.PlatformAdminRequired(d.db), clusterHandler.TestConnection)

		// 動態 cluster 子分組（需要叢集權限檢查）
		cluster := clusters.Group("/:clusterID")
		cluster.Use(d.permMiddleware.ClusterAccessRequired()) // 啟用叢集權限檢查
		cluster.Use(d.permMiddleware.AutoWriteCheck())        // 自動檢查寫權限（POST/PUT/DELETE需要非只讀權限）
		{
			cluster.GET("", clusterHandler.GetCluster)
			cluster.GET("/status", clusterHandler.GetClusterStatus)
			cluster.GET("/overview", clusterHandler.GetClusterOverview)
			cluster.GET("/metrics", clusterHandler.GetClusterMetrics)
			cluster.GET("/events", clusterHandler.GetClusterEvents)
			cluster.DELETE("", clusterHandler.DeleteCluster)

			// namespaces 子分組
			namespaceHandler := handlers.NewNamespaceHandler(d.clusterSvc, d.k8sMgr)
			namespaces := cluster.Group("/namespaces")
			{
				namespaces.GET("", namespaceHandler.GetNamespaces)
				namespaces.GET("/:namespace", namespaceHandler.GetNamespaceDetail)
				namespaces.POST("", namespaceHandler.CreateNamespace)
				namespaces.DELETE("/:namespace", namespaceHandler.DeleteNamespace)
				// ResourceQuota CRUD
				namespaces.GET("/:namespace/quotas", namespaceHandler.ListResourceQuotas)
				namespaces.POST("/:namespace/quotas", namespaceHandler.CreateResourceQuota)
				namespaces.PUT("/:namespace/quotas/:name", namespaceHandler.UpdateResourceQuota)
				namespaces.DELETE("/:namespace/quotas/:name", namespaceHandler.DeleteResourceQuota)
				// LimitRange CRUD
				namespaces.GET("/:namespace/limitranges", namespaceHandler.ListLimitRanges)
				namespaces.POST("/:namespace/limitranges", namespaceHandler.CreateLimitRange)
				namespaces.PUT("/:namespace/limitranges/:name", namespaceHandler.UpdateLimitRange)
				namespaces.DELETE("/:namespace/limitranges/:name", namespaceHandler.DeleteLimitRange)
			}

			// monitoring 子分組
			monitoringHandler := handlers.NewMonitoringHandler(d.monitoringCfgSvc, d.prometheusSvc)
			monitoring := cluster.Group("/monitoring")
			{
				monitoring.GET("/config", monitoringHandler.GetMonitoringConfig)
				monitoring.PUT("/config", monitoringHandler.UpdateMonitoringConfig)
				monitoring.POST("/test-connection", monitoringHandler.TestMonitoringConnection)
				monitoring.GET("/metrics", monitoringHandler.GetClusterMetrics)
			}

			// alertmanager 子分組
			alertManagerConfigSvc := services.NewAlertManagerConfigService(d.db)
			alertManagerSvc := services.NewAlertManagerService()
			alertHandler := handlers.NewAlertHandler(alertManagerConfigSvc, alertManagerSvc, d.k8sMgr, d.clusterSvc)
			alertmanager := cluster.Group("/alertmanager")
			{
				alertmanager.GET("/config", alertHandler.GetAlertManagerConfig)
				alertmanager.PUT("/config", alertHandler.UpdateAlertManagerConfig)
				alertmanager.POST("/test-connection", alertHandler.TestAlertManagerConnection)
				alertmanager.GET("/status", alertHandler.GetAlertManagerStatus)
				alertmanager.GET("/template", alertHandler.GetAlertManagerConfigTemplate)
			}

			// alerts 子分組
			alerts := cluster.Group("/alerts")
			{
				alerts.GET("", alertHandler.GetAlerts)
				alerts.GET("/groups", alertHandler.GetAlertGroups)
				alerts.GET("/stats", alertHandler.GetAlertStats)
			}

			// silences 子分組
			silences := cluster.Group("/silences")
			{
				silences.GET("", alertHandler.GetSilences)
				silences.POST("", alertHandler.CreateSilence)
				silences.DELETE("/:silenceId", alertHandler.DeleteSilence)
			}

			// receivers 子分組
			receivers := cluster.Group("/receivers")
			{
				receivers.GET("", alertHandler.GetReceivers)
				receivers.GET("/full", alertHandler.GetFullReceivers)
				receivers.POST("", alertHandler.CreateReceiver)
				receivers.PUT("/:name", alertHandler.UpdateReceiver)
				receivers.DELETE("/:name", alertHandler.DeleteReceiver)
				receivers.POST("/:name/test", alertHandler.TestReceiver)
			}

			// nodes 子分組
			nodeHandler := handlers.NewNodeHandler(d.cfg, d.clusterSvc, d.k8sMgr, d.prometheusSvc, d.monitoringCfgSvc)
			nodes := cluster.Group("/nodes")
			{
				nodes.GET("", nodeHandler.GetNodes)
				nodes.GET("/overview", nodeHandler.GetNodeOverview)
				nodes.GET("/:name", nodeHandler.GetNode)
				nodes.POST("/:name/cordon", nodeHandler.CordonNode)
				nodes.POST("/:name/uncordon", nodeHandler.UncordonNode)
				nodes.POST("/:name/drain", nodeHandler.DrainNode)
				nodes.PATCH("/:name/labels", nodeHandler.PatchNodeLabels)
			nodes.GET("/:name/metrics", monitoringHandler.GetNodeMetrics)
			}

			// pods 子分組
			podHandler := handlers.NewPodHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			pods := cluster.Group("/pods")
			{
				pods.GET("", podHandler.GetPods) // 可考慮使用 query 過濾 namespace/name
				pods.GET("/namespaces", podHandler.GetPodNamespaces)
				pods.GET("/nodes", podHandler.GetPodNodes)
				pods.GET("/:namespace/:name", podHandler.GetPod)
				pods.DELETE("/:namespace/:name", podHandler.DeletePod)
				pods.GET("/:namespace/:name/logs", podHandler.GetPodLogs)
				pods.GET("/:namespace/:name/metrics", monitoringHandler.GetPodMetrics)
			}

			// Deployment 子分組
			deploymentHandler := handlers.NewDeploymentHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			deployments := cluster.Group("/deployments")
			{
				deployments.GET("", deploymentHandler.ListDeployments)
				deployments.GET("/namespaces", deploymentHandler.GetDeploymentNamespaces)
				deployments.GET("/:namespace/:name", deploymentHandler.GetDeployment)
				deployments.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
				deployments.POST("/yaml/apply", deploymentHandler.ApplyYAML)
				deployments.POST("/:namespace/:name/scale", deploymentHandler.ScaleDeployment)
				deployments.DELETE("/:namespace/:name", deploymentHandler.DeleteDeployment)
				// Deployment詳情頁相關介面
				deployments.GET("/:namespace/:name/pods", deploymentHandler.GetDeploymentPods)
				deployments.GET("/:namespace/:name/services", deploymentHandler.GetDeploymentServices)
				deployments.GET("/:namespace/:name/ingresses", deploymentHandler.GetDeploymentIngresses)
				deployments.GET("/:namespace/:name/hpa", deploymentHandler.GetDeploymentHPA)
				deployments.GET("/:namespace/:name/replicasets", deploymentHandler.GetDeploymentReplicaSets)
				deployments.GET("/:namespace/:name/events", deploymentHandler.GetDeploymentEvents)
			}

			// Rollout 子分組
			rolloutHandler := handlers.NewRolloutHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			rollouts := cluster.Group("/rollouts")
			{
				rollouts.GET("/crd-check", rolloutHandler.CheckRolloutCRD)
				rollouts.GET("", rolloutHandler.ListRollouts)
				rollouts.GET("/namespaces", rolloutHandler.GetRolloutNamespaces)
				rollouts.GET("/:namespace/:name", rolloutHandler.GetRollout)
				rollouts.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
				// Rollout詳情相關路由
				rollouts.GET("/:namespace/:name/pods", rolloutHandler.GetRolloutPods)
				rollouts.GET("/:namespace/:name/services", rolloutHandler.GetRolloutServices)
				rollouts.GET("/:namespace/:name/ingresses", rolloutHandler.GetRolloutIngresses)
				rollouts.GET("/:namespace/:name/hpa", rolloutHandler.GetRolloutHPA)
				rollouts.GET("/:namespace/:name/replicasets", rolloutHandler.GetRolloutReplicaSets)
				rollouts.GET("/:namespace/:name/events", rolloutHandler.GetRolloutEvents)
				rollouts.POST("/yaml/apply", rolloutHandler.ApplyYAML)
				rollouts.POST("/:namespace/:name/scale", rolloutHandler.ScaleRollout)
				rollouts.DELETE("/:namespace/:name", rolloutHandler.DeleteRollout)
				rollouts.POST("/:namespace/:name/promote", rolloutHandler.PromoteRollout)
				rollouts.POST("/:namespace/:name/promote-full", rolloutHandler.PromoteFullRollout)
				rollouts.POST("/:namespace/:name/abort", rolloutHandler.AbortRollout)
				rollouts.GET("/:namespace/:name/analysis-runs", rolloutHandler.GetRolloutAnalysisRuns)
			}

			// HPA 子分組
			hpaHandler := handlers.NewHPAHandler(d.clusterSvc, d.k8sMgr)
			hpa := cluster.Group("/hpa")
			{
				hpa.GET("", hpaHandler.ListHPA)
				hpa.POST("", hpaHandler.CreateHPA)
				hpa.PUT("/:namespace/:name", hpaHandler.UpdateHPA)
				hpa.DELETE("/:namespace/:name", hpaHandler.DeleteHPA)
			}

			// StatefulSet 子分組
			statefulSetHandler := handlers.NewStatefulSetHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			statefulSets := cluster.Group("/statefulsets")
			{
				statefulSets.GET("", statefulSetHandler.ListStatefulSets)
				statefulSets.GET("/namespaces", statefulSetHandler.GetStatefulSetNamespaces)
				statefulSets.GET("/:namespace/:name", statefulSetHandler.GetStatefulSet)
				statefulSets.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
				statefulSets.POST("/yaml/apply", statefulSetHandler.ApplyYAML)
				statefulSets.POST("/:namespace/:name/scale", statefulSetHandler.ScaleStatefulSet)
				statefulSets.DELETE("/:namespace/:name", statefulSetHandler.DeleteStatefulSet)
			}

			// DaemonSet 子分組
			daemonSetHandler := handlers.NewDaemonSetHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			daemonsets := cluster.Group("/daemonsets")
			{
				daemonsets.GET("", daemonSetHandler.ListDaemonSets)
				daemonsets.GET("/namespaces", daemonSetHandler.GetDaemonSetNamespaces)
				daemonsets.GET("/:namespace/:name", daemonSetHandler.GetDaemonSet)
				daemonsets.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
				daemonsets.POST("/yaml/apply", daemonSetHandler.ApplyYAML)
				daemonsets.DELETE("/:namespace/:name", daemonSetHandler.DeleteDaemonSet)
			}

			// Job 子分組
			jobHandler := handlers.NewJobHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			jobs := cluster.Group("/jobs")
			{
				jobs.GET("", jobHandler.ListJobs)
				jobs.GET("/namespaces", jobHandler.GetJobNamespaces)
				jobs.GET("/:namespace/:name", jobHandler.GetJob)
				jobs.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
				jobs.POST("/yaml/apply", jobHandler.ApplyYAML)
				jobs.DELETE("/:namespace/:name", jobHandler.DeleteJob)
			}

			// CronJob 子分組
			cronJobHandler := handlers.NewCronJobHandler(d.cfg, d.clusterSvc, d.k8sMgr)
			cronjobs := cluster.Group("/cronjobs")
			{
				cronjobs.GET("", cronJobHandler.ListCronJobs)
				cronjobs.GET("/namespaces", cronJobHandler.GetCronJobNamespaces)
				cronjobs.GET("/:namespace/:name", cronJobHandler.GetCronJob)
				cronjobs.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
				cronjobs.POST("/yaml/apply", cronJobHandler.ApplyYAML)
				cronjobs.DELETE("/:namespace/:name", cronJobHandler.DeleteCronJob)
			}

			// 通用資源 YAML 處理器（用於 dry-run 和 apply）
			resourceYAMLHandler := handlers.NewResourceYAMLHandler(d.cfg, d.clusterSvc, d.k8sMgr)

			// configmaps 子分組
			configMapHandler := handlers.NewConfigMapHandler(d.db, d.cfg, d.clusterSvc, d.k8sMgr)
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

			// secrets 子分組
			secretHandler := handlers.NewSecretHandler(d.db, d.cfg, d.clusterSvc, d.k8sMgr)
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

			// services 子分組
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

			// ingresses 子分組
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

			// Gateway API 子分組（Phase 1：唯讀）
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

			// 叢集網路拓樸（Phase 4）
			netTopoHandler := handlers.NewNetworkTopologyHandler(d.clusterSvc, d.k8sMgr)
			cluster.GET("/network/topology", netTopoHandler.GetClusterTopology)
			cluster.GET("/network/integrations", netTopoHandler.GetIntegrations)

			// networkpolicies 子分組
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

			// storage 子分組 - PVC, PV, StorageClass
			storageHandler := handlers.NewStorageHandler(d.cfg, d.clusterSvc, d.k8sMgr)

			// PVCs 子分組
			pvcs := cluster.Group("/pvcs")
			{
				pvcs.GET("", storageHandler.ListPVCs)
				pvcs.GET("/namespaces", storageHandler.GetPVCNamespaces)
				pvcs.GET("/:namespace/:name", storageHandler.GetPVC)
				pvcs.GET("/:namespace/:name/yaml", storageHandler.GetPVCYAML)
				pvcs.DELETE("/:namespace/:name", storageHandler.DeletePVC)
				pvcs.POST("/yaml/apply", resourceYAMLHandler.ApplyPVCYAML)
			}

			// PVs 子分組
			pvs := cluster.Group("/pvs")
			{
				pvs.GET("", storageHandler.ListPVs)
				pvs.GET("/:name", storageHandler.GetPV)
				pvs.GET("/:name/yaml", storageHandler.GetPVYAML)
				pvs.DELETE("/:name", storageHandler.DeletePV)
				pvs.POST("/yaml/apply", resourceYAMLHandler.ApplyPVYAML)
			}

			// StorageClasses 子分組
			storageclasses := cluster.Group("/storageclasses")
			{
				storageclasses.GET("", storageHandler.ListStorageClasses)
				storageclasses.GET("/:name", storageHandler.GetStorageClass)
				storageclasses.GET("/:name/yaml", storageHandler.GetStorageClassYAML)
				storageclasses.DELETE("/:name", storageHandler.DeleteStorageClass)
				storageclasses.POST("/yaml/apply", resourceYAMLHandler.ApplyStorageClassYAML)
			}

			// ArgoCD / GitOps 外掛中心
			argoCDSvc := services.NewArgoCDService(d.db)
			argoCDHandler := handlers.NewArgoCDHandler(argoCDSvc)
			argocd := cluster.Group("/argocd")
			{
				// 配置管理
				argocd.GET("/config", argoCDHandler.GetConfig)
				argocd.PUT("/config", argoCDHandler.SaveConfig)
				argocd.POST("/test-connection", argoCDHandler.TestConnection)

				// 應用管理（透過 ArgoCD API 代理）
				argocd.GET("/applications", argoCDHandler.ListApplications)
				argocd.GET("/applications/:appName", argoCDHandler.GetApplication)
				argocd.POST("/applications", argoCDHandler.CreateApplication)
				argocd.PUT("/applications/:appName", argoCDHandler.UpdateApplication)
				argocd.DELETE("/applications/:appName", argoCDHandler.DeleteApplication)
				argocd.POST("/applications/:appName/sync", argoCDHandler.SyncApplication)
				argocd.POST("/applications/:appName/rollback", argoCDHandler.RollbackApplication)
				argocd.GET("/applications/:appName/resources", argoCDHandler.GetApplicationResources)
			}

			// RBAC 子分組 - Synapse 權限管理
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

			// logs - 日誌中心
			logCenterHandler := handlers.NewLogCenterHandler(d.clusterSvc, d.k8sMgr)
			logs := cluster.Group("/logs")
			{
				logs.GET("/containers", logCenterHandler.GetContainerLogs)     // 獲取容器日誌
				logs.GET("/events", logCenterHandler.GetEventLogs)             // 獲取K8s事件日誌
				logs.POST("/search", logCenterHandler.SearchLogs)              // 日誌搜尋
				logs.GET("/stats", logCenterHandler.GetLogStats)               // 日誌統計
				logs.GET("/namespaces", logCenterHandler.GetNamespacesForLogs) // 獲取命名空間列表
				logs.GET("/pods", logCenterHandler.GetPodsForLogs)             // 獲取Pod列表
				logs.POST("/export", logCenterHandler.ExportLogs)              // 匯出日誌
			}

			// log-sources - 外部日誌源（Loki / Elasticsearch）
			logSrcHandler := handlers.NewLogSourceHandler(d.db)
			logSrcs := cluster.Group("/log-sources")
			{
				logSrcs.GET("", logSrcHandler.ListLogSources)
				logSrcs.POST("", logSrcHandler.CreateLogSource)
				logSrcs.PUT("/:sourceId", logSrcHandler.UpdateLogSource)
				logSrcs.DELETE("/:sourceId", logSrcHandler.DeleteLogSource)
				logSrcs.POST("/:sourceId/search", logSrcHandler.SearchExternalLogs)
			}

			// O&M - 監控中心（運維）
			omSvc := services.NewOMService(d.prometheusSvc, d.monitoringCfgSvc)
			omHandler := handlers.NewOMHandler(d.clusterSvc, omSvc, d.k8sMgr)
			om := cluster.Group("/om")
			{
				om.GET("/health-diagnosis", omHandler.GetHealthDiagnosis)        // 叢集健康診斷
				om.GET("/resource-top", omHandler.GetResourceTop)                // 資源消耗 Top N
				om.GET("/control-plane-status", omHandler.GetControlPlaneStatus) // 控制面元件狀態
			}

			// Helm Release 管理（cluster-scoped）
			helmHandler := handlers.NewHelmHandler(d.clusterSvc, d.db)
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

			// CRD 自動發現與通用資源列表
			crdHandler := handlers.NewCRDHandler(d.clusterSvc, d.k8sMgr)
			crdGroup := cluster.Group("/crds")
			{
				crdGroup.GET("", crdHandler.ListCRDs)
				crdGroup.GET("/resources", crdHandler.ListCRDResources)
				crdGroup.GET("/resources/:namespace/:name", crdHandler.GetCRDResource)
				crdGroup.DELETE("/resources/:namespace/:name", crdHandler.DeleteCRDResource)
			}

			// Event 告警規則
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

			// 資源治理（Phase 1+2：K8s API 佔用 + Prometheus 效率分析）
			resourceSvc := services.NewResourceService(d.db, d.k8sMgr, d.clusterSvc, d.prometheusSvc, d.monitoringCfgSvc)
			resourceHandler := handlers.NewResourceHandler(resourceSvc, d.clusterSvc)
			resources := cluster.Group("/resources")
			{
				resources.GET("/snapshot", resourceHandler.GetSnapshot)
				resources.GET("/namespaces", resourceHandler.GetNamespaceOccupancy)
				resources.GET("/efficiency", resourceHandler.GetNamespaceEfficiency)
				resources.GET("/workloads", resourceHandler.GetWorkloadEfficiency)
				resources.GET("/waste", resourceHandler.GetWasteWorkloads)
				resources.GET("/waste/export", resourceHandler.ExportWasteCSV) // Phase 3
				resources.GET("/trend", resourceHandler.GetTrend)              // Phase 3
				resources.GET("/forecast", resourceHandler.GetForecast)        // Phase 3
			}

			// 雲端帳單整合（Phase 4）
			cloudBillingSvc := services.NewCloudBillingService(d.db)
			cloudBillingHandler := handlers.NewCloudBillingHandler(cloudBillingSvc, d.clusterSvc)
			billing := cluster.Group("/billing")
			{
				billing.GET("/config", cloudBillingHandler.GetBillingConfig)
				billing.PUT("/config", cloudBillingHandler.UpdateBillingConfig)
				billing.POST("/sync", cloudBillingHandler.SyncBilling)
				billing.GET("/overview", cloudBillingHandler.GetBillingOverview)
			}

			// 資源成本分析（保留既有金錢估算功能）
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

			// VPA 子分組（§8.3 Phase C）
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

			// 審批請求（per-cluster）（§8.3 Phase C）
			approvalHandler := handlers.NewApprovalHandler(d.db, d.clusterSvc)
			clusterApprovals := cluster.Group("/approvals")
			{
				clusterApprovals.POST("", approvalHandler.CreateApprovalRequest)
			}

			// 命名空間保護設定（§8.3 Phase C）
			nsProt := cluster.Group("/namespace-protections")
			{
				nsProt.GET("", approvalHandler.GetNamespaceProtections)
				nsProt.PUT("/:namespace", approvalHandler.SetNamespaceProtection)
				nsProt.GET("/:namespace", approvalHandler.GetNamespaceProtectionStatus)
			}

			// PDB（PodDisruptionBudget）管理（§8.3 Phase D）
			pdbHandler := handlers.NewPDBHandler(d.clusterSvc, d.k8sMgr)
			pdbGroup := cluster.Group("/pdbs")
			{
				pdbGroup.GET("", pdbHandler.ListPDB)
				pdbGroup.GET("/:namespace", pdbHandler.GetWorkloadPDB)
				pdbGroup.POST("", pdbHandler.CreatePDB)
				pdbGroup.PUT("/:namespace/:name", pdbHandler.UpdatePDB)
				pdbGroup.DELETE("/:namespace/:name", pdbHandler.DeletePDB)
			}

			// Port-Forward（per-pod）（§8.3 Phase D）
			pfHandler := handlers.NewPortForwardHandler(d.db, d.clusterSvc, d.k8sMgr)
			cluster.POST("/pods/:namespace/:name/portforward", pfHandler.StartPortForward)

			// VolumeSnapshot + Velero（§5.22）
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

			// 彈性伸縮深化（§5.19）：KEDA / Karpenter / CAS
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

			// cert-manager 憑證管理（§5.18）
			certMgrHandler := handlers.NewCertManagerHandler(d.clusterSvc, d.k8sMgr)
			cluster.GET("/cert-manager/status", certMgrHandler.CheckCertManagerStatus)
			certGroup := cluster.Group("/cert-manager")
			{
				certGroup.GET("/certificates", certMgrHandler.ListCertificates)
				certGroup.GET("/issuers", certMgrHandler.ListIssuers)
			}

			// Service Mesh（Istio）
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
	}
}
