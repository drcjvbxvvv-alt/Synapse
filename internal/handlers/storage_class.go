package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// StorageClassInfo StorageClass資訊
type StorageClassInfo struct {
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner"`
	ReclaimPolicy        string            `json:"reclaimPolicy"`
	VolumeBindingMode    string            `json:"volumeBindingMode"`
	AllowVolumeExpansion bool              `json:"allowVolumeExpansion"`
	Parameters           map[string]string `json:"parameters,omitempty"`
	MountOptions         []string          `json:"mountOptions,omitempty"`
	IsDefault            bool              `json:"isDefault"`
	CreatedAt            time.Time         `json:"createdAt"`
	Labels               map[string]string `json:"labels"`
	Annotations          map[string]string `json:"annotations"`
}

// ListStorageClasses 獲取StorageClass列表
func (h *StorageHandler) ListStorageClasses(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取StorageClasses
	scList, err := clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取StorageClasses失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取StorageClasses失敗: %v", err))
		return
	}

	scs := make([]StorageClassInfo, 0, len(scList.Items))
	for _, sc := range scList.Items {
		scs = append(scs, h.convertToStorageClassInfo(&sc))
	}

	// 過濾和搜尋
	filteredSCs := h.filterStorageClasses(scs, search)

	// 排序
	sort.Slice(filteredSCs, func(i, j int) bool {
		return filteredSCs[i].CreatedAt.After(filteredSCs[j].CreatedAt)
	})

	// 分頁
	total := len(filteredSCs)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedSCs := filteredSCs[start:end]

	response.PagedList(c, pagedSCs, int64(total), page, pageSize)
}

// GetStorageClass 獲取單個StorageClass詳情
func (h *StorageHandler) GetStorageClass(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

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

	// 獲取StorageClass
	sc, err := clientset.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取StorageClass失敗", "error", err, "clusterId", clusterID, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取StorageClass失敗: %v", err))
		return
	}

	scInfo := h.convertToStorageClassInfo(sc)

	response.OK(c, scInfo)
}

// GetStorageClassYAML 獲取StorageClass的YAML
func (h *StorageHandler) GetStorageClassYAML(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

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

	// 獲取StorageClass
	sc, err := clientset.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取StorageClass失敗", "error", err, "clusterId", clusterID, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取StorageClass失敗: %v", err))
		return
	}

	// 清理 managed fields、noisy annotations 並設定 TypeMeta
	clean := sc.DeepCopy()
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)
	clean.APIVersion = "storage.k8s.io/v1"
	clean.Kind = "StorageClass"

	// 轉換為YAML
	yamlData, err := yaml.Marshal(clean)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeleteStorageClass 刪除StorageClass
func (h *StorageHandler) DeleteStorageClass(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

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

	// 刪除StorageClass
	err = clientset.StorageV1().StorageClasses().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除StorageClass失敗", "error", err, "clusterId", clusterID, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除StorageClass失敗: %v", err))
		return
	}

	logger.Info("StorageClass刪除成功", "clusterId", clusterID, "name", name)
	response.NoContent(c)
}

// convertToStorageClassInfo 轉換為StorageClassInfo
func (h *StorageHandler) convertToStorageClassInfo(sc *storagev1.StorageClass) StorageClassInfo {
	reclaimPolicy := ""
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = string(*sc.ReclaimPolicy)
	}

	volumeBindingMode := ""
	if sc.VolumeBindingMode != nil {
		volumeBindingMode = string(*sc.VolumeBindingMode)
	}

	allowVolumeExpansion := false
	if sc.AllowVolumeExpansion != nil {
		allowVolumeExpansion = *sc.AllowVolumeExpansion
	}

	// 檢查是否為預設StorageClass
	isDefault := false
	if sc.Annotations != nil {
		if val, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
			isDefault = true
		}
		if val, ok := sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"]; ok && val == "true" {
			isDefault = true
		}
	}

	return StorageClassInfo{
		Name:                 sc.Name,
		Provisioner:          sc.Provisioner,
		ReclaimPolicy:        reclaimPolicy,
		VolumeBindingMode:    volumeBindingMode,
		AllowVolumeExpansion: allowVolumeExpansion,
		Parameters:           sc.Parameters,
		MountOptions:         sc.MountOptions,
		IsDefault:            isDefault,
		CreatedAt:            sc.CreationTimestamp.Time,
		Labels:               sc.Labels,
		Annotations:          sc.Annotations,
	}
}

// filterStorageClasses 過濾StorageClasses
func (h *StorageHandler) filterStorageClasses(scs []StorageClassInfo, search string) []StorageClassInfo {
	if search == "" {
		return scs
	}

	filtered := make([]StorageClassInfo, 0)
	searchLower := strings.ToLower(search)
	for _, sc := range scs {
		if strings.Contains(strings.ToLower(sc.Name), searchLower) ||
			strings.Contains(strings.ToLower(sc.Provisioner), searchLower) {
			filtered = append(filtered, sc)
		}
	}
	return filtered
}
