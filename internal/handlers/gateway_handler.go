package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// GatewayHandler 管理 Gateway API 資源（Phase 1：唯讀）
type GatewayHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewGatewayHandler 建立 GatewayHandler
func NewGatewayHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *GatewayHandler {
	return &GatewayHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// getGatewaySvc 取得已驗證的 GatewayService（含 CRD 可用性檢查）
func (h *GatewayHandler) getGatewaySvc(c *gin.Context, checkAvailable bool) (*services.GatewayService, bool) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return nil, false
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("取得叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return nil, false
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("取得 K8s 客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 K8s 客戶端失敗: %v", err))
		return nil, false
	}

	svc, err := services.NewGatewayService(k8sClient)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("建立 Gateway client 失敗: %v", err))
		return nil, false
	}

	if checkAvailable {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		if !svc.IsGatewayAPIAvailable(ctx) {
			response.BadRequest(c, "此叢集尚未安裝 Gateway API CRD。請執行：kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/latest/download/standard-install.yaml")
			return nil, false
		}
	}

	return svc, true
}

// GetGatewayAPIStatus 偵測叢集是否安裝 Gateway API CRD
// GET /api/v1/clusters/:clusterID/gateway/status
func (h *GatewayHandler) GetGatewayAPIStatus(c *gin.Context) {
	svc, ok := h.getGatewaySvc(c, false)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	available := svc.IsGatewayAPIAvailable(ctx)
	response.OK(c, gin.H{"available": available})
}
