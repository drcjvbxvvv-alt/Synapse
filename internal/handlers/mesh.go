package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
)

// MeshHandler Service Mesh（Istio）處理器
type MeshHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	meshSvc        services.MeshQuerier
}

// NewMeshHandler 建立 MeshHandler
func NewMeshHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager, meshSvc services.MeshQuerier) *MeshHandler {
	return &MeshHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
		meshSvc:        meshSvc,
	}
}

// getClients 取得 clientset 和 dynamic client
func (h *MeshHandler) getClients(c *gin.Context) (dynamic.Interface, uint, bool) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return nil, 0, false
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("取得叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return nil, 0, false
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("取得 K8s 客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 K8s 客戶端失敗: %v", err))
		return nil, 0, false
	}

	dynClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "建立 dynamic client 失敗: "+err.Error())
		return nil, 0, false
	}

	return dynClient, clusterID, true
}

// GetStatus 取得 Istio 安裝狀態
func (h *MeshHandler) GetStatus(c *gin.Context) {
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
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	status := h.meshSvc.GetStatus(ctx, k8sClient.GetClientset())
	response.OK(c, status)
}

// GetTopology 取得服務網格拓撲
func (h *MeshHandler) GetTopology(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	namespace := c.Query("namespace")

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	topology, err := h.meshSvc.GetTopology(ctx, k8sClient.GetClientset(), clusterID, namespace)
	if err != nil {
		response.InternalError(c, "取得拓撲失敗: "+err.Error())
		return
	}

	response.OK(c, topology)
}

// ListVirtualServices 列出 VirtualServices
func (h *MeshHandler) ListVirtualServices(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Query("namespace")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := h.meshSvc.ListVirtualServices(ctx, dynClient, namespace)
	if err != nil {
		if isIstioNotInstalled(err) {
			response.OK(c, gin.H{"items": []interface{}{}, "installed": false})
			return
		}
		response.InternalError(c, "列出 VirtualService 失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"items": list, "total": len(list), "installed": true})
}

// GetVirtualService 取得單一 VirtualService
func (h *MeshHandler) GetVirtualService(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	obj, err := h.meshSvc.GetVirtualService(ctx, dynClient, namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "VirtualService 不存在")
			return
		}
		response.InternalError(c, "取得 VirtualService 失敗: "+err.Error())
		return
	}

	response.OK(c, obj.Object)
}

// CreateVirtualService 建立 VirtualService
func (h *MeshHandler) CreateVirtualService(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "讀取請求 Body 失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	created, err := h.meshSvc.CreateVirtualService(ctx, dynClient, namespace, json.RawMessage(body))
	if err != nil {
		if errors.IsAlreadyExists(err) {
			response.BadRequest(c, "VirtualService 已存在")
			return
		}
		response.InternalError(c, "建立 VirtualService 失敗: "+err.Error())
		return
	}

	logger.Info("建立 VirtualService", "cluster", c.Param("clusterID"), "namespace", namespace, "name", created.GetName())
	response.OK(c, created.Object)
}

// UpdateVirtualService 更新 VirtualService
func (h *MeshHandler) UpdateVirtualService(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "讀取請求 Body 失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	updated, err := h.meshSvc.UpdateVirtualService(ctx, dynClient, namespace, name, json.RawMessage(body))
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "VirtualService 不存在")
			return
		}
		response.InternalError(c, "更新 VirtualService 失敗: "+err.Error())
		return
	}

	logger.Info("更新 VirtualService", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, updated.Object)
}

// DeleteVirtualService 刪除 VirtualService
func (h *MeshHandler) DeleteVirtualService(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	if err := h.meshSvc.DeleteVirtualService(ctx, dynClient, namespace, name); err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "VirtualService 不存在")
			return
		}
		response.InternalError(c, "刪除 VirtualService 失敗: "+err.Error())
		return
	}

	logger.Info("刪除 VirtualService", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "VirtualService 刪除成功"})
}

// ListDestinationRules 列出 DestinationRules
func (h *MeshHandler) ListDestinationRules(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Query("namespace")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := h.meshSvc.ListDestinationRules(ctx, dynClient, namespace)
	if err != nil {
		if isIstioNotInstalled(err) {
			response.OK(c, gin.H{"items": []interface{}{}, "installed": false})
			return
		}
		response.InternalError(c, "列出 DestinationRule 失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"items": list, "total": len(list), "installed": true})
}

// GetDestinationRule 取得單一 DestinationRule
func (h *MeshHandler) GetDestinationRule(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	obj, err := h.meshSvc.GetDestinationRule(ctx, dynClient, namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "DestinationRule 不存在")
			return
		}
		response.InternalError(c, "取得 DestinationRule 失敗: "+err.Error())
		return
	}

	response.OK(c, obj.Object)
}

// CreateDestinationRule 建立 DestinationRule
func (h *MeshHandler) CreateDestinationRule(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "讀取請求 Body 失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	created, err := h.meshSvc.CreateDestinationRule(ctx, dynClient, namespace, json.RawMessage(body))
	if err != nil {
		if errors.IsAlreadyExists(err) {
			response.BadRequest(c, "DestinationRule 已存在")
			return
		}
		response.InternalError(c, "建立 DestinationRule 失敗: "+err.Error())
		return
	}

	logger.Info("建立 DestinationRule", "cluster", c.Param("clusterID"), "namespace", namespace, "name", created.GetName())
	response.OK(c, created.Object)
}

// UpdateDestinationRule 更新 DestinationRule
func (h *MeshHandler) UpdateDestinationRule(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "讀取請求 Body 失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	updated, err := h.meshSvc.UpdateDestinationRule(ctx, dynClient, namespace, name, json.RawMessage(body))
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "DestinationRule 不存在")
			return
		}
		response.InternalError(c, "更新 DestinationRule 失敗: "+err.Error())
		return
	}

	logger.Info("更新 DestinationRule", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, updated.Object)
}

// DeleteDestinationRule 刪除 DestinationRule
func (h *MeshHandler) DeleteDestinationRule(c *gin.Context) {
	dynClient, _, ok := h.getClients(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	if err := h.meshSvc.DeleteDestinationRule(ctx, dynClient, namespace, name); err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "DestinationRule 不存在")
			return
		}
		response.InternalError(c, "刪除 DestinationRule 失敗: "+err.Error())
		return
	}

	logger.Info("刪除 DestinationRule", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "DestinationRule 刪除成功"})
}

// isIstioNotInstalled 判斷錯誤是否為 Istio CRD 未安裝
func isIstioNotInstalled(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no kind is registered") ||
		strings.Contains(msg, "resource not found") ||
		strings.Contains(msg, "the server could not find the requested resource") ||
		errors.IsNotFound(err)
}
