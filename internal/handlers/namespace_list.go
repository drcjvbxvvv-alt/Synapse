package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
)

// GetNamespaces 獲取叢集命名空間列表
func (h *NamespaceHandler) GetNamespaces(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 獲取命名空間列表
	namespaces, err := h.k8sMgr.NamespacesLister(clusterID).List(labels.Everything())
	if err != nil {
		response.InternalError(c, "讀取命名空間快取失敗: "+err.Error())
		return
	}

	// 獲取使用者的命名空間權限
	allowedNs, hasAllAccess := middleware.GetAllowedNamespaces(c)

	// 構建響應資料
	var namespaceList []NamespaceResponse
	for _, ns := range namespaces {
		// 非全部權限使用者，只返回有權限的命名空間
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

	// 返回結果，同時告知前端使用者是否有全部權限
	response.OK(c, gin.H{
		"items": namespaceList,
		"meta": gin.H{
			"hasAllAccess":      hasAllAccess,
			"allowedNamespaces": allowedNs,
		},
	})
}

// GetNamespaceDetail 獲取命名空間詳情
func (h *NamespaceHandler) GetNamespaceDetail(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespaceName := c.Param("namespace")

	// 檢查命名空間訪問權限
	if !middleware.HasNamespaceAccess(c, namespaceName) {
		response.Forbidden(c, "無權訪問命名空間: "+namespaceName)
		return
	}

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在: "+err.Error())
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()

	// 獲取命名空間詳情
	namespace, err := clientset.CoreV1().Namespaces().Get(c.Request.Context(), namespaceName, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "命名空間不存在: "+err.Error())
		return
	}

	// 獲取資源配額
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
