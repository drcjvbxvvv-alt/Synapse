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

// NetworkTopologyHandler 叢集網路拓樸處理器（Phase 4）
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

// GetIntegrations 偵測叢集是否安裝 Cilium / Istio
// GET /clusters/:clusterID/network/integrations
func (h *NetworkTopologyHandler) GetIntegrations(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	status := services.DetectIntegrations(ctx, k8sClient.GetClientset())
	response.OK(c, status)
}

// GetClusterTopology 取得叢集網路拓樸
// GET /clusters/:clusterID/network/topology?namespaces=ns1,ns2&enrich=true
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

	clientset := k8sClient.GetClientset()

	// Parse namespace filter (comma-separated)
	var namespaces []string
	if nsParam := c.Query("namespaces"); nsParam != "" {
		for _, ns := range strings.Split(nsParam, ",") {
			if ns = strings.TrimSpace(ns); ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}

	enrich := c.Query("enrich") == "true"
	policy := c.Query("policy") == "true"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Build static topology
	topo, err := services.GetClusterNetworkTopology(ctx, clientset, namespaces)
	if err != nil {
		logger.Error("取得叢集網路拓樸失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, "取得叢集網路拓樸失敗")
		return
	}

	// Phase B: Optionally enrich with Istio metrics
	if enrich {
		integCtx, integCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		status := services.DetectIntegrations(integCtx, clientset)
		integCancel()

		if status.Istio {
			metricsCtx, metricsCancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
			metrics, err := services.QueryIstioMetrics(metricsCtx, clientset)
			metricsCancel()
			if err != nil {
				logger.Warn("Istio metrics 查詢失敗（繼續返回靜態拓樸）", "error", err)
			} else {
				topo.EnrichWithIstioMetrics(metrics)
			}
		}
	}

	// Phase E: Optionally overlay NetworkPolicy status
	if policy {
		policyCtx, policyCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		if err := topo.InferNetworkPolicies(policyCtx, clientset, namespaces); err != nil {
			logger.Warn("NetworkPolicy 推論失敗（繼續返回拓樸）", "error", err)
		}
		policyCancel()
	}

	response.OK(c, topo)
}
