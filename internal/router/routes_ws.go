package router

import (
	"github.com/clay-wangzhi/Synapse/internal/handlers"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerWSRoutes registers all /ws WebSocket routes.
func registerWSRoutes(r *gin.Engine, d *routeDeps) {
	// WebSocket：建议也加认证
	ws := r.Group("/ws")
	ws.Use(middleware.AuthRequired(d.cfg.JWT.Secret))
	{
		// 终端处理器（注入审计服务）
		kctl := handlers.NewKubectlTerminalHandler(d.clusterSvc, d.auditSvc)
		ssh := handlers.NewSSHHandler(d.auditSvc)
		podTerminal := handlers.NewPodTerminalHandler(d.clusterSvc, d.auditSvc, d.k8sMgr)
		kubectlPod := handlers.NewKubectlPodTerminalHandler(d.clusterSvc, d.auditSvc, d.k8sMgr)
		podHandler := handlers.NewPodHandler(d.db, d.cfg, d.clusterSvc, d.k8sMgr)
		logCenterHandler := handlers.NewLogCenterHandler(d.clusterSvc, d.k8sMgr)

		// 节点 SSH 终端（需要平台管理员权限）
		ws.GET("/ssh/terminal", middleware.PlatformAdminRequired(d.db), ssh.SSHConnect)

		// 集群相关的 WebSocket 路由（需要集群权限检查）
		wsCluster := ws.Group("/clusters/:clusterID")
		wsCluster.Use(d.permMiddleware.ClusterAccessRequired()) // 启用集群权限检查
		{
			// 集群级 kubectl 终端（旧方案：本地执行）
			wsCluster.GET("/terminal", kctl.HandleKubectlTerminal)

			// 集群级 kubectl 终端（新方案：Pod 模式，支持 tab 补全）
			wsCluster.GET("/kubectl", kubectlPod.HandleKubectlPodTerminal)

			// Pod 终端：使用 kubectl exec 连接到 Pod
			wsCluster.GET("/pods/:namespace/:name/terminal", podTerminal.HandlePodTerminal)

			// Pod 日志流式传输
			wsCluster.GET("/pods/:namespace/:name/logs", podHandler.StreamPodLogs)

			// 日志中心 WebSocket 路由
			wsCluster.GET("/logs/stream", logCenterHandler.HandleAggregateLogStream)               // 多Pod聚合日志流
			wsCluster.GET("/logs/pod/:namespace/:name", logCenterHandler.HandleSinglePodLogStream) // 单Pod日志流
		}
	}
}
