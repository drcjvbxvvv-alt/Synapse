package handlers

import (
	"fmt"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/response"
)

// ─── ResourceQuota CRUD ───────────────────────────────────────────────────────

// ListResourceQuotas 列出命名空間下的 ResourceQuota
func (h *NamespaceHandler) ListResourceQuotas(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
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
	list, err := k8sClient.GetClientset().CoreV1().ResourceQuotas(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 ResourceQuota 失敗: "+err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, q := range list.Items {
		items = append(items, map[string]interface{}{
			"name":      q.Name,
			"namespace": q.Namespace,
			"hard":      convertResourceList(q.Status.Hard),
			"used":      convertResourceList(q.Status.Used),
			"createdAt": q.CreationTimestamp.Time,
		})
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// CreateResourceQuota 建立 ResourceQuota
func (h *NamespaceHandler) CreateResourceQuota(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	var req struct {
		Name string            `json:"name" binding:"required"`
		Hard map[string]string `json:"hard" binding:"required"`
	}
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
	hard := corev1.ResourceList{}
	for k, v := range req.Hard {
		qty, err := resource.ParseQuantity(v)
		if err != nil {
			response.BadRequest(c, fmt.Sprintf("無效的資源數值 %s=%s: %v", k, v, err))
			return
		}
		hard[corev1.ResourceName(k)] = qty
	}
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: namespace},
		Spec:       corev1.ResourceQuotaSpec{Hard: hard},
	}
	created, err := k8sClient.GetClientset().CoreV1().ResourceQuotas(namespace).Create(c.Request.Context(), quota, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "建立 ResourceQuota 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": created.Name, "namespace": created.Namespace})
}

// UpdateResourceQuota 更新 ResourceQuota
func (h *NamespaceHandler) UpdateResourceQuota(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")
	var req struct {
		Hard map[string]string `json:"hard" binding:"required"`
	}
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
	existing, err := k8sClient.GetClientset().CoreV1().ResourceQuotas(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "ResourceQuota 不存在")
		return
	}
	hard := corev1.ResourceList{}
	for k, v := range req.Hard {
		qty, err := resource.ParseQuantity(v)
		if err != nil {
			response.BadRequest(c, fmt.Sprintf("無效的資源數值 %s=%s: %v", k, v, err))
			return
		}
		hard[corev1.ResourceName(k)] = qty
	}
	existing.Spec.Hard = hard
	updated, err := k8sClient.GetClientset().CoreV1().ResourceQuotas(namespace).Update(c.Request.Context(), existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "更新 ResourceQuota 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": updated.Name, "hard": convertResourceList(updated.Spec.Hard)})
}

// DeleteResourceQuota 刪除 ResourceQuota
func (h *NamespaceHandler) DeleteResourceQuota(c *gin.Context) {
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
	if err := k8sClient.GetClientset().CoreV1().ResourceQuotas(namespace).Delete(c.Request.Context(), name, metav1.DeleteOptions{}); err != nil {
		response.InternalError(c, "刪除 ResourceQuota 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "ResourceQuota 刪除成功"})
}
