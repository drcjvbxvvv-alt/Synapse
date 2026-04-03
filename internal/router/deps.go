package router

import (
	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"gorm.io/gorm"
)

// routeDeps holds all shared dependencies for route registration functions.
type routeDeps struct {
	db               *gorm.DB
	cfg              *config.Config
	k8sMgr           *k8s.ClusterInformerManager
	clusterSvc       *services.ClusterService
	prometheusSvc    *services.PrometheusService
	opLogSvc         *services.OperationLogService
	permissionSvc    *services.PermissionService
	auditSvc         *services.AuditService
	grafanaSvc       *services.GrafanaService
	monitoringCfgSvc *services.MonitoringConfigService
	permMiddleware   *middleware.PermissionMiddleware
}
