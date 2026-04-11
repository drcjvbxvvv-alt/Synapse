package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// MultiClusterTopologyHandler serves the federation topology endpoint.
type MultiClusterTopologyHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewMultiClusterTopologyHandler wires dependencies.
func NewMultiClusterTopologyHandler(
	clusterSvc *services.ClusterService,
	k8sMgr *k8s.ClusterInformerManager,
) *MultiClusterTopologyHandler {
	return &MultiClusterTopologyHandler{
		clusterService: clusterSvc,
		k8sMgr:         k8sMgr,
	}
}

// GetMultiClusterTopology returns the aggregated federation topology.
// GET /api/v1/network/multi-cluster-topology?clusterIDs=1,2,3
func (h *MultiClusterTopologyHandler) GetMultiClusterTopology(c *gin.Context) {
	// ── Step 1: Parse clusterIDs ──────────────────────────────────────────
	rawIDs := c.Query("clusterIDs")
	if rawIDs == "" {
		response.BadRequest(c, "clusterIDs 參數不能為空")
		return
	}

	var clusterIDs []uint
	for _, s := range strings.Split(rawIDs, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			response.BadRequest(c, fmt.Sprintf("無效的叢集 ID：%s", s))
			return
		}
		clusterIDs = append(clusterIDs, uint(id))
	}
	if len(clusterIDs) == 0 {
		response.BadRequest(c, "clusterIDs 參數不能為空")
		return
	}
	if len(clusterIDs) > 10 {
		response.BadRequest(c, "一次最多查詢 10 個叢集")
		return
	}

	// ── Step 2: Resolve clusters + K8s clients ────────────────────────────
	inputs := make([]services.ClusterTopoInput, 0, len(clusterIDs))
	for _, id := range clusterIDs {
		cluster, err := h.clusterService.GetCluster(id)
		if err != nil {
			logger.Warn("multi-cluster topology: cluster not found, skipping",
				"cluster_id", id)
			continue
		}
		k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
		if err != nil {
			logger.Warn("multi-cluster topology: K8s client unavailable, skipping",
				"cluster_id", id, "error", err)
			continue
		}
		inputs = append(inputs, services.ClusterTopoInput{
			ID:        id,
			Name:      cluster.Name,
			Clientset: k8sClient.GetClientset(),
		})
	}
	if len(inputs) == 0 {
		response.BadRequest(c, "所有指定叢集均無法取得 K8s 客戶端")
		return
	}

	// ── Step 3: Context with timeout ─────────────────────────────────────
	// 60s: parallel K8s List calls across N clusters can take longer than single-cluster
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// ── Step 4: Fetch multi-cluster topology ─────────────────────────────
	logger.Info("fetching multi-cluster topology",
		"cluster_count", len(inputs))

	topo, err := services.GetMultiClusterTopology(ctx, inputs)
	if err != nil {
		logger.Error("multi-cluster topology failed", "error", err)
		response.InternalError(c, "多叢集拓樸取得失敗："+err.Error())
		return
	}

	// ── Step 5: Response ─────────────────────────────────────────────────
	response.OK(c, topo)
}
