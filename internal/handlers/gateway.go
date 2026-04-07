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
	"gorm.io/gorm"
)

// GatewayHandler 管理 Gateway API 資源（Phase 1：唯讀）
type GatewayHandler struct {
	db             *gorm.DB
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewGatewayHandler 建立 GatewayHandler
func NewGatewayHandler(db *gorm.DB, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *GatewayHandler {
	return &GatewayHandler{
		db:             db,
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

// --- GatewayClass ---

// ListGatewayClasses 列出所有 GatewayClass（cluster-scoped）
// GET /api/v1/clusters/:clusterID/gatewayclasses
func (h *GatewayHandler) ListGatewayClasses(c *gin.Context) {
	logger.Info("列出 GatewayClass")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := svc.ListGatewayClasses(ctx)
	if err != nil {
		logger.Error("列出 GatewayClass 失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("列出 GatewayClass 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetGatewayClass 取得單一 GatewayClass
// GET /api/v1/clusters/:clusterID/gatewayclasses/:name
func (h *GatewayHandler) GetGatewayClass(c *gin.Context) {
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.GetGatewayClass(ctx, name)
	if err != nil {
		logger.Error("取得 GatewayClass 失敗", "error", err, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 GatewayClass 失敗: %v", err))
		return
	}

	response.OK(c, item)
}

// --- Gateway ---

// ListGateways 列出 Gateway（支援 namespace 過濾）
// GET /api/v1/clusters/:clusterID/gateways?namespace=xxx
func (h *GatewayHandler) ListGateways(c *gin.Context) {
	logger.Info("列出 Gateway")
	namespace := c.Query("namespace")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := svc.ListGateways(ctx, namespace)
	if err != nil {
		logger.Error("列出 Gateway 失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("列出 Gateway 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetGateway 取得單一 Gateway
// GET /api/v1/clusters/:clusterID/gateways/:namespace/:name
func (h *GatewayHandler) GetGateway(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.GetGateway(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 Gateway 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 Gateway 失敗: %v", err))
		return
	}

	response.OK(c, item)
}

// GetGatewayYAML 取得 Gateway YAML
// GET /api/v1/clusters/:clusterID/gateways/:namespace/:name/yaml
func (h *GatewayHandler) GetGatewayYAML(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	yamlStr, err := svc.GetGatewayYAML(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 Gateway YAML 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 Gateway YAML 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": yamlStr})
}

// --- HTTPRoute ---

// ListHTTPRoutes 列出 HTTPRoute（支援 namespace 過濾）
// GET /api/v1/clusters/:clusterID/httproutes?namespace=xxx
func (h *GatewayHandler) ListHTTPRoutes(c *gin.Context) {
	logger.Info("列出 HTTPRoute")
	namespace := c.Query("namespace")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := svc.ListHTTPRoutes(ctx, namespace)
	if err != nil {
		logger.Error("列出 HTTPRoute 失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("列出 HTTPRoute 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetHTTPRoute 取得單一 HTTPRoute
// GET /api/v1/clusters/:clusterID/httproutes/:namespace/:name
func (h *GatewayHandler) GetHTTPRoute(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.GetHTTPRoute(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 HTTPRoute 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 HTTPRoute 失敗: %v", err))
		return
	}

	response.OK(c, item)
}

// GetHTTPRouteYAML 取得 HTTPRoute YAML
// GET /api/v1/clusters/:clusterID/httproutes/:namespace/:name/yaml
func (h *GatewayHandler) GetHTTPRouteYAML(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	yamlStr, err := svc.GetHTTPRouteYAML(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 HTTPRoute YAML 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 HTTPRoute YAML 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": yamlStr})
}
