package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"encoding/json"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

type ConfigMapHandler struct {
	db         *gorm.DB
	cfg        *config.Config
	clusterSvc *services.ClusterService
	k8sMgr     *k8s.ClusterInformerManager
}

func NewConfigMapHandler(db *gorm.DB, cfg *config.Config, clusterSvc *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *ConfigMapHandler {
	return &ConfigMapHandler{
		db:         db,
		cfg:        cfg,
		clusterSvc: clusterSvc,
		k8sMgr:     k8sMgr,
	}
}

// ConfigMapListItem ConfigMap列表項
type ConfigMapListItem struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels"`
	DataCount         int               `json:"dataCount"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Age               string            `json:"age"`
}

// ConfigMapDetail ConfigMap詳情
type ConfigMapDetail struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	Data              map[string]string `json:"data"`
	BinaryData        map[string][]byte `json:"binaryData,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Age               string            `json:"age"`
	ResourceVersion   string            `json:"resourceVersion"`
}

// GetConfigMaps 獲取ConfigMap列表
func (h *ConfigMapHandler) GetConfigMaps(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Query("namespace") // 支援過濾命名空間
	name := c.Query("name")           // 支援搜尋名稱
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 確保 informer 已啟動並同步
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	// 從 informer 快取獲取 ConfigMap 列表
	var configMaps []corev1.ConfigMap
	sel := labels.Everything()

	if namespace != "" && namespace != "_all_" {
		// 獲取指定命名空間的 ConfigMaps
		cms, err := h.k8sMgr.ConfigMapsLister(cluster.ID).ConfigMaps(namespace).List(sel)
		if err != nil {
			logger.Error("讀取ConfigMap快取失敗", "cluster", cluster.Name, "namespace", namespace, "error", err)
			response.InternalError(c, fmt.Sprintf("獲取ConfigMap列表失敗: %v", err))
			return
		}
		// 轉換為 []corev1.ConfigMap
		for _, cm := range cms {
			configMaps = append(configMaps, *cm)
		}
	} else {
		// 獲取所有命名空間的 ConfigMaps
		cms, err := h.k8sMgr.ConfigMapsLister(cluster.ID).List(sel)
		if err != nil {
			logger.Error("讀取ConfigMap快取失敗", "cluster", cluster.Name, "error", err)
			response.InternalError(c, fmt.Sprintf("獲取ConfigMap列表失敗: %v", err))
			return
		}
		// 轉換為 []corev1.ConfigMap
		for _, cm := range cms {
			configMaps = append(configMaps, *cm)
		}
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && (namespace == "" || namespace == "_all_") {
		configMaps = middleware.FilterResourcesByNamespace(c, configMaps, func(cm corev1.ConfigMap) string {
			return cm.Namespace
		})
	}

	// 過濾和轉換
	var items []ConfigMapListItem
	for _, cm := range configMaps {
		// 名稱過濾
		if name != "" && !strings.Contains(strings.ToLower(cm.Name), strings.ToLower(name)) {
			continue
		}

		item := ConfigMapListItem{
			Name:              cm.Name,
			Namespace:         cm.Namespace,
			Labels:            cm.Labels,
			DataCount:         len(cm.Data) + len(cm.BinaryData),
			CreationTimestamp: cm.CreationTimestamp.Time,
			Age:               formatAge(time.Since(cm.CreationTimestamp.Time)),
		}
		items = append(items, item)
	}

	// 分頁
	total := len(items)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pagedItems := items[start:end]

	response.PagedList(c, pagedItems, int64(total), page, pageSize)
}

// GetConfigMap 獲取ConfigMap詳情
func (h *ConfigMapHandler) GetConfigMap(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
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

	// 獲取ConfigMap
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取ConfigMap失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.NotFound(c, fmt.Sprintf("ConfigMap不存在: %v", err))
		return
	}

	detail := ConfigMapDetail{
		Name:              cm.Name,
		Namespace:         cm.Namespace,
		Labels:            cm.Labels,
		Annotations:       cm.Annotations,
		Data:              cm.Data,
		BinaryData:        cm.BinaryData,
		CreationTimestamp: cm.CreationTimestamp.Time,
		Age:               formatAge(time.Since(cm.CreationTimestamp.Time)),
		ResourceVersion:   cm.ResourceVersion,
	}

	response.OK(c, detail)
}

// GetConfigMapNamespaces 獲取ConfigMap所在的命名空間列表
func (h *ConfigMapHandler) GetConfigMapNamespaces(c *gin.Context) {
	clusterID := c.Param("clusterID")

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
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

	// 獲取所有ConfigMaps
	configMaps, err := clientset.CoreV1().ConfigMaps("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取ConfigMap列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取ConfigMap列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的ConfigMap數量
	nsMap := make(map[string]int)
	for _, cm := range configMaps.Items {
		nsMap[cm.Namespace]++
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

	response.OK(c, namespaces)
}

// DeleteConfigMap 刪除ConfigMap
func (h *ConfigMapHandler) DeleteConfigMap(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
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

	// 刪除ConfigMap
	err = clientset.CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除ConfigMap失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.InternalError(c, fmt.Sprintf("刪除ConfigMap失敗: %v", err))
		return
	}

	response.NoContent(c)
}

// CreateConfigMap 建立ConfigMap
func (h *ConfigMapHandler) CreateConfigMap(c *gin.Context) {
	clusterID := c.Param("clusterID")

	var req struct {
		Name        string            `json:"name" binding:"required"`
		Namespace   string            `json:"namespace" binding:"required"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Data        map[string]string `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("請求參數錯誤: %v", err))
		return
	}

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
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

	// 建立ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   req.Namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Data: req.Data,
	}

	created, err := clientset.CoreV1().ConfigMaps(req.Namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		logger.Error("建立ConfigMap失敗", "cluster", cluster.Name, "namespace", req.Namespace, "name", req.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("建立ConfigMap失敗: %v", err))
		return
	}

	response.OK(c, gin.H{
		"name":      created.Name,
		"namespace": created.Namespace,
	})
}

// UpdateConfigMap 更新ConfigMap
func (h *ConfigMapHandler) UpdateConfigMap(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Data        map[string]string `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("請求參數錯誤: %v", err))
		return
	}

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 建立K8s客戶端
	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 獲取現有ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取ConfigMap失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.NotFound(c, fmt.Sprintf("ConfigMap不存在: %v", err))
		return
	}

	// 更新前儲存版本快照
	h.saveConfigVersion(uint(id), "configmap", namespace, name, configMap.Data, c)

	// 更新ConfigMap
	configMap.Labels = req.Labels
	configMap.Annotations = req.Annotations
	configMap.Data = req.Data

	updated, err := clientset.CoreV1().ConfigMaps(namespace).Update(context.Background(), configMap, metav1.UpdateOptions{})
	if err != nil {
		logger.Error("更新ConfigMap失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.InternalError(c, fmt.Sprintf("更新ConfigMap失敗: %v", err))
		return
	}

	response.OK(c, gin.H{
		"name":            updated.Name,
		"namespace":       updated.Namespace,
		"resourceVersion": updated.ResourceVersion,
	})
}

// formatAge 格式化時間差
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// ─── 版本歷史 ─────────────────────────────────────────────────────────────────

// saveConfigVersion 儲存 ConfigMap 版本快照（非同步，不影響主流程）
func (h *ConfigMapHandler) saveConfigVersion(clusterID uint, rType, namespace, name string, data map[string]string, c *gin.Context) {
	contentBytes, _ := json.Marshal(data)
	var nextVer int
	h.db.Model(&models.ConfigVersion{}).
		Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ?", clusterID, rType, namespace, name).
		Select("COALESCE(MAX(version),0) + 1").Scan(&nextVer)
	if nextVer == 0 {
		nextVer = 1
	}
	changedBy := ""
	if u, ok := c.Get("username"); ok {
		changedBy, _ = u.(string)
	}
	ver := &models.ConfigVersion{
		ClusterID:    clusterID,
		ResourceType: rType,
		Namespace:    namespace,
		Name:         name,
		Version:      nextVer,
		ContentJSON:  string(contentBytes),
		ChangedBy:    changedBy,
		ChangedAt:    time.Now(),
	}
	if err := h.db.Create(ver).Error; err != nil {
		logger.Warn("儲存 ConfigMap 版本快照失敗", "error", err)
	}
}

// GetConfigMapVersions 取得 ConfigMap 版本列表
func (h *ConfigMapHandler) GetConfigMapVersions(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	id, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	var versions []models.ConfigVersion
	h.db.Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ?",
		uint(id), "configmap", namespace, name).
		Order("version DESC").
		Find(&versions)
	response.OK(c, gin.H{"items": versions, "total": len(versions)})
}

// RollbackConfigMap 回滾 ConfigMap 到指定版本
func (h *ConfigMapHandler) RollbackConfigMap(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")
	versionStr := c.Param("version")
	id, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	version, _ := strconv.Atoi(versionStr)

	var ver models.ConfigVersion
	if err := h.db.Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ? AND version = ?",
		uint(id), "configmap", namespace, name, version).First(&ver).Error; err != nil {
		response.NotFound(c, "版本不存在")
		return
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(ver.ContentJSON), &data); err != nil {
		response.InternalError(c, "版本資料損壞: "+err.Error())
		return
	}

	cluster, err := h.clusterSvc.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}
	clientset := k8sClient.GetClientset()
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "ConfigMap 不存在")
		return
	}
	// 回滾前先存快照
	h.saveConfigVersion(uint(id), "configmap", namespace, name, cm.Data, c)
	cm.Data = data
	if _, err := clientset.CoreV1().ConfigMaps(namespace).Update(context.Background(), cm, metav1.UpdateOptions{}); err != nil {
		response.InternalError(c, "回滾失敗: "+err.Error())
		return
	}
	logger.Info("回滾 ConfigMap", "cluster", clusterIDStr, "namespace", namespace, "name", name, "version", version)
	response.OK(c, gin.H{"message": fmt.Sprintf("已回滾至版本 %d", version)})
}
