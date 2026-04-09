package router

import (
	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	smetrics "github.com/shaia/Synapse/internal/metrics"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
	"gorm.io/gorm"
)

// routeDeps holds all shared dependencies for route registration functions.
type routeDeps struct {
	db               *gorm.DB
	cfg              *config.Config
	k8sMgr           *k8s.ClusterInformerManager
	metrics          *smetrics.Registry
	clusterSvc       *services.ClusterService
	prometheusSvc    *services.PrometheusService
	opLogSvc         *services.OperationLogService
	permissionSvc    *services.PermissionService
	auditSvc         *services.AuditService
	grafanaSvc       *services.GrafanaService
	monitoringCfgSvc *services.MonitoringConfigService
	tokenBlacklist   *services.TokenBlacklistService
	permMiddleware   *middleware.PermissionMiddleware
}
