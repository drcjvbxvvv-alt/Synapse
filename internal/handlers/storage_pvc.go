package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// PVCInfo PVC資訊
type PVCInfo struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Status           string            `json:"status"`
	VolumeName       string            `json:"volumeName"`
	StorageClassName string            `json:"storageClassName"`
	AccessModes      []string          `json:"accessModes"`
	Capacity         string            `json:"capacity"`
	VolumeMode       string            `json:"volumeMode"`
	CreatedAt        time.Time         `json:"createdAt"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
}

// ListPVCs 獲取PVC列表
func (h *StorageHandler) ListPVCs(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
	namespace := c.DefaultQuery("namespace", "")
	status := c.DefaultQuery("status", "")
	search := c.DefaultQuery("search", "")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取PVCs
	pvcs, err := h.getPVCs(ctx, clientset, namespace)
	if err != nil {
		logger.Error("獲取PVCs失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取PVCs失敗: %v", err))
		return
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		pvcs = middleware.FilterResourcesByNamespace(c, pvcs, func(pvc PVCInfo) string {
			return pvc.Namespace
		})
	}

	// 過濾和搜尋
	filteredPVCs := h.filterPVCs(pvcs, status, search)

	// 排序
	sort.Slice(filteredPVCs, func(i, j int) bool {
		return filteredPVCs[i].CreatedAt.After(filteredPVCs[j].CreatedAt)
	})

	// 分頁
	total := len(filteredPVCs)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedPVCs := filteredPVCs[start:end]

	response.PagedList(c, pagedPVCs, int64(total), page, pageSize)
}

// GetPVC 獲取單個PVC詳情
func (h *StorageHandler) GetPVC(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取PVC
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取PVC失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取PVC失敗: %v", err))
		return
	}

	pvcInfo := h.convertToPVCInfo(pvc)

	response.OK(c, pvcInfo)
}

// GetPVCYAML 獲取PVC的YAML
func (h *StorageHandler) GetPVCYAML(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取PVC
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取PVC失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取PVC失敗: %v", err))
		return
	}

	// 轉換為YAML
	yamlData, err := yaml.Marshal(pvc)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeletePVC 刪除PVC
func (h *StorageHandler) DeletePVC(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 刪除PVC
	err = clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除PVC失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除PVC失敗: %v", err))
		return
	}

	logger.Info("PVC刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// GetPVCNamespaces 獲取PVC所在的命名空間列表
func (h *StorageHandler) GetPVCNamespaces(c *gin.Context) {
	clusterID := c.Param("clusterID")

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}
	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取所有PVCs
	pvcList, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取PVC列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取PVC列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的PVC數量
	nsMap := make(map[string]int)
	for _, pvc := range pvcList.Items {
		nsMap[pvc.Namespace]++
	}

	type NamespaceItem struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceItem
	for ns, count := range nsMap {
		namespaces = append(namespaces, NamespaceItem{
			Name:  ns,
			Count: count,
		})
	}

	// 按名稱排序
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	response.OK(c, namespaces)
}

// getPVCs 獲取PVCs
func (h *StorageHandler) getPVCs(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]PVCInfo, error) {
	var pvcList *corev1.PersistentVolumeClaimList
	var err error

	if namespace == "" || namespace == "_all_" {
		pvcList, err = clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	} else {
		pvcList, err = clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	pvcs := make([]PVCInfo, 0, len(pvcList.Items))
	for _, pvc := range pvcList.Items {
		pvcs = append(pvcs, h.convertToPVCInfo(&pvc))
	}

	return pvcs, nil
}

// convertToPVCInfo 轉換為PVCInfo
func (h *StorageHandler) convertToPVCInfo(pvc *corev1.PersistentVolumeClaim) PVCInfo {
	accessModes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, mode := range pvc.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	capacity := ""
	if pvc.Status.Capacity != nil {
		if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = storage.String()
		}
	}

	storageClassName := ""
	if pvc.Spec.StorageClassName != nil {
		storageClassName = *pvc.Spec.StorageClassName
	}

	volumeMode := ""
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	}

	return PVCInfo{
		Name:             pvc.Name,
		Namespace:        pvc.Namespace,
		Status:           string(pvc.Status.Phase),
		VolumeName:       pvc.Spec.VolumeName,
		StorageClassName: storageClassName,
		AccessModes:      accessModes,
		Capacity:         capacity,
		VolumeMode:       volumeMode,
		CreatedAt:        pvc.CreationTimestamp.Time,
		Labels:           pvc.Labels,
		Annotations:      pvc.Annotations,
	}
}

// filterPVCs 過濾PVCs
func (h *StorageHandler) filterPVCs(pvcs []PVCInfo, status, search string) []PVCInfo {
	filtered := make([]PVCInfo, 0)
	for _, pvc := range pvcs {
		// 狀態過濾
		if status != "" && pvc.Status != status {
			continue
		}

		// 搜尋過濾
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(pvc.Name), searchLower) &&
				!strings.Contains(strings.ToLower(pvc.Namespace), searchLower) &&
				!strings.Contains(strings.ToLower(pvc.StorageClassName), searchLower) {
				continue
			}
		}

		filtered = append(filtered, pvc)
	}
	return filtered
}
