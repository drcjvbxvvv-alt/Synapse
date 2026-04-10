package handlers

import (
	"fmt"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/response"
)

// ─── LimitRange CRUD ──────────────────────────────────────────────────────────

// ListLimitRanges 列出命名空間下的 LimitRange
func (h *NamespaceHandler) ListLimitRanges(c *gin.Context) {
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
	list, err := k8sClient.GetClientset().CoreV1().LimitRanges(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 LimitRange 失敗: "+err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, lr := range list.Items {
		limitsInfo := make([]map[string]interface{}, 0, len(lr.Spec.Limits))
		for _, l := range lr.Spec.Limits {
			limitsInfo = append(limitsInfo, map[string]interface{}{
				"type":           string(l.Type),
				"max":            convertResourceList(l.Max),
				"min":            convertResourceList(l.Min),
				"default":        convertResourceList(l.Default),
				"defaultRequest": convertResourceList(l.DefaultRequest),
			})
		}
		items = append(items, map[string]interface{}{
			"name":      lr.Name,
			"namespace": lr.Namespace,
			"limits":    limitsInfo,
			"createdAt": lr.CreationTimestamp.Time,
		})
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// LimitRangeItemReq 單條 LimitRange 限制請求
type LimitRangeItemReq struct {
	Type           string            `json:"type"` // Container / Pod / PersistentVolumeClaim
	Max            map[string]string `json:"max"`
	Min            map[string]string `json:"min"`
	Default        map[string]string `json:"default"`
	DefaultRequest map[string]string `json:"defaultRequest"`
}

func parseLimitRangeItems(reqs []LimitRangeItemReq) ([]corev1.LimitRangeItem, error) {
	parseRL := func(m map[string]string) (corev1.ResourceList, error) {
		rl := corev1.ResourceList{}
		for k, v := range m {
			qty, err := resource.ParseQuantity(v)
			if err != nil {
				return nil, fmt.Errorf("無效數值 %s=%s: %w", k, v, err)
			}
			rl[corev1.ResourceName(k)] = qty
		}
		return rl, nil
	}
	items := make([]corev1.LimitRangeItem, 0, len(reqs))
	for _, r := range reqs {
		item := corev1.LimitRangeItem{Type: corev1.LimitType(r.Type)}
		var err error
		if item.Max, err = parseRL(r.Max); err != nil {
			return nil, err
		}
		if item.Min, err = parseRL(r.Min); err != nil {
			return nil, err
		}
		if item.Default, err = parseRL(r.Default); err != nil {
			return nil, err
		}
		if item.DefaultRequest, err = parseRL(r.DefaultRequest); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// CreateLimitRange 建立 LimitRange
func (h *NamespaceHandler) CreateLimitRange(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	var req struct {
		Name   string              `json:"name" binding:"required"`
		Limits []LimitRangeItemReq `json:"limits" binding:"required"`
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
	lrItems, err := parseLimitRangeItems(req.Limits)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: namespace},
		Spec:       corev1.LimitRangeSpec{Limits: lrItems},
	}
	created, err := k8sClient.GetClientset().CoreV1().LimitRanges(namespace).Create(c.Request.Context(), lr, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "建立 LimitRange 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": created.Name, "namespace": created.Namespace})
}

// UpdateLimitRange 更新 LimitRange
func (h *NamespaceHandler) UpdateLimitRange(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")
	var req struct {
		Limits []LimitRangeItemReq `json:"limits" binding:"required"`
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
	existing, err := k8sClient.GetClientset().CoreV1().LimitRanges(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "LimitRange 不存在")
		return
	}
	lrItems, err := parseLimitRangeItems(req.Limits)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	existing.Spec.Limits = lrItems
	updated, err := k8sClient.GetClientset().CoreV1().LimitRanges(namespace).Update(c.Request.Context(), existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "更新 LimitRange 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": updated.Name})
}

// DeleteLimitRange 刪除 LimitRange
func (h *NamespaceHandler) DeleteLimitRange(c *gin.Context) {
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
	if err := k8sClient.GetClientset().CoreV1().LimitRanges(namespace).Delete(c.Request.Context(), name, metav1.DeleteOptions{}); err != nil {
		response.InternalError(c, "刪除 LimitRange 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "LimitRange 刪除成功"})
}
