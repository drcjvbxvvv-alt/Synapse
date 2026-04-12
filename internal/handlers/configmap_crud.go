package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 刪除ConfigMap
	err = clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
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
		DryRun      bool              `json:"dryRun"`
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

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

	createOpts := metav1.CreateOptions{}
	if req.DryRun {
		createOpts.DryRun = []string{metav1.DryRunAll}
	}

	created, err := clientset.CoreV1().ConfigMaps(req.Namespace).Create(ctx, configMap, createOpts)
	if err != nil {
		logger.Error("建立ConfigMap失敗", "cluster", cluster.Name, "namespace", req.Namespace, "name", req.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("建立ConfigMap失敗: %v", err))
		return
	}

	if !req.DryRun {
		logger.Info("建立ConfigMap成功", "cluster", cluster.Name, "namespace", req.Namespace, "name", req.Name)
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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取現有ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
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

	updated, err := clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
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

// ─── 版本歷史 ─────────────────────────────────────────────────────────────────

// saveConfigVersion 儲存 ConfigMap 版本快照（非同步，不影響主流程）
func (h *ConfigMapHandler) saveConfigVersion(clusterID uint, _, namespace, name string, data map[string]string, c *gin.Context) {
	changedBy := ""
	if u, ok := c.Get("username"); ok {
		changedBy, _ = u.(string)
	}
	h.cfgVerSvc.SaveConfigMapVersion(c.Request.Context(), clusterID, namespace, name, changedBy, data)
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	versions, err := h.cfgVerSvc.ListVersions(ctx, uint(id), "configmap", namespace, name) //nolint:gosec
	if err != nil {
		response.InternalError(c, "查詢版本失敗")
		return
	}
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

	ctxDB, cancelDB := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancelDB()
	ver, err := h.cfgVerSvc.GetVersion(ctxDB, uint(id), "configmap", namespace, name, version) //nolint:gosec
	if err != nil {
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "ConfigMap 不存在")
		return
	}
	// 回滾前先存快照
	h.saveConfigVersion(uint(id), "configmap", namespace, name, cm.Data, c)
	cm.Data = data
	if _, err := clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		response.InternalError(c, "回滾失敗: "+err.Error())
		return
	}
	logger.Info("回滾 ConfigMap", "cluster", clusterIDStr, "namespace", namespace, "name", name, "version", version)
	response.OK(c, gin.H{"message": fmt.Sprintf("已回滾至版本 %d", version)})
}
