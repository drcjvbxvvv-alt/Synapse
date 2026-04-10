package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/gin-gonic/gin"
)

// registerClusterWorkloadRoutes registers workload-related routes:
// nodes, pods, deployments, rollouts, HPA, statefulsets, daemonsets, jobs, cronjobs.
func registerClusterWorkloadRoutes(cluster *gin.RouterGroup, d *routeDeps) {
	monitoringHandler := handlers.NewMonitoringHandler(d.monitoringCfgSvc, d.prometheusSvc)

	// nodes
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

	// pods
	podHandler := handlers.NewPodHandler(d.cfg, d.clusterSvc, d.k8sMgr)
	pods := cluster.Group("/pods")
	{
		pods.GET("", podHandler.GetPods)
		pods.GET("/namespaces", podHandler.GetPodNamespaces)
		pods.GET("/nodes", podHandler.GetPodNodes)
		pods.GET("/:namespace/:name", podHandler.GetPod)
		pods.DELETE("/:namespace/:name", podHandler.DeletePod)
		pods.GET("/:namespace/:name/logs", podHandler.GetPodLogs)
		pods.GET("/:namespace/:name/metrics", monitoringHandler.GetPodMetrics)
	}

	// deployments
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
		deployments.GET("/:namespace/:name/pods", deploymentHandler.GetDeploymentPods)
		deployments.GET("/:namespace/:name/services", deploymentHandler.GetDeploymentServices)
		deployments.GET("/:namespace/:name/ingresses", deploymentHandler.GetDeploymentIngresses)
		deployments.GET("/:namespace/:name/hpa", deploymentHandler.GetDeploymentHPA)
		deployments.GET("/:namespace/:name/replicasets", deploymentHandler.GetDeploymentReplicaSets)
		deployments.GET("/:namespace/:name/events", deploymentHandler.GetDeploymentEvents)
	}

	// rollouts
	rolloutHandler := handlers.NewRolloutHandler(d.cfg, d.clusterSvc, d.k8sMgr)
	rollouts := cluster.Group("/rollouts")
	{
		rollouts.GET("/crd-check", rolloutHandler.CheckRolloutCRD)
		rollouts.GET("", rolloutHandler.ListRollouts)
		rollouts.GET("/namespaces", rolloutHandler.GetRolloutNamespaces)
		rollouts.GET("/:namespace/:name", rolloutHandler.GetRollout)
		rollouts.GET("/:namespace/:name/metrics", monitoringHandler.GetWorkloadMetrics)
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

	// HPA
	hpaHandler := handlers.NewHPAHandler(d.clusterSvc, d.k8sMgr)
	hpa := cluster.Group("/hpa")
	{
		hpa.GET("", hpaHandler.ListHPA)
		hpa.POST("", hpaHandler.CreateHPA)
		hpa.PUT("/:namespace/:name", hpaHandler.UpdateHPA)
		hpa.DELETE("/:namespace/:name", hpaHandler.DeleteHPA)
	}

	// statefulsets
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

	// daemonsets
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

	// jobs
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

	// cronjobs
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
}
