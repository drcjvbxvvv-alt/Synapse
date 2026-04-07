package router

import (
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerWSRoutes registers all /ws WebSocket routes.
func registerWSRoutes(r *gin.Engine, d *routeDeps) {
	// WebSocket：建議也加認證
	ws := r.Group("/ws")
	ws.Use(middleware.AuthRequired(d.cfg.JWT.Secret))
	{
		// 終端處理器（注入審計服務）
		kctl := handlers.NewKubectlTerminalHandler(d.clusterSvc, d.auditSvc)
		ssh := handlers.NewSSHHandler(d.auditSvc)
		podTerminal := handlers.NewPodTerminalHandler(d.clusterSvc, d.auditSvc, d.k8sMgr)
		kubectlPod := handlers.NewKubectlPodTerminalHandler(d.clusterSvc, d.auditSvc, d.k8sMgr)
		podHandler := handlers.NewPodHandler(d.db, d.cfg, d.clusterSvc, d.k8sMgr)
		logCenterHandler := handlers.NewLogCenterHandler(d.clusterSvc, d.k8sMgr)

		// 節點 SSH 終端（需要平臺管理員權限）
		ws.GET("/ssh/terminal", middleware.PlatformAdminRequired(d.db), ssh.SSHConnect)

		// 叢集相關的 WebSocket 路由（需要叢集權限檢查）
		wsCluster := ws.Group("/clusters/:clusterID")
		wsCluster.Use(d.permMiddleware.ClusterAccessRequired()) // 啟用叢集權限檢查
		{
			// 叢集級 kubectl 終端（舊方案：本地執行）
			wsCluster.GET("/terminal", kctl.HandleKubectlTerminal)

			// 叢集級 kubectl 終端（新方案：Pod 模式，支援 tab 補全）
			wsCluster.GET("/kubectl", kubectlPod.HandleKubectlPodTerminal)

			// Pod 終端：使用 kubectl exec 連線到 Pod
			wsCluster.GET("/pods/:namespace/:name/terminal", podTerminal.HandlePodTerminal)

			// Pod 日誌流式傳輸
			wsCluster.GET("/pods/:namespace/:name/logs", podHandler.StreamPodLogs)

			// 日誌中心 WebSocket 路由
			wsCluster.GET("/logs/stream", logCenterHandler.HandleAggregateLogStream)               // 多Pod聚合日誌流
			wsCluster.GET("/logs/pod/:namespace/:name", logCenterHandler.HandleSinglePodLogStream) // 單Pod日誌流
		}
	}
}
