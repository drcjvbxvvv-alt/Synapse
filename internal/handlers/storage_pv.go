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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// PVInfo PV資訊
type PVInfo struct {
	Name                   string            `json:"name"`
	Status                 string            `json:"status"`
	Capacity               string            `json:"capacity"`
	AccessModes            []string          `json:"accessModes"`
	ReclaimPolicy          string            `json:"reclaimPolicy"`
	StorageClassName       string            `json:"storageClassName"`
	VolumeMode             string            `json:"volumeMode"`
	ClaimRef               *PVClaimRef       `json:"claimRef,omitempty"`
	PersistentVolumeSource string            `json:"persistentVolumeSource"`
	MountOptions           []string          `json:"mountOptions,omitempty"`
	NodeAffinity           string            `json:"nodeAffinity,omitempty"`
	CreatedAt              time.Time         `json:"createdAt"`
	Labels                 map[string]string `json:"labels"`
	Annotations            map[string]string `json:"annotations"`
}

// PVClaimRef PV宣告引用
type PVClaimRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ListPVs 獲取PV列表
func (h *StorageHandler) ListPVs(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取PVs
	pvList, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取PVs失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取PVs失敗: %v", err))
		return
	}

	pvs := make([]PVInfo, 0, len(pvList.Items))
	for _, pv := range pvList.Items {
		pvs = append(pvs, h.convertToPVInfo(&pv))
	}

	// 過濾和搜尋
	filteredPVs := h.filterPVs(pvs, status, search)

	// 排序
	sort.Slice(filteredPVs, func(i, j int) bool {
		return filteredPVs[i].CreatedAt.After(filteredPVs[j].CreatedAt)
	})

	// 分頁
	total := len(filteredPVs)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedPVs := filteredPVs[start:end]

	response.PagedList(c, pagedPVs, int64(total), page, pageSize)
}

// GetPV 獲取單個PV詳情
func (h *StorageHandler) GetPV(c *gin.Context) {
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

	// 獲取PV
	pv, err := clientset.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取PV失敗", "error", err, "clusterId", clusterID, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取PV失敗: %v", err))
		return
	}

	pvInfo := h.convertToPVInfo(pv)

	response.OK(c, pvInfo)
}

// GetPVYAML 獲取PV的YAML
func (h *StorageHandler) GetPVYAML(c *gin.Context) {
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

	// 獲取PV
	pv, err := clientset.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取PV失敗", "error", err, "clusterId", clusterID, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取PV失敗: %v", err))
		return
	}

	// 轉換為YAML
	yamlData, err := yaml.Marshal(pv)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeletePV 刪除PV
func (h *StorageHandler) DeletePV(c *gin.Context) {
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

	// 刪除PV
	err = clientset.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除PV失敗", "error", err, "clusterId", clusterID, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除PV失敗: %v", err))
		return
	}

	logger.Info("PV刪除成功", "clusterId", clusterID, "name", name)
	response.NoContent(c)
}

// convertToPVInfo 轉換為PVInfo
func (h *StorageHandler) convertToPVInfo(pv *corev1.PersistentVolume) PVInfo {
	accessModes := make([]string, 0, len(pv.Spec.AccessModes))
	for _, mode := range pv.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	capacity := ""
	if pv.Spec.Capacity != nil {
		if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = storage.String()
		}
	}

	volumeMode := ""
	if pv.Spec.VolumeMode != nil {
		volumeMode = string(*pv.Spec.VolumeMode)
	}

	var claimRef *PVClaimRef
	if pv.Spec.ClaimRef != nil {
		claimRef = &PVClaimRef{
			Namespace: pv.Spec.ClaimRef.Namespace,
			Name:      pv.Spec.ClaimRef.Name,
		}
	}

	// 獲取PersistentVolumeSource型別
	sourceType := h.getPVSourceType(&pv.Spec.PersistentVolumeSource)

	return PVInfo{
		Name:                   pv.Name,
		Status:                 string(pv.Status.Phase),
		Capacity:               capacity,
		AccessModes:            accessModes,
		ReclaimPolicy:          string(pv.Spec.PersistentVolumeReclaimPolicy),
		StorageClassName:       pv.Spec.StorageClassName,
		VolumeMode:             volumeMode,
		ClaimRef:               claimRef,
		PersistentVolumeSource: sourceType,
		MountOptions:           pv.Spec.MountOptions,
		CreatedAt:              pv.CreationTimestamp.Time,
		Labels:                 pv.Labels,
		Annotations:            pv.Annotations,
	}
}

// getPVSourceType 獲取PV源型別
func (h *StorageHandler) getPVSourceType(source *corev1.PersistentVolumeSource) string {
	if source.HostPath != nil {
		return "HostPath"
	}
	if source.NFS != nil {
		return "NFS"
	}
	if source.ISCSI != nil {
		return "iSCSI"
	}
	if source.Cinder != nil {
		return "Cinder"
	}
	if source.CephFS != nil {
		return "CephFS"
	}
	if source.FC != nil {
		return "FC"
	}
	if source.FlexVolume != nil {
		return "FlexVolume"
	}
	if source.AWSElasticBlockStore != nil {
		return "AWSElasticBlockStore"
	}
	if source.GCEPersistentDisk != nil {
		return "GCEPersistentDisk"
	}
	if source.AzureDisk != nil {
		return "AzureDisk"
	}
	if source.AzureFile != nil {
		return "AzureFile"
	}
	if source.VsphereVolume != nil {
		return "vSphereVolume"
	}
	if source.RBD != nil {
		return "RBD"
	}
	if source.Glusterfs != nil {
		return "Glusterfs"
	}
	if source.Local != nil {
		return "Local"
	}
	if source.CSI != nil {
		return "CSI"
	}
	return "Unknown"
}

// filterPVs 過濾PVs
func (h *StorageHandler) filterPVs(pvs []PVInfo, status, search string) []PVInfo {
	filtered := make([]PVInfo, 0)
	for _, pv := range pvs {
		// 狀態過濾
		if status != "" && pv.Status != status {
			continue
		}

		// 搜尋過濾
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(pv.Name), searchLower) &&
				!strings.Contains(strings.ToLower(pv.StorageClassName), searchLower) &&
				!strings.Contains(strings.ToLower(pv.PersistentVolumeSource), searchLower) {
				continue
			}
		}

		filtered = append(filtered, pv)
	}
	return filtered
}
