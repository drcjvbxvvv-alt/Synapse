package handlers

import (
	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/services"
)

// NodeHandler 節點處理器
type NodeHandler struct {
	cfg              *config.Config
	clusterService   *services.ClusterService
	k8sMgr           *k8s.ClusterInformerManager
	promService      services.PrometheusQuerier
	monitoringCfgSvc *services.MonitoringConfigService
}

// NewNodeHandler 建立節點處理器
func NewNodeHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager, promService services.PrometheusQuerier, monitoringCfgSvc *services.MonitoringConfigService) *NodeHandler {
	return &NodeHandler{
		cfg:              cfg,
		clusterService:   clusterService,
		k8sMgr:           k8sMgr,
		promService:      promService,
		monitoringCfgSvc: monitoringCfgSvc,
	}
}
