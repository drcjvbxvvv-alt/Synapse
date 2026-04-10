package handlers

import (
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
)

// CreateNamespace 建立命名空間
func (h *NamespaceHandler) CreateNamespace(c *gin.Context) {
	// 檢查是否有管理員權限（只有管理員才能建立命名空間）
	permission := middleware.GetClusterPermission(c)
	if permission == nil || permission.PermissionType != "admin" {
		response.Forbidden(c, "只有管理員才能建立命名空間")
		return
	}

	clusterIDStr := c.Param("clusterID")
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

	var req struct {
		Name        string            `json:"name" binding:"required"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()

	// 構建命名空間物件
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
	}

	// 建立命名空間
	createdNs, err := clientset.CoreV1().Namespaces().Create(c.Request.Context(), namespace, metav1.CreateOptions{})
	if err != nil {
		response.InternalError(c, "建立命名空間失敗: "+err.Error())
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

// DeleteNamespace 刪除命名空間
func (h *NamespaceHandler) DeleteNamespace(c *gin.Context) {
	// 檢查是否有管理員權限（只有管理員才能刪除命名空間）
	permission := middleware.GetClusterPermission(c)
	if permission == nil || permission.PermissionType != "admin" {
		response.Forbidden(c, "只有管理員才能刪除命名空間")
		return
	}

	clusterIDStr := c.Param("clusterID")
	namespaceName := c.Param("namespace")

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

	// 刪除命名空間
	err = clientset.CoreV1().Namespaces().Delete(c.Request.Context(), namespaceName, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除命名空間失敗: "+err.Error())
		return
	}

	response.NoContent(c)
}
