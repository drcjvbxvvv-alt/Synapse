package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// DeleteSecret 刪除Secret
func (h *SecretHandler) DeleteSecret(c *gin.Context) {
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

	// 刪除Secret
	err = clientset.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除Secret失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.InternalError(c, fmt.Sprintf("刪除Secret失敗: %v", err))
		return
	}

	response.NoContent(c)
}

// CreateSecret 建立Secret
func (h *SecretHandler) CreateSecret(c *gin.Context) {
	clusterID := c.Param("clusterID")

	var req struct {
		Name        string            `json:"name" binding:"required"`
		Namespace   string            `json:"namespace" binding:"required"`
		Type        string            `json:"type"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Data        map[string]string `json:"data"` // Base64編碼的資料
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

	// 將字串資料轉換為位元組陣列
	dataBytes := make(map[string][]byte)
	for k, v := range req.Data {
		dataBytes[k] = []byte(v)
	}

	// 預設型別
	secretType := corev1.SecretTypeOpaque
	if req.Type != "" {
		secretType = corev1.SecretType(req.Type)
	}

	// 建立Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   req.Namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Type: secretType,
		Data: dataBytes,
	}

	createOpts := metav1.CreateOptions{}
	if req.DryRun {
		createOpts.DryRun = []string{metav1.DryRunAll}
	}

	created, err := clientset.CoreV1().Secrets(req.Namespace).Create(context.Background(), secret, createOpts)
	if err != nil {
		logger.Error("建立Secret失敗", "cluster", cluster.Name, "namespace", req.Namespace, "name", req.Name, "error", err)
		if k8serrors.IsInvalid(err) || k8serrors.IsAlreadyExists(err) {
			response.BadRequest(c, fmt.Sprintf("建立Secret失敗: %v", err))
		} else {
			response.InternalError(c, fmt.Sprintf("建立Secret失敗: %v", err))
		}
		return
	}

	if !req.DryRun {
		logger.Info("建立Secret成功", "cluster", cluster.Name, "namespace", req.Namespace, "name", req.Name)
	}

	response.OK(c, gin.H{
		"name":      created.Name,
		"namespace": created.Namespace,
	})
}

// UpdateSecret 更新Secret
func (h *SecretHandler) UpdateSecret(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Data        map[string]string `json:"data"` // Base64編碼的資料
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

	// 獲取現有Secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Secret失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.NotFound(c, fmt.Sprintf("Secret不存在: %v", err))
		return
	}

	// 更新前儲存版本快照（只儲存 key 列表，不存明文 value）
	h.saveSecretVersion(uint(id), namespace, name, secret.Data, c)

	// 將字串資料轉換為位元組陣列
	dataBytes := make(map[string][]byte)
	for k, v := range req.Data {
		dataBytes[k] = []byte(v)
	}

	// 更新Secret
	secret.Labels = req.Labels
	secret.Annotations = req.Annotations
	secret.Data = dataBytes

	updated, err := clientset.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		logger.Error("更新Secret失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.InternalError(c, fmt.Sprintf("更新Secret失敗: %v", err))
		return
	}

	response.OK(c, gin.H{
		"name":            updated.Name,
		"namespace":       updated.Namespace,
		"resourceVersion": updated.ResourceVersion,
	})
}

// ─── 版本歷史 ─────────────────────────────────────────────────────────────────

// saveSecretVersion 儲存 Secret 版本快照（只記錄 key 列表，不儲存明文 value）
func (h *SecretHandler) saveSecretVersion(clusterID uint, namespace, name string, data map[string][]byte, c *gin.Context) {
	changedBy := ""
	if u, ok := c.Get("username"); ok {
		changedBy, _ = u.(string)
	}
	h.cfgVerSvc.SaveSecretVersion(c.Request.Context(), clusterID, namespace, name, changedBy, data)
}

// GetSecretVersions 取得 Secret 版本列表
func (h *SecretHandler) GetSecretVersions(c *gin.Context) {
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
	versions, err := h.cfgVerSvc.ListVersions(ctx, uint(id), "secret", namespace, name) //nolint:gosec
	if err != nil {
		response.InternalError(c, "查詢版本失敗")
		return
	}
	// 隱藏 contentJSON（安全考量）
	items := make([]map[string]interface{}, 0, len(versions))
	for _, v := range versions {
		items = append(items, map[string]interface{}{
			"id":        v.ID,
			"version":   v.Version,
			"changedBy": v.ChangedBy,
			"changedAt": v.ChangedAt,
			"note":      "Secret 內容不顯示，僅記錄變更時間與操作者",
		})
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}
