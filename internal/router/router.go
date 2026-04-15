package router

import (
	"context"
	"embed"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/features"
	"github.com/shaia/Synapse/internal/tracing"
	"github.com/shaia/Synapse/internal/handlers"
	"github.com/shaia/Synapse/internal/k8s"
	smetrics "github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// staticFS 儲存嵌入的前端靜態檔案系統，由 Setup 注入
var staticFS embed.FS

func Setup(db *gorm.DB, cfg *config.Config, frontendFS embed.FS) (*gin.Engine, *k8s.ClusterInformerManager) {
	staticFS = frontendFS
	r := gin.New()

	// Declare k8sMgr early so the /readyz closure can capture it by reference.
	// It is assigned below after the K8s section; by the time any HTTP request
	// arrives (after Setup returns and the server starts) the value is set.
	var k8sMgr *k8s.ClusterInformerManager

	// ── Distributed tracing (OTel) ─────────────────────────────────────────
	if _, err := tracing.Setup(context.Background(), cfg.Tracing); err != nil {
		logger.Warn("tracing: setup failed, continuing without tracing", "error", err)
	}
	// Register GORM OTel plugin so every DB query gets a span.
	if cfg.Tracing.Enabled {
		if pluginErr := db.Use(otelgorm.NewPlugin()); pluginErr != nil {
			logger.Warn("tracing: failed to attach otelgorm plugin", "error", pluginErr)
		}
	}

	// ── Observability registry (Prometheus) ───────────────────────────────
	var reg *smetrics.Registry
	if cfg.Observability.Enabled {
		reg = smetrics.New()
		// Attach GORM Prometheus callbacks
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
		otelgin.Middleware(cfg.Tracing.ServiceName),
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

	// /readyz — checks DB connectivity and K8s informer state; returns 503 if degraded
	r.GET(readyPath, func(c *gin.Context) {
		checks := gin.H{}
		overallOK := true

		// ── Database ──────────────────────────────────────────────────────
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

		// ── K8s Informers ─────────────────────────────────────────────────
		// Informer sync failures are informational — cluster connectivity is
		// external and should not block pod readiness. Only DB failure → 503.
		if k8sMgr != nil {
			health := k8sMgr.HealthCheck()
			total, synced := len(health), 0
			for _, h := range health {
				if h.Synced {
					synced++
				}
			}
			checks["k8s_informers"] = gin.H{
				"status": "ok",
				"total":  total,
				"synced": synced,
			}
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

	// ── Repositories (P0-4b pilot) ─────────────────────────────────────────
	// Built once and passed into the services that have been migrated to the
	// Repository layer. Constructors are cheap — they only capture the *gorm.DB
	// handle — so doing it here keeps wiring in a single place.
	clusterRepo := repositories.NewClusterRepository(db)
	userRepo := repositories.NewUserRepository(db)
	permissionRepo := repositories.NewPermissionRepository(db)

	// ── Services ───────────────────────────────────────────────────────────
	clusterSvc := services.NewClusterService(db, clusterRepo)
	prometheusSvc := services.NewPrometheusService()
	auditSvc := services.NewAuditService(db)
	permissionSvc := services.NewPermissionService(db, permissionRepo)
	tokenBlacklistSvc := services.NewTokenBlacklistService(db)

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
	k8sMgr = k8s.NewClusterInformerManager()
	if cfg.K8s.InformerSyncTimeout > 0 {
		k8sMgr.SetSyncTimeout(time.Duration(cfg.K8s.InformerSyncTimeout) * time.Second)
	}
	// Attach k8s metrics to manager if observability is on
	if reg != nil {
		k8sMgr.SetMetrics(reg.K8s)
		// Register informer_last_sync_age_seconds — computed on each scrape
		reg.RegisterInformerAgeCollector(k8sMgr.GetSyncAges)
	}

	// Pre-warm Informers for connectable clusters (background, non-blocking)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		clusters, err := clusterSvc.GetConnectableClusters(ctx)
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
	imageIndexWorker := services.NewImageIndexWorker(db, clusterSvc, k8sMgr)

	if reg != nil {
		eventAlertWorker.SetMetrics(reg.Worker)
		costWorker.SetMetrics(reg.Worker)
		logRetentionWorker.SetMetrics(reg.Worker)
		certExpiryWorker.SetMetrics(reg.Worker)
		imageIndexWorker.SetMetrics(reg.Worker)
	}

	eventAlertWorker.Start()
	costWorker.Start()
	logRetentionWorker.Start()
	certExpiryWorker.Start()
	imageIndexWorker.Start()

	k8sMgr.StartGC(30*time.Minute, 2*time.Hour)
	// P2-8: restart informers stuck in un-synced state for > 5 minutes
	k8sMgr.StartHealthWatcher(1*time.Minute, 5*time.Minute)

	// ── API routes ─────────────────────────────────────────────────────────
	api := r.Group("/api/v1")

	// ── Rate limiter (memory or Redis) ─────────────────────────────────────
	loginLimiter := buildRateLimiter(cfg)

	auth := api.Group("/auth")
	{
		authSvc := services.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.ExpireTime, permissionRepo)
		authHandler := handlers.NewAuthHandler(authSvc, opLogSvc, tokenBlacklistSvc)
		auth.POST("/login", middleware.LoginRateLimit(loginLimiter), authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken) // silent refresh with httpOnly cookie
		// Logout 需要認證才能取得 jti 並加入黑名單
		auth.POST("/logout", middleware.AuthRequired(cfg.JWT.Secret, tokenBlacklistSvc), authHandler.Logout)
		auth.GET("/status", authHandler.GetAuthStatus)
		auth.GET("/me", middleware.AuthRequired(cfg.JWT.Secret, tokenBlacklistSvc), authHandler.GetProfile)
		auth.PUT("/me", middleware.AuthRequired(cfg.JWT.Secret, tokenBlacklistSvc), authHandler.UpdateProfile)
		auth.POST("/change-password", middleware.AuthRequired(cfg.JWT.Secret, tokenBlacklistSvc), authHandler.ChangePassword)
	}

	permMiddleware := middleware.NewPermissionMiddleware(permissionSvc)

	// ── Feature Flags (P2-6) ───────────────────────────────────────────────
	featureFlagSvc := services.NewFeatureFlagService(db)
	featureDBStore := features.NewDBStore(db, 30*time.Second)
	// Replace the default env-var store with the DB-backed store so that
	// features.IsEnabled() reflects admin-managed flags at runtime.
	features.SetStore(featureDBStore)

	// ── Pipeline CI/CD subsystem (shared singletons) ─────────────────────
	pipelineSvc := services.NewPipelineService(db)
	pipelineSecretSvc := services.NewPipelineSecretService(db)
	pipelineLogSvc := services.NewPipelineLogService(db)
	pipelineJobBuilder := services.NewJobBuilder()
	pipelineWatcherCfg := services.DefaultJobWatcherConfig()
	pipelineWatcher := services.NewJobWatcher(db, k8sMgr, pipelineWatcherCfg)
	pipelineWatcher.SetLogService(pipelineLogSvc)
	pipelineWatcher.SetRolloutService(services.NewRolloutService())
	pipelineDedup := services.NewNotifyDedup(5 * time.Minute)
	pipelineNotifier := services.NewPipelineNotifier(db, pipelineDedup)
	pipelineScheduler := services.NewPipelineScheduler(
		db, pipelineJobBuilder, pipelineSecretSvc, k8sMgr,
		pipelineWatcher, pipelineNotifier, services.DefaultSchedulerConfig(),
	)
	_ = pipelineSecretSvc // used by webhook routes below
	_ = pipelineLogSvc    // used by pipeline routes below

	// M15: wire registry service for push-image credential injection
	pipelineScheduler.SetRegistryService(services.NewRegistryService(db))

	// Pipeline startup: recover interrupted runs → start scheduler + watcher
	pipelineRecoverer := services.NewPipelineRecover(db, pipelineWatcher, services.DefaultRecoverConfig())
	recoverCtx, recoverCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := pipelineRecoverer.Recover(recoverCtx); err != nil {
		logger.Warn("pipeline recovery completed with errors", "error", err)
	}
	recoverCancel()
	pipelineScheduler.Start()

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
		tokenBlacklist:   tokenBlacklistSvc,
		permMiddleware:   permMiddleware,
		logSourceSvc:     services.NewLogSourceService(db),
		portForwardSvc:   services.NewPortForwardService(db),
		helmSvc:          services.NewHelmService(db),
		approvalSvc:      services.NewApprovalService(db),
		cfgVerSvc:        services.NewConfigVersionService(db),
		imageIndexSvc:    services.NewImageIndexService(db),
		syncPolicySvc:    services.NewSyncPolicyService(db),
		featureFlagSvc:   featureFlagSvc,
		featureDBStore:   featureDBStore,
		pipelineScheduler: pipelineScheduler,
		pipelineSvc:       pipelineSvc,
		gitProviderSvc:    services.NewGitProviderService(db),
		projectSvc:        services.NewProjectService(db),
		registrySvc:       services.NewRegistryService(db),
		tagRetentionSvc:   services.NewTagRetentionService(db, services.NewRegistryService(db)),
	}

	// ── Webhook routes (public, HMAC-authenticated) ───────────────────────
	registerWebhookRoutes(api, &deps)

	protected := api.Group("")
	protected.Use(middleware.AuthRequired(cfg.JWT.Secret, tokenBlacklistSvc))
	protected.Use(middleware.APIRateLimit("api", 300)) // 300 req/min per user (P0-2)
	{
		userSvc := services.NewUserService(db, userRepo)
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

		// §6.3 多叢集拓樸 — global route (not under /clusters/:clusterID)
		mcTopoHandler := handlers.NewMultiClusterTopologyHandler(deps.clusterSvc, deps.k8sMgr)
		protected.GET("/network/multi-cluster-topology", mcTopoHandler.GetMultiClusterTopology)

		// Top-level Pipeline routes (cluster-independent)
		registerPipelineRoutes(protected, &deps)

		clusters := protected.Group("/clusters")
		registerSystemRoutes(protected, clusters, &deps)
	}

	registerWSRoutes(r, &deps)

	setupStatic(r)

	return r, k8sMgr
}

// buildRateLimiter constructs the appropriate RateLimiter backend based on
// cfg.RateLimiter.Backend. When backend is "redis", a Redis client is created
// using cfg.Redis; if the ping fails, it falls back to in-memory with a warning.
// All other values use the in-memory implementation.
func buildRateLimiter(cfg *config.Config) middleware.RateLimiter {
	if cfg.RateLimiter.Backend != "redis" {
		logger.Info("rate limiter: using in-memory backend (single-pod only)")
		return middleware.NewMemoryRateLimiter()
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Warn("rate limiter: Redis ping failed, falling back to in-memory",
			"addr", cfg.Redis.Addr, "error", err)
		_ = client.Close()
		return middleware.NewMemoryRateLimiter()
	}

	logger.Info("rate limiter: using Redis backend", "addr", cfg.Redis.Addr)
	return middleware.NewRedisRateLimiter(client)
}

