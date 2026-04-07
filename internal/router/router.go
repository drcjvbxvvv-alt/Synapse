package router

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/k8s"
	smetrics "github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// staticFS 儲存嵌入的前端靜態檔案系統，由 Setup 注入
var staticFS embed.FS

func Setup(db *gorm.DB, cfg *config.Config, frontendFS embed.FS) (*gin.Engine, *k8s.ClusterInformerManager) {
	staticFS = frontendFS
	r := gin.New()

	// ── Observability registry ─────────────────────────────────────────────
	var reg *smetrics.Registry
	if cfg.Observability.Enabled {
		reg = smetrics.New()
		// Attach GORM callbacks
		reg.DB.Register(db)
	}

	// ── Global middleware ──────────────────────────────────────────────────
	var httpMetrics *smetrics.HTTPMetrics
	if reg != nil {
		httpMetrics = reg.HTTP
	}
	r.Use(
		middleware.RequestID(),
		gin.Recovery(),
		gin.Logger(),
		middleware.CORS(),
	)

	// Build opLogSvc early so OperationAudit middleware can be set up
	opLogSvc := services.NewOperationLogService(db)
	r.Use(
		middleware.OperationAudit(opLogSvc),
		middleware.PrometheusMetrics(httpMetrics),
		gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/ws/"})),
	)

	// ── Observability endpoints ────────────────────────────────────────────
	obsCfg := cfg.Observability
	healthPath := obsCfg.HealthPath
	if healthPath == "" {
		healthPath = "/healthz"
	}
	readyPath := obsCfg.ReadyPath
	if readyPath == "" {
		readyPath = "/readyz"
	}

	startTime := time.Now()

	// /healthz — always responds 200 as long as the process is alive
	r.GET(healthPath, func(c *gin.Context) {
		response.OK(c, gin.H{
			"status": "ok",
			"uptime": time.Since(startTime).Round(time.Second).String(),
		})
	})

	// /readyz — checks DB connectivity; returns 503 if degraded
	r.GET(readyPath, func(c *gin.Context) {
		checks := gin.H{}
		overallOK := true

		sqlDB, err := db.DB()
		if err != nil {
			checks["database"] = gin.H{"status": "error", "message": err.Error()}
			overallOK = false
		} else if err = sqlDB.Ping(); err != nil {
			checks["database"] = gin.H{"status": "error", "message": err.Error()}
			overallOK = false
		} else {
			checks["database"] = gin.H{"status": "ok"}
		}

		status := "ok"
		if !overallOK {
			status = "degraded"
		}
		body := gin.H{"status": status, "checks": checks}
		if !overallOK {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "data": body})
			return
		}
		response.OK(c, body)
	})

	// /metrics — only when observability is enabled
	if reg != nil {
		metricsPath := obsCfg.MetricsPath
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		metricsHandler := reg.Handler()
		if obsCfg.MetricsToken != "" {
			expectedAuth := "Bearer " + obsCfg.MetricsToken
			r.GET(metricsPath, func(c *gin.Context) {
				if c.GetHeader("Authorization") != expectedAuth {
					c.Header("WWW-Authenticate", `Bearer realm="synapse-metrics"`)
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				metricsHandler.ServeHTTP(c.Writer, c.Request)
			})
		} else {
			r.GET(metricsPath, gin.WrapH(metricsHandler))
		}
		logger.Info("Observability enabled",
			"metrics", metricsPath,
			"health", healthPath,
			"ready", readyPath,
			"token_auth", obsCfg.MetricsToken != "",
		)
	}

	// ── Services ───────────────────────────────────────────────────────────
	clusterSvc := services.NewClusterService(db)
	prometheusSvc := services.NewPrometheusService()
	auditSvc := services.NewAuditService(db)
	permissionSvc := services.NewPermissionService(db)

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
	monitoringConfigSvc := services.NewMonitoringConfigServiceWithGrafana(db, grafanaSvc)

	// ── K8s Informer manager ───────────────────────────────────────────────
	k8sMgr := k8s.NewClusterInformerManager()
	if cfg.K8s.InformerSyncTimeout > 0 {
		k8sMgr.SetSyncTimeout(time.Duration(cfg.K8s.InformerSyncTimeout) * time.Second)
	}
	// Attach k8s metrics to manager if observability is on
	if reg != nil {
		k8sMgr.SetMetrics(reg.K8s)
	}

	// Pre-warm Informers for connectable clusters (background, non-blocking)
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

	// ── Background workers ─────────────────────────────────────────────────
	eventAlertWorker := services.NewEventAlertWorker(db, k8sMgr, clusterSvc)
	costWorker := services.NewCostWorker(db, clusterSvc, k8sMgr)
	logRetentionWorker := services.NewLogRetentionWorker(db, 0)
	certExpiryWorker := services.NewCertExpiryWorker(db)

	if reg != nil {
		eventAlertWorker.SetMetrics(reg.Worker)
		costWorker.SetMetrics(reg.Worker)
		logRetentionWorker.SetMetrics(reg.Worker)
		certExpiryWorker.SetMetrics(reg.Worker)
	}

	eventAlertWorker.Start()
	costWorker.Start()
	logRetentionWorker.Start()
	certExpiryWorker.Start()

	k8sMgr.StartGC(30*time.Minute, 2*time.Hour)

	// ── API routes ─────────────────────────────────────────────────────────
	api := r.Group("/api/v1")

	auth := api.Group("/auth")
	{
		authSvc := services.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.ExpireTime)
		authHandler := handlers.NewAuthHandler(authSvc, opLogSvc)
		auth.POST("/login", middleware.LoginRateLimit(), authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/status", authHandler.GetAuthStatus)
		auth.GET("/me", middleware.AuthRequired(cfg.JWT.Secret), authHandler.GetProfile)
		auth.POST("/change-password", middleware.AuthRequired(cfg.JWT.Secret), authHandler.ChangePassword)
	}

	permMiddleware := middleware.NewPermissionMiddleware(permissionSvc)

	deps := routeDeps{
		db:               db,
		cfg:              cfg,
		k8sMgr:           k8sMgr,
		metrics:          reg,
		clusterSvc:       clusterSvc,
		prometheusSvc:    prometheusSvc,
		opLogSvc:         opLogSvc,
		permissionSvc:    permissionSvc,
		auditSvc:         auditSvc,
		grafanaSvc:       grafanaSvc,
		monitoringCfgSvc: monitoringConfigSvc,
		permMiddleware:   permMiddleware,
	}

	protected := api.Group("")
	protected.Use(middleware.AuthRequired(cfg.JWT.Secret))
	{
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

	setupStatic(r)

	return r, k8sMgr
}

// setupStatic 配置嵌入的前端靜態檔案服務
func setupStatic(r *gin.Engine) {
	assetsFS, err := fs.Sub(staticFS, "ui/dist/assets")
	if err != nil {
		logger.Error("載入前端靜態資源失敗", "error", err)
		return
	}

	assetsGroup := r.Group("/assets")
	assetsGroup.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Next()
	})
	assetsGroup.StaticFS("/", http.FS(assetsFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
			response.NotFound(c, "not found")
			return
		}

		filePath := strings.TrimPrefix(path, "/")
		if filePath != "" {
			if f, err := staticFS.Open("ui/dist/" + filePath); err == nil {
				_ = f.Close()
				fileServer := http.FileServer(http.FS(mustSub(staticFS, "ui/dist")))
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		content, err := staticFS.ReadFile("ui/dist/index.html")
		if err != nil {
			response.InternalError(c, "frontend not available")
			return
		}
		c.Data(200, "text/html; charset=utf-8", content)
	})
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(fmt.Sprintf("fs.Sub(%q): %v", dir, err))
	}
	return sub
}
