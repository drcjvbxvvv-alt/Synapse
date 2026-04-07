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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"gorm.io/gorm"
)

var vpaGVR = schema.GroupVersionResource{
	Group:    "autoscaling.k8s.io",
	Version:  "v1",
	Resource: "verticalpodautoscalers",
}

// VPAHandler VPA 處理器（透過 dynamic client 存取 CRD）
type VPAHandler struct {
	db             *gorm.DB
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewVPAHandler(db *gorm.DB, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *VPAHandler {
	return &VPAHandler{db: db, clusterService: clusterService, k8sMgr: k8sMgr}
}

// VPARequest 建立/更新 VPA 請求
type VPARequest struct {
	Name            string `json:"name" binding:"required"`
	Namespace       string `json:"namespace" binding:"required"`
	TargetKind      string `json:"targetKind" binding:"required"` // Deployment / StatefulSet / DaemonSet
	TargetName      string `json:"targetName" binding:"required"`
	UpdateMode      string `json:"updateMode"` // Off / Initial / Recreate / Auto，預設 Auto
	MinCPU          string `json:"minCPU"`     // e.g. "100m"
	MaxCPU          string `json:"maxCPU"`     // e.g. "1"
	MinMemory       string `json:"minMemory"`  // e.g. "50Mi"
	MaxMemory       string `json:"maxMemory"`  // e.g. "500Mi"
}

func getDynamicVPAClient(k8sClient *services.K8sClient) (dynamic.NamespaceableResourceInterface, error) {
	dynClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		return nil, fmt.Errorf("建立 dynamic client 失敗: %w", err)
	}
	return dynClient.Resource(vpaGVR), nil
}

func vpaToInfo(obj *unstructured.Unstructured) map[string]interface{} {
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	status, _, _ := unstructured.NestedMap(obj.Object, "status")

	targetRef, _ := spec["targetRef"].(map[string]interface{})
	updatePolicy, _ := spec["updatePolicy"].(map[string]interface{})
	updateMode := ""
	if updatePolicy != nil {
		updateMode, _ = updatePolicy["updateMode"].(string)
	}

	var recommendations []map[string]interface{}
	if statusMap, ok := status["recommendation"].(map[string]interface{}); ok {
		if containers, ok := statusMap["containerRecommendations"].([]interface{}); ok {
			for _, cr := range containers {
				if m, ok := cr.(map[string]interface{}); ok {
					recommendations = append(recommendations, m)
				}
			}
		}
	}

	return map[string]interface{}{
		"name":            obj.GetName(),
		"namespace":       obj.GetNamespace(),
		"targetKind":      targetRef["kind"],
		"targetName":      targetRef["name"],
		"updateMode":      updateMode,
		"recommendations": recommendations,
		"createdAt":       obj.GetCreationTimestamp().Time,
	}
}

func buildVPAObject(req *VPARequest) *unstructured.Unstructured {
	if req.UpdateMode == "" {
		req.UpdateMode = "Auto"
	}

	resourcePolicy := map[string]interface{}{}
	if req.MinCPU != "" || req.MaxCPU != "" || req.MinMemory != "" || req.MaxMemory != "" {
		minAllowed := map[string]interface{}{}
		maxAllowed := map[string]interface{}{}
		if req.MinCPU != "" {
			minAllowed["cpu"] = req.MinCPU
		}
		if req.MaxCPU != "" {
			maxAllowed["cpu"] = req.MaxCPU
		}
		if req.MinMemory != "" {
			minAllowed["memory"] = req.MinMemory
		}
		if req.MaxMemory != "" {
			maxAllowed["memory"] = req.MaxMemory
		}
		containerPolicy := map[string]interface{}{
			"containerName": "*",
		}
		if len(minAllowed) > 0 {
			containerPolicy["minAllowed"] = minAllowed
		}
		if len(maxAllowed) > 0 {
			containerPolicy["maxAllowed"] = maxAllowed
		}
		resourcePolicy["containerPolicies"] = []interface{}{containerPolicy}
	}

	spec := map[string]interface{}{
		"targetRef": map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       req.TargetKind,
			"name":       req.TargetName,
		},
		"updatePolicy": map[string]interface{}{
			"updateMode": req.UpdateMode,
		},
	}
	if len(resourcePolicy) > 0 {
		spec["resourcePolicy"] = resourcePolicy
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling.k8s.io/v1",
			"kind":       "VerticalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": req.Namespace,
			},
			"spec": spec,
		},
	}
}

// CheckVPACRD 檢查叢集是否安裝 VPA controller（透過 discovery API）
func (h *VPAHandler) CheckVPACRD(c *gin.Context) {
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

	// 透過 Discovery API 確認 autoscaling.k8s.io/v1 是否存在
	_, err = k8sClient.GetClientset().Discovery().ServerResourcesForGroupVersion("autoscaling.k8s.io/v1")
	installed := err == nil
	response.OK(c, gin.H{"installed": installed})
}

// ListVPA 列出命名空間下的 VPA
func (h *VPAHandler) ListVPA(c *gin.Context) {
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

	vpaClient, err := getDynamicVPAClient(k8sClient)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := vpaClient.Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "no kind is registered") || errors.IsNotFound(err) {
			response.OK(c, gin.H{"items": []interface{}{}, "total": 0, "installed": false})
			return
		}
		response.InternalError(c, "取得 VPA 列表失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, vpaToInfo(&list.Items[i]))
	}
	response.OK(c, gin.H{"items": items, "total": len(items), "installed": true})
}

// CreateVPA 建立 VPA
func (h *VPAHandler) CreateVPA(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var req VPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
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

	vpaClient, err := getDynamicVPAClient(k8sClient)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	obj := buildVPAObject(&req)
	created, err := vpaClient.Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			response.BadRequest(c, fmt.Sprintf("VPA '%s' 已存在", req.Name))
			return
		}
		response.InternalError(c, "建立 VPA 失敗: "+err.Error())
		return
	}

	logger.Info("建立 VPA", "cluster", c.Param("clusterID"), "namespace", req.Namespace, "name", req.Name)
	response.OK(c, vpaToInfo(created))
}

// UpdateVPA 更新 VPA
func (h *VPAHandler) UpdateVPA(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req VPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
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

	vpaClient, err := getDynamicVPAClient(k8sClient)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	existing, err := vpaClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "VPA 不存在")
			return
		}
		response.InternalError(c, "取得 VPA 失敗: "+err.Error())
		return
	}

	// 用新 spec 覆蓋，保留 metadata（含 resourceVersion）
	newObj := buildVPAObject(&req)
	newObj.SetResourceVersion(existing.GetResourceVersion())
	newObj.SetNamespace(namespace)
	newObj.SetName(name)

	updated, err := vpaClient.Namespace(namespace).Update(ctx, newObj, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "更新 VPA 失敗: "+err.Error())
		return
	}

	logger.Info("更新 VPA", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, vpaToInfo(updated))
}

// DeleteVPA 刪除 VPA
func (h *VPAHandler) DeleteVPA(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

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

	vpaClient, err := getDynamicVPAClient(k8sClient)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := vpaClient.Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "VPA 不存在")
			return
		}
		response.InternalError(c, "刪除 VPA 失敗: "+err.Error())
		return
	}

	logger.Info("刪除 VPA", "cluster", c.Param("clusterID"), "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "VPA 刪除成功"})
}

// GetWorkloadVPA 取得指定工作負載關聯的 VPA
func (h *VPAHandler) GetWorkloadVPA(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	workloadName := c.Param("name")
	workloadKind := c.Query("kind") // Deployment / StatefulSet / DaemonSet

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

	vpaClient, err := getDynamicVPAClient(k8sClient)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := vpaClient.Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.OK(c, gin.H{"vpa": nil, "installed": false})
		return
	}

	for i := range list.Items {
		spec, _, _ := unstructured.NestedMap(list.Items[i].Object, "spec")
		targetRef, _ := spec["targetRef"].(map[string]interface{})
		if targetRef == nil {
			continue
		}
		kind, _ := targetRef["kind"].(string)
		tName, _ := targetRef["name"].(string)
		if tName == workloadName && (workloadKind == "" || kind == workloadKind) {
			response.OK(c, gin.H{"vpa": vpaToInfo(&list.Items[i]), "installed": true})
			return
		}
	}
	response.OK(c, gin.H{"vpa": nil, "installed": true})
}

// --- JSON marshalling helper（VPARequest → JSON for logging）---
func vpaReqJSON(req *VPARequest) string {
	b, _ := json.Marshal(req)
	return string(b)
}

var _ = vpaReqJSON // suppress unused warning
