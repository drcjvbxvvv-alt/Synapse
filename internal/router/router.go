package router

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/handlers"
	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// staticFS 儲存嵌入的前端靜態檔案系統，由 Setup 注入
var staticFS embed.FS

func Setup(db *gorm.DB, cfg *config.Config, frontendFS embed.FS) (*gin.Engine, *k8s.ClusterInformerManager) {
	staticFS = frontendFS
	r := gin.New()

	// 根據環境設定 gin 模式（可選）
	// if cfg.Server.Mode == "release" {
	// 	gin.SetMode(gin.ReleaseMode)
	// }

	// 建立操作審計日誌服務
	opLogSvc := services.NewOperationLogService(db)

	// 全域性中介軟體
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

	// 統一的 Service 例項，避免重複建立
	clusterSvc := services.NewClusterService(db)
	prometheusSvc := services.NewPrometheusService()
	auditSvc := services.NewAuditService(db)           // 審計服務
	permissionSvc := services.NewPermissionService(db) // 權限服務

	// 初始化 Grafana 服務（始終建立例項，從資料庫讀取配置，env 僅控制代理和自動同步）
	grafanaSettingSvc := services.NewGrafanaSettingService(db)
	grafanaSvc := services.NewGrafanaService("", "")
	grafanaConfig, err := grafanaSettingSvc.GetGrafanaConfig()
	if err != nil {
		logger.Error("讀取 Grafana 配置失敗", "error", err)
	} else if grafanaConfig.URL != "" && grafanaConfig.APIKey != "" {
		grafanaSvc.UpdateConfig(grafanaConfig.URL, grafanaConfig.APIKey)
		if err := grafanaSvc.TestConnection(); err != nil {
			logger.Warn("Grafana 連線測試失敗，資料來源同步將被禁用", "error", err)
		} else {
			logger.Info("Grafana 服務已啟用（配置來自資料庫）", "url", grafanaConfig.URL)
		}
	} else {
		logger.Info("Grafana 尚未配置連線資訊，請在系統設定中配置")
	}
	// 始終將 grafanaSvc 傳給 monitoringConfigSvc，執行時透過 IsEnabled() 判斷是否同步資料來源
	monitoringConfigSvc := services.NewMonitoringConfigServiceWithGrafana(db, grafanaSvc)
	// K8s Informer 管理器（套用可設定的快取同步逾時）
	k8sMgr := k8s.NewClusterInformerManager()
	if cfg.K8s.InformerSyncTimeout > 0 {
		k8sMgr.SetSyncTimeout(time.Duration(cfg.K8s.InformerSyncTimeout) * time.Second)
	}
	// 預熱可連線叢集的 Informer（後臺執行，不阻塞啟動；跳過 unhealthy 叢集，並行初始化）
	go func() {
		clusters, err := clusterSvc.GetConnectableClusters()
		if err != nil {
			logger.Error("預熱 informer 失敗", "error", err)
			return
		}
		var wg sync.WaitGroup
		for _, cl := range clusters {
			wg.Add(1)
			go func(cl *models.Cluster) {
				defer wg.Done()
				if _, err := k8sMgr.EnsureForCluster(cl); err != nil {
					logger.Warn("初始化叢集 informer 失敗，跳過", "cluster", cl.Name, "error", err)
				}
			}(cl)
		}
		wg.Wait()
	}()

	// 啟動 Event 告警工作器（後臺週期掃描 K8s Events 並比對規則）
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

	// Auth 僅開放登入與登出，其餘走受保護分組
	auth := api.Group("/auth")
	{
		authSvc := services.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.ExpireTime)
		authHandler := handlers.NewAuthHandler(authSvc, opLogSvc)
		auth.POST("/login", middleware.LoginRateLimit(), authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/status", authHandler.GetAuthStatus) // 獲取認證狀態（無需登入）
		// /me 必須帶 Auth
		auth.GET("/me", middleware.AuthRequired(cfg.JWT.Secret), authHandler.GetProfile)
		auth.POST("/change-password", middleware.AuthRequired(cfg.JWT.Secret), authHandler.ChangePassword)
	}

	// 建立權限中介軟體（在受保護路由和 WebSocket 路由中共用）
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

	// 受保護的業務路由
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(cfg.JWT.Secret))
	{
		// users - 使用者管理（僅平臺管理員）
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

	// 嵌入前端靜態檔案服務
	setupStatic(r)

	// TODO:
	// - 統一錯誤處理/響應格式中介軟體
	// - OpenAPI/Swagger 文件路由（/swagger/*any）
	return r, k8sMgr
}

// setupStatic 配置嵌入的前端靜態檔案服務
func setupStatic(r *gin.Engine) {
	assetsFS, err := fs.Sub(staticFS, "ui/dist/assets")
	if err != nil {
		logger.Error("載入前端靜態資源失敗", "error", err)
		return
	}

	// 靜態資源快取（assets 目錄包含帶 hash 的檔案，可以長期快取）
	assetsGroup := r.Group("/assets")
	assetsGroup.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Next()
	})
	assetsGroup.StaticFS("/", http.FS(assetsFS))

	// 所有未匹配的路由回退到 index.html（SPA 路由支援）
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 和 WebSocket 路徑返回 404
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
			response.NotFound(c, "not found")
			return
		}

		// 嘗試提供靜態檔案（如 favicon.ico 等根目錄檔案）
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

// mustSub 是 fs.Sub 的便捷封裝
func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
