package handlers

import (
	"context"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"

	"fmt"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type NamespaceHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewNamespaceHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *NamespaceHandler {
	return &NamespaceHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// NamespaceResponse 命名空间响应结构
type NamespaceResponse struct {
	Name              string             `json:"name"`
	Status            string             `json:"status"`
	Labels            map[string]string  `json:"labels"`
	Annotations       map[string]string  `json:"annotations"`
	CreationTimestamp string             `json:"creationTimestamp"`
	ResourceQuota     *ResourceQuotaInfo `json:"resourceQuota,omitempty"`
}

// ResourceQuotaInfo 资源配额信息
type ResourceQuotaInfo struct {
	Hard map[string]string `json:"hard"`
	Used map[string]string `json:"used"`
}

// GetNamespaces 获取集群命名空间列表
func (h *NamespaceHandler) GetNamespaces(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")

	// 获取集群信息
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就绪: "+err.Error())
		return
	}

	// 获取命名空间列表
	namespaces, err := h.k8sMgr.NamespacesLister(clusterID).List(labels.Everything())
	if err != nil {
		response.InternalError(c, "读取命名空间缓存失败: "+err.Error())
		return
	}

	// 获取用户的命名空间权限
	allowedNs, hasAllAccess := middleware.GetAllowedNamespaces(c)

	// 构建响应数据
	var namespaceList []NamespaceResponse
	for _, ns := range namespaces {
		// 非全部权限用户，只返回有权限的命名空间
		if !hasAllAccess && !middleware.HasNamespaceAccess(c, ns.Name) {
			continue
		}

		namespaceResp := NamespaceResponse{
			Name:              ns.Name,
			Status:            string(ns.Status.Phase),
			Labels:            ns.Labels,
			Annotations:       ns.Annotations,
			CreationTimestamp: ns.CreationTimestamp.Format("2006-01-02 15:04:05"),
		}
		namespaceList = append(namespaceList, namespaceResp)
	}

	// 返回结果，同时告知前端用户是否有全部权限
	response.OK(c, gin.H{
		"items": namespaceList,
		"meta": gin.H{
			"hasAllAccess":      hasAllAccess,
			"allowedNamespaces": allowedNs,
		},
	})
}

// GetNamespaceDetail 获取命名空间详情
func (h *NamespaceHandler) GetNamespaceDetail(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespaceName := c.Param("namespace")

	// 检查命名空间访问权限
	if !middleware.HasNamespaceAccess(c, namespaceName) {
		response.Forbidden(c, "无权访问命名空间: "+namespaceName)
		return
	}

	// 获取集群信息
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在: "+err.Error())
		return
	}

	// 获取缓存的 K8s 客户端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "获取K8s客户端失败: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()

	// 获取命名空间详情
	namespace, err := clientset.CoreV1().Namespaces().Get(c.Request.Context(), namespaceName, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "命名空间不存在: "+err.Error())
		return
	}

	// 获取资源配额
	quotas, err := clientset.CoreV1().ResourceQuotas(namespaceName).List(c.Request.Context(), metav1.ListOptions{})
	var resourceQuota *ResourceQuotaInfo
	if err == nil && len(quotas.Items) > 0 {
		quota := quotas.Items[0]
		resourceQuota = &ResourceQuotaInfo{
			Hard: convertResourceList(quota.Status.Hard),
			Used: convertResourceList(quota.Status.Used),
		}
	}

	resourceCount := map[string]int{
		"pods": 0, "services": 0, "configMaps": 0, "secrets": 0,
	}
	if pods, err := clientset.CoreV1().Pods(namespaceName).List(c.Request.Context(), metav1.ListOptions{}); err == nil {
		resourceCount["pods"] = len(pods.Items)
	}
	if svcs, err := clientset.CoreV1().Services(namespaceName).List(c.Request.Context(), metav1.ListOptions{}); err == nil {
		resourceCount["services"] = len(svcs.Items)
	}
	if cms, err := clientset.CoreV1().ConfigMaps(namespaceName).List(c.Request.Context(), metav1.ListOptions{}); err == nil {
		resourceCount["configMaps"] = len(cms.Items)
	}
	if secs, err := clientset.CoreV1().Secrets(namespaceName).List(c.Request.Context(), metav1.ListOptions{}); err == nil {
		resourceCount["secrets"] = len(secs.Items)
	}

	namespaceDetail := map[string]interface{}{
		"name":              namespace.Name,
		"status":            string(namespace.Status.Phase),
		"labels":            namespace.Labels,
		"annotations":       namespace.Annotations,
		"creationTimestamp": namespace.CreationTimestamp.Format("2006-01-02 15:04:05"),
		"resourceQuota":     resourceQuota,
		"resourceCount":     resourceCount,
	}

	response.OK(c, namespaceDetail)
}

// CreateNamespace 创建命名空间
func (h *NamespaceHandler) CreateNamespace(c *gin.Context) {
	// 检查是否有管理员权限（只有管理员才能创建命名空间）
	permission := middleware.GetClusterPermission(c)
	if permission == nil || permission.PermissionType != "admin" {
		response.Forbidden(c, "只有管理员才能创建命名空间")
		return
	}

	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在: "+err.Error())
		return
	}

	var req struct {
		Name        string            `json:"name" binding:"required"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 获取缓存的 K8s 客户端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "获取K8s客户端失败: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()

	// 构建命名空间对象
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
	}

	// 创建命名空间
	createdNs, err := clientset.CoreV1().Namespaces().Create(c.Request.Context(), namespace, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "创建命名空间失败: "+err.Error())
		return
	}

	response.OK(c, NamespaceResponse{
		Name:              createdNs.Name,
		Status:            string(createdNs.Status.Phase),
		Labels:            createdNs.Labels,
		Annotations:       createdNs.Annotations,
		CreationTimestamp: createdNs.CreationTimestamp.Format("2006-01-02 15:04:05"),
	})
}

// DeleteNamespace 删除命名空间
func (h *NamespaceHandler) DeleteNamespace(c *gin.Context) {
	// 检查是否有管理员权限（只有管理员才能删除命名空间）
	permission := middleware.GetClusterPermission(c)
	if permission == nil || permission.PermissionType != "admin" {
		response.Forbidden(c, "只有管理员才能删除命名空间")
		return
	}

	clusterIDStr := c.Param("clusterID")
	namespaceName := c.Param("namespace")

	// 获取集群信息
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "无效的集群ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "集群不存在: "+err.Error())
		return
	}

	// 获取缓存的 K8s 客户端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "获取K8s客户端失败: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()

	// 删除命名空间
	err = clientset.CoreV1().Namespaces().Delete(c.Request.Context(), namespaceName, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "删除命名空间失败: "+err.Error())
		return
	}

	response.NoContent(c)
}

// convertResourceList 转换资源列表为字符串map
func convertResourceList(rl corev1.ResourceList) map[string]string {
	result := make(map[string]string)
	for k, v := range rl {
		result[string(k)] = v.String()
	}
	return result
}

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
	Type           string            `json:"type"`           // Container / Pod / PersistentVolumeClaim
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
