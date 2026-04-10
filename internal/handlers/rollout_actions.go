package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetRolloutHPA 獲取Rollout關聯的HPA
func (h *RolloutHandler) GetRolloutHPA(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的HPA: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	hpaList, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取HPA失敗: "+err.Error())
		return
	}

	// 查詢與Rollout關聯的HPA
	var targetHPA interface{}
	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Rollout" && hpa.Spec.ScaleTargetRef.Name == name {
			targetHPA = hpa
			break
		}
	}

	response.OK(c, targetHPA)
}

// GetRolloutReplicaSets 獲取Rollout關聯的ReplicaSets
func (h *RolloutHandler) GetRolloutReplicaSets(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的ReplicaSets: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	replicaSets, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取ReplicaSets失敗: "+err.Error())
		return
	}

	// 篩選由Rollout管理的ReplicaSets
	var relatedReplicaSets []interface{}
	for _, rs := range replicaSets.Items {
		for _, ownerRef := range rs.OwnerReferences {
			if ownerRef.Kind == "Rollout" && ownerRef.Name == name {
				relatedReplicaSets = append(relatedReplicaSets, rs)
				break
			}
		}
	}

	response.OK(c, relatedReplicaSets)
}

// GetRolloutEvents 獲取Rollout相關的Events
func (h *RolloutHandler) GetRolloutEvents(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout相關的Events: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Rollout", name),
	})
	if err != nil {
		response.InternalError(c, "獲取Events失敗: "+err.Error())
		return
	}

	response.OK(c, events)
}

// PromoteRollout 推進 Rollout 一個步驟（解除 pause）
func (h *RolloutHandler) PromoteRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

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
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	patch := []byte(`{"spec":{"paused":false}}`)
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		response.InternalError(c, "Promote 失敗: "+err.Error())
		return
	}
	logger.Info("Promote Rollout", "cluster", clusterId, "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "Promote 成功"})
}

// PromoteFullRollout 全量推進 Rollout（跳過所有 pause 和 analysis）
func (h *RolloutHandler) PromoteFullRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

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
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 先取得現有 Rollout，設定 status.promoteFull = true 並透過 UpdateStatus 更新
	existing, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "取得 Rollout 失敗: "+err.Error())
		return
	}
	existing.Status.PromoteFull = true
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).UpdateStatus(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "Promote Full 失敗: "+err.Error())
		return
	}
	logger.Info("PromoteFull Rollout", "cluster", clusterId, "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "Promote Full 成功"})
}

// AbortRollout 中止 Rollout
func (h *RolloutHandler) AbortRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

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
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	existing, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "取得 Rollout 失敗: "+err.Error())
		return
	}
	existing.Status.Abort = true
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).UpdateStatus(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "Abort 失敗: "+err.Error())
		return
	}
	logger.Info("Abort Rollout", "cluster", clusterId, "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "Abort 成功"})
}

// GetRolloutAnalysisRuns 取得 Rollout 相關的 AnalysisRun 列表
func (h *RolloutHandler) GetRolloutAnalysisRuns(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

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
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// List AnalysisRuns 以 rollout owner label 過濾
	labelSelector := fmt.Sprintf("rollouts-pod-template-hash,rollout-type")
	_ = labelSelector // 改用 List all + filter by owner ref
	allRuns, err := rolloutClient.ArgoprojV1alpha1().AnalysisRuns(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 AnalysisRun 失敗: "+err.Error())
		return
	}

	// 篩選屬於此 Rollout 的 AnalysisRun（透過 ownerReferences）
	result := make([]map[string]interface{}, 0)
	for _, ar := range allRuns.Items {
		for _, ref := range ar.OwnerReferences {
			if ref.Kind == "Rollout" && ref.Name == name {
				result = append(result, map[string]interface{}{
					"name":      ar.Name,
					"namespace": ar.Namespace,
					"phase":     string(ar.Status.Phase),
					"message":   ar.Status.Message,
					"startedAt": ar.CreationTimestamp.Time,
				})
				break
			}
		}
	}

	response.OK(c, gin.H{"items": result, "total": len(result)})
}
