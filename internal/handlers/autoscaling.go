package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"gorm.io/gorm"
)

// AutoscalingHandler 彈性伸縮深化（KEDA / Karpenter / CAS）
type AutoscalingHandler struct {
	db             *gorm.DB
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewAutoscalingHandler(db *gorm.DB, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *AutoscalingHandler {
	return &AutoscalingHandler{db: db, clusterService: clusterService, k8sMgr: k8sMgr}
}

// ─── KEDA GVRs ─────────────────────────────────────────────────────────────
var (
	kedaScaledObjectGVR = schema.GroupVersionResource{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"}
	kedaScaledJobGVR    = schema.GroupVersionResource{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledjobs"}
)

// ─── Karpenter GVRs ────────────────────────────────────────────────────────
var (
	karpenterNodePoolGVR  = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"}
	karpenterNodeClaimGVR = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}
)

// getClient 共用輔助：解析 clusterID 並取得 dynamic client
func (h *AutoscalingHandler) getClient(c *gin.Context) (dynamic.Interface, bool) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return nil, false
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return nil, false
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return nil, false
	}
	dyn, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "建立 dynamic client 失敗: "+err.Error())
		return nil, false
	}
	return dyn, true
}

// isCRDPresent 檢查特定 CRD 是否已安裝（透過嘗試列出一個空結果來偵測）
func isCRDPresent(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource) bool {
	_, err := dyn.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// ═══════════════════════════════════════════════════════════════════════════
// KEDA
// ═══════════════════════════════════════════════════════════════════════════

// CheckKEDA 偵測 KEDA 是否安裝
// GET /clusters/:clusterID/keda/status
func (h *AutoscalingHandler) CheckKEDA(c *gin.Context) {
	dyn, ok := h.getClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	installed := isCRDPresent(ctx, dyn, kedaScaledObjectGVR)
	response.OK(c, map[string]interface{}{"installed": installed})
}

// ListScaledObjects 列出 KEDA ScaledObjects
// GET /clusters/:clusterID/keda/scaled-objects?namespace=
func (h *AutoscalingHandler) ListScaledObjects(c *gin.Context) {
	dyn, ok := h.getClient(c)
	if !ok {
		return
	}
	namespace := c.DefaultQuery("namespace", "")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := dyn.Resource(kedaScaledObjectGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 ScaledObject 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		spec, _ := obj.Object["spec"].(map[string]interface{})
		status, _ := obj.Object["status"].(map[string]interface{})

		// 解析 scaleTargetRef
		scaleTargetRef, _ := spec["scaleTargetRef"].(map[string]interface{})
		targetName, _ := scaleTargetRef["name"].(string)
		targetKind, _ := scaleTargetRef["kind"].(string)

		// 解析 triggers
		triggers, _ := spec["triggers"].([]interface{})
		triggerInfos := make([]map[string]interface{}, 0, len(triggers))
		for _, t := range triggers {
			tMap, _ := t.(map[string]interface{})
			triggerInfos = append(triggerInfos, map[string]interface{}{
				"type":     tMap["type"],
				"metadata": tMap["metadata"],
			})
		}

		minReplicas, _ := spec["minReplicaCount"].(int64)
		maxReplicas, _ := spec["maxReplicaCount"].(int64)
		currentReplicas, _ := status["currentReplicas"].(int64)
		desiredReplicas, _ := status["desiredReplicas"].(int64)

		items = append(items, map[string]interface{}{
			"name":            obj.GetName(),
			"namespace":       obj.GetNamespace(),
			"targetName":      targetName,
			"targetKind":      targetKind,
			"minReplicas":     minReplicas,
			"maxReplicas":     maxReplicas,
			"currentReplicas": currentReplicas,
			"desiredReplicas": desiredReplicas,
			"triggers":        triggerInfos,
			"createdAt":       obj.GetCreationTimestamp().Time,
		})
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// ListScaledJobs 列出 KEDA ScaledJobs
// GET /clusters/:clusterID/keda/scaled-jobs?namespace=
func (h *AutoscalingHandler) ListScaledJobs(c *gin.Context) {
	dyn, ok := h.getClient(c)
	if !ok {
		return
	}
	namespace := c.DefaultQuery("namespace", "")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := dyn.Resource(kedaScaledJobGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 ScaledJob 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		spec, _ := obj.Object["spec"].(map[string]interface{})
		status, _ := obj.Object["status"].(map[string]interface{})
		triggers, _ := spec["triggers"].([]interface{})
		triggerInfos := make([]map[string]interface{}, 0, len(triggers))
		for _, t := range triggers {
			tMap, _ := t.(map[string]interface{})
			triggerInfos = append(triggerInfos, map[string]interface{}{
				"type":     tMap["type"],
				"metadata": tMap["metadata"],
			})
		}
		ready, _ := status["ready"].(bool)
		items = append(items, map[string]interface{}{
			"name":      obj.GetName(),
			"namespace": obj.GetNamespace(),
			"triggers":  triggerInfos,
			"ready":     ready,
			"createdAt": obj.GetCreationTimestamp().Time,
		})
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// ═══════════════════════════════════════════════════════════════════════════
// Karpenter
// ═══════════════════════════════════════════════════════════════════════════

// CheckKarpenter 偵測 Karpenter 是否安裝
// GET /clusters/:clusterID/karpenter/status
func (h *AutoscalingHandler) CheckKarpenter(c *gin.Context) {
	dyn, ok := h.getClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	installed := isCRDPresent(ctx, dyn, karpenterNodePoolGVR)
	response.OK(c, map[string]interface{}{"installed": installed})
}

// ListNodePools 列出 Karpenter NodePools
// GET /clusters/:clusterID/karpenter/node-pools
func (h *AutoscalingHandler) ListNodePools(c *gin.Context) {
	dyn, ok := h.getClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := dyn.Resource(karpenterNodePoolGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 NodePool 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		spec, _ := obj.Object["spec"].(map[string]interface{})
		status, _ := obj.Object["status"].(map[string]interface{})

		// limits (CPU/memory 上限)
		limits, _ := spec["limits"].(map[string]interface{})

		// disruption / weight
		disruption, _ := spec["disruption"].(map[string]interface{})
		consolidationPolicy, _ := disruption["consolidationPolicy"].(string)

		// status counters
		resources, _ := status["resources"].(map[string]interface{})

		items = append(items, map[string]interface{}{
			"name":                obj.GetName(),
			"limits":              limits,
			"consolidationPolicy": consolidationPolicy,
			"resources":           resources,
			"createdAt":           obj.GetCreationTimestamp().Time,
		})
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// ListNodeClaims 列出 Karpenter NodeClaims
// GET /clusters/:clusterID/karpenter/node-claims
func (h *AutoscalingHandler) ListNodeClaims(c *gin.Context) {
	dyn, ok := h.getClient(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	list, err := dyn.Resource(karpenterNodeClaimGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 NodeClaim 失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, obj := range list.Items {
		spec, _ := obj.Object["spec"].(map[string]interface{})
		status, _ := obj.Object["status"].(map[string]interface{})
		nodePoolName := obj.GetLabels()["karpenter.sh/nodepool"]
		nodeName, _ := status["nodeName"].(string)
		conditions, _ := status["conditions"].([]interface{})
		instanceType, _ := spec["requirements"].(interface{}) // simplified

		items = append(items, map[string]interface{}{
			"name":         obj.GetName(),
			"nodePool":     nodePoolName,
			"nodeName":     nodeName,
			"instanceType": instanceType,
			"conditions":   conditions,
			"createdAt":    obj.GetCreationTimestamp().Time,
		})
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// ═══════════════════════════════════════════════════════════════════════════
// Cluster Autoscaler (CAS)
// ═══════════════════════════════════════════════════════════════════════════

// GetCASStatus 偵測 Cluster Autoscaler 並回傳狀態
// GET /clusters/:clusterID/cas/status
func (h *AutoscalingHandler) GetCASStatus(c *gin.Context) {
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
	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// 1. 偵測 CAS Deployment
	_, casDeplErr := clientset.AppsV1().Deployments("kube-system").Get(ctx, "cluster-autoscaler", metav1.GetOptions{})
	installed := casDeplErr == nil

	// 2. 讀取狀態 ConfigMap
	statusCM, cmErr := clientset.CoreV1().ConfigMaps("kube-system").Get(ctx, "cluster-autoscaler-status", metav1.GetOptions{})
	var statusText string
	if cmErr == nil {
		statusText = statusCM.Data["status"]
	}

	// 3. 列出具有 cluster-autoscaler.kubernetes.io/safe-to-evict 的節點群組
	nodeList, _ := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "k8s.io/cluster-autoscaler/enabled=true",
	})
	nodeGroupCount := 0
	if nodeList != nil {
		nodeGroupCount = len(nodeList.Items)
	}

	response.OK(c, map[string]interface{}{
		"installed":      installed,
		"status":         statusText,
		"nodeGroupCount": nodeGroupCount,
	})
}
