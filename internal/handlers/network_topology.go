package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// NetworkTopologyHandler 叢集網路拓樸處理器（Phase 4 Phase A：靜態拓樸）
type NetworkTopologyHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewNetworkTopologyHandler 建立 NetworkTopologyHandler
func NewNetworkTopologyHandler(
	clusterService *services.ClusterService,
	k8sMgr *k8s.ClusterInformerManager,
) *NetworkTopologyHandler {
	return &NetworkTopologyHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// GetClusterTopology 取得叢集網路拓樸（靜態：Services + Workloads）
// GET /clusters/:clusterID/network/topology?namespaces=ns1,ns2
func (h *NetworkTopologyHandler) GetClusterTopology(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("取得叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("取得 K8s 客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, "取得 K8s 客戶端失敗")
		return
	}

	// Parse namespace filter (comma-separated)
	var namespaces []string
	if nsParam := c.Query("namespaces"); nsParam != "" {
		for _, ns := range strings.Split(nsParam, ",") {
			if ns = strings.TrimSpace(ns); ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	topo, err := services.GetClusterNetworkTopology(ctx, k8sClient.GetClientset(), namespaces)
	if err != nil {
		logger.Error("取得叢集網路拓樸失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, "取得叢集網路拓樸失敗")
		return
	}

	response.OK(c, topo)
}
