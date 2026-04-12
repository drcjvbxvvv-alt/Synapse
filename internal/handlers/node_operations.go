package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// CordonNode 封鎖節點
func (h *NodeHandler) CordonNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("封鎖節點: %s/%s", clusterId, name)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	// 封鎖節點
	err = k8sClient.CordonNode(name)
	if err != nil {
		response.InternalError(c, "封鎖節點失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// UncordonNode 解封節點
func (h *NodeHandler) UncordonNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("解封節點: %s/%s", clusterId, name)

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	// 解封節點
	err = k8sClient.UncordonNode(name)
	if err != nil {
		response.InternalError(c, "解封節點失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// DrainNode 驅逐節點
func (h *NodeHandler) DrainNode(c *gin.Context) {
	clusterId := c.Param("clusterID")
	name := c.Param("name")
	logger.Info("驅逐節點: %s/%s", clusterId, name)

	// 解析請求參數
	var options map[string]interface{}
	if err := c.ShouldBindJSON(&options); err != nil {
		response.BadRequest(c, "參數解析失敗: "+err.Error())
		return
	}

	// 從叢集服務獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	// 驅逐節點
	err = k8sClient.DrainNode(name, options)
	if err != nil {
		response.InternalError(c, "驅逐節點失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}

// PatchNodeLabels 新增或更新節點標籤
func (h *NodeHandler) PatchNodeLabels(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	name := c.Param("name")

	var req struct {
		Labels map[string]string `json:"labels" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": req.Labels,
		},
	}
	patchBytes, _ := json.Marshal(patch)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err = k8sClient.GetClientset().CoreV1().Nodes().Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		response.InternalError(c, "更新標籤失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": name, "labels": req.Labels})
}

// PatchNodeTaints 替換節點污點列表
// PATCH /clusters/:clusterID/nodes/:name/taints
func (h *NodeHandler) PatchNodeTaints(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	name := c.Param("name")

	type TaintItem struct {
		Key    string `json:"key"    binding:"required"`
		Value  string `json:"value"`
		Effect string `json:"effect" binding:"required"`
	}
	var req struct {
		Taints []TaintItem `json:"taints" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	validEffects := map[string]bool{"NoSchedule": true, "PreferNoSchedule": true, "NoExecute": true}
	for _, t := range req.Taints {
		if !validEffects[t.Effect] {
			response.BadRequest(c, "無效的污點效果: "+t.Effect)
			return
		}
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	taints := make([]map[string]interface{}, 0, len(req.Taints))
	for _, t := range req.Taints {
		taint := map[string]interface{}{"key": t.Key, "effect": t.Effect}
		if t.Value != "" {
			taint["value"] = t.Value
		}
		taints = append(taints, taint)
	}

	patchBytes, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{"taints": taints},
	})

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err = k8sClient.GetClientset().CoreV1().Nodes().Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		response.InternalError(c, "更新污點失敗: "+err.Error())
		return
	}

	logger.Info("更新節點污點", "cluster_id", clusterID, "node", name, "taints", len(req.Taints))
	response.OK(c, gin.H{"name": name, "taints": req.Taints})
}
