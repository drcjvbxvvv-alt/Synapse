package router

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/handlers"
	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// staticFS 保存嵌入的前端静态文件系统，由 Setup 注入
var staticFS embed.FS

func Setup(db *gorm.DB, cfg *config.Config, frontendFS embed.FS) (*gin.Engine, *k8s.ClusterInformerManager) {
	staticFS = frontendFS
	r := gin.New()

	// 根据环境设置 gin 模式（可选）
	// if cfg.Server.Mode == "release" {
	// 	gin.SetMode(gin.ReleaseMode)
	// }

	// 创建操作审计日志服务
	opLogSvc := services.NewOperationLogService(db)

	// 全局中間件
	r.Use(
		middleware.RequestID(),              // 注入 X-Request-ID
		gin.Recovery(),
		gin.Logger(),
		middleware.CORS(),
		middleware.OperationAudit(opLogSvc),
		middleware.PrometheusMetrics(),      // 應用層 Prometheus 指標
		gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/ws/"})),
	)

	// Prometheus metrics（不掛 Gzip，讓 Prometheus scraper 直接讀取）
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health endpoints：liveness 與 readiness
	r.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			response.Error(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database unavailable")
			return
		}
		if err := sqlDB.Ping(); err != nil {
			response.Error(c, http.StatusServiceUnavailable, "DB_PING_FAILED", "database ping failed")
			return
		}
		response.OK(c, gin.H{"ready": true, "db": "ok"})
	})

	// 统一的 Service 实例，避免重复创建
	clusterSvc := services.NewClusterService(db)
	prometheusSvc := services.NewPrometheusService()
	auditSvc := services.NewAuditService(db)           // 审计服务
	permissionSvc := services.NewPermissionService(db) // 权限服务

	// 初始化 Grafana 服务（始终创建实例，从数据库读取配置，env 仅控制代理和自动同步）
	grafanaSettingSvc := services.NewGrafanaSettingService(db)
	grafanaSvc := services.NewGrafanaService("", "")
	grafanaConfig, err := grafanaSettingSvc.GetGrafanaConfig()
	if err != nil {
		logger.Error("读取 Grafana 配置失败", "error", err)
	} else if grafanaConfig.URL != "" && grafanaConfig.APIKey != "" {
		grafanaSvc.UpdateConfig(grafanaConfig.URL, grafanaConfig.APIKey)
		if err := grafanaSvc.TestConnection(); err != nil {
			logger.Warn("Grafana 连接测试失败，数据源同步将被禁用", "error", err)
		} else {
			logger.Info("Grafana 服务已启用（配置来自数据库）", "url", grafanaConfig.URL)
		}
	} else {
		logger.Info("Grafana 尚未配置连接信息，请在系统设置中配置")
	}
	// 始终将 grafanaSvc 传给 monitoringConfigSvc，运行时通过 IsEnabled() 判断是否同步数据源
	monitoringConfigSvc := services.NewMonitoringConfigServiceWithGrafana(db, grafanaSvc)
	// K8s Informer 管理器（套用可設定的快取同步逾時）
	k8sMgr := k8s.NewClusterInformerManager()
	if cfg.K8s.InformerSyncTimeout > 0 {
		k8sMgr.SetSyncTimeout(time.Duration(cfg.K8s.InformerSyncTimeout) * time.Second)
	}
	// 预热所有已存在集群的 Informer（后台执行，不阻塞启动）
	go func() {
		clusters, err := clusterSvc.GetAllClusters()
		if err != nil {
			logger.Error("预热 informer 失败", "error", err)
			return
		}
		for _, cl := range clusters {
			if _, err := k8sMgr.EnsureForCluster(cl); err != nil {
				logger.Error("初始化集群 informer 失败", "cluster", cl.Name, "error", err)
			}
		}
	}()

	// 啟動 Event 告警工作器（後台週期掃描 K8s Events 並比對規則）
	eventAlertWorker := services.NewEventAlertWorker(db, k8sMgr, clusterSvc)
	eventAlertWorker.Start()

	// 啟動成本快照工作器（每日 00:05 UTC 拍攝資源用量快照）
	costWorker := services.NewCostWorker(db, clusterSvc)
	costWorker.Start()

	// 啟動日誌保留清理工作器（預設保留 90 天）
	logRetentionWorker := services.NewLogRetentionWorker(db, 0)
	logRetentionWorker.Start()

	// 啟動閒置叢集 GC（每 30 分鐘掃描，閒置超過 2 小時則停止 informer）
	k8sMgr.StartGC(30*time.Minute, 2*time.Hour)

	// /api/v1
	api := r.Group("/api/v1")

	// Auth 仅开放登录与登出，其余走受保护分组
	auth := api.Group("/auth")
	{
		authSvc := services.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.ExpireTime)
		authHandler := handlers.NewAuthHandler(authSvc, opLogSvc)
		auth.POST("/login", middleware.LoginRateLimit(), authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/status", authHandler.GetAuthStatus) // 获取认证状态（无需登录）
		// /me 必须带 Auth
		auth.GET("/me", middleware.AuthRequired(cfg.JWT.Secret), authHandler.GetProfile)
		auth.POST("/change-password", middleware.AuthRequired(cfg.JWT.Secret), authHandler.ChangePassword)
	}

	// 创建权限中间件（在受保护路由和 WebSocket 路由中共用）
	permMiddleware := middleware.NewPermissionMiddleware(permissionSvc)

	deps := routeDeps{
		db:               db,
		cfg:              cfg,
		k8sMgr:           k8sMgr,
		clusterSvc:       clusterSvc,
		prometheusSvc:    prometheusSvc,
		opLogSvc:         opLogSvc,
		permissionSvc:    permissionSvc,
		auditSvc:         auditSvc,
		grafanaSvc:       grafanaSvc,
		monitoringCfgSvc: monitoringConfigSvc,
		permMiddleware:   permMiddleware,
	}

	// 受保护的业务路由
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(cfg.JWT.Secret))
	{
		// users - 用户管理（仅平台管理员）
		userSvc := services.NewUserService(db)
		userHandler := handlers.NewUserHandler(userSvc)
		users := protected.Group("/users")
		users.Use(middleware.PlatformAdminRequired(db))
		{
			users.GET("", userHandler.ListUsers)
			users.POST("", userHandler.CreateUser)
			users.GET("/:id", userHandler.GetUser)
			users.PUT("/:id", userHandler.UpdateUser)
			users.DELETE("/:id", userHandler.DeleteUser)
			users.PUT("/:id/status", userHandler.UpdateUserStatus)
			users.PUT("/:id/reset-password", userHandler.ResetPassword)
		}

		registerClusterRoutes(protected, &deps)
		clusters := protected.Group("/clusters")
		registerSystemRoutes(protected, clusters, &deps)
	}

	registerWSRoutes(r, &deps)

	// 嵌入前端静态文件服务
	setupStatic(r)

	// TODO:
	// - 统一错误处理/响应格式中间件
	// - OpenAPI/Swagger 文档路由（/swagger/*any）
	return r, k8sMgr
}

// setupStatic 配置嵌入的前端静态文件服务
func setupStatic(r *gin.Engine) {
	assetsFS, err := fs.Sub(staticFS, "ui/dist/assets")
	if err != nil {
		logger.Error("加载前端静态资源失败", "error", err)
		return
	}

	// 静态资源缓存（assets 目录包含带 hash 的文件，可以长期缓存）
	assetsGroup := r.Group("/assets")
	assetsGroup.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Next()
	})
	assetsGroup.StaticFS("/", http.FS(assetsFS))

	// 所有未匹配的路由回退到 index.html（SPA 路由支持）
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 和 WebSocket 路径返回 404
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
			response.NotFound(c, "not found")
			return
		}

		// 尝试提供静态文件（如 favicon.ico 等根目录文件）
		filePath := strings.TrimPrefix(path, "/")
		if filePath != "" {
			if f, err := staticFS.Open("ui/dist/" + filePath); err == nil {
				_ = f.Close()
				fileServer := http.FileServer(http.FS(mustSub(staticFS, "ui/dist")))
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		// 回退到 index.html
		content, err := staticFS.ReadFile("ui/dist/index.html")
		if err != nil {
			response.InternalError(c, "frontend not available")
			return
		}
		c.Data(200, "text/html; charset=utf-8", content)
	})
}

// mustSub 是 fs.Sub 的便捷封装
func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
