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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"encoding/json"

	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/middleware"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

type SecretHandler struct {
	db         *gorm.DB
	cfg        *config.Config
	clusterSvc *services.ClusterService
	k8sMgr     *k8s.ClusterInformerManager
}

func NewSecretHandler(db *gorm.DB, cfg *config.Config, clusterSvc *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *SecretHandler {
	return &SecretHandler{
		db:         db,
		cfg:        cfg,
		clusterSvc: clusterSvc,
		k8sMgr:     k8sMgr,
	}
}

// SecretListItem Secret列表項
type SecretListItem struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              string            `json:"type"`
	Labels            map[string]string `json:"labels"`
	DataCount         int               `json:"dataCount"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Age               string            `json:"age"`
}

// SecretDetail Secret詳情
type SecretDetail struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              string            `json:"type"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	Data              map[string]string `json:"data"` // Base64編碼的資料
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Age               string            `json:"age"`
	ResourceVersion   string            `json:"resourceVersion"`
}

// GetSecrets 獲取Secret列表
func (h *SecretHandler) GetSecrets(c *gin.Context) {
	clusterID := c.Param("clusterID")
	namespace := c.Query("namespace") // 支援過濾命名空間
	name := c.Query("name")           // 支援搜尋名稱
	secretType := c.Query("type")     // 支援按型別過濾 (如 kubernetes.io/dockerconfigjson)
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

	// 從 informer 快取獲取 Secret 列表
	var secrets []corev1.Secret
	sel := labels.Everything()

	if namespace != "" && namespace != "_all_" {
		// 獲取指定命名空間的 Secrets
		secs, err := h.k8sMgr.SecretsLister(cluster.ID).Secrets(namespace).List(sel)
		if err != nil {
			logger.Error("讀取Secret快取失敗", "cluster", cluster.Name, "namespace", namespace, "error", err)
			response.InternalError(c, fmt.Sprintf("獲取Secret列表失敗: %v", err))
			return
		}
		// 轉換為 []corev1.Secret
		for _, sec := range secs {
			secrets = append(secrets, *sec)
		}
	} else {
		// 獲取所有命名空間的 Secrets
		secs, err := h.k8sMgr.SecretsLister(cluster.ID).List(sel)
		if err != nil {
			logger.Error("讀取Secret快取失敗", "cluster", cluster.Name, "error", err)
			response.InternalError(c, fmt.Sprintf("獲取Secret列表失敗: %v", err))
			return
		}
		// 轉換為 []corev1.Secret
		for _, sec := range secs {
			secrets = append(secrets, *sec)
		}
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && (namespace == "" || namespace == "_all_") {
		secrets = middleware.FilterResourcesByNamespace(c, secrets, func(s corev1.Secret) string {
			return s.Namespace
		})
	}

	// 過濾和轉換
	var items []SecretListItem
	for _, secret := range secrets {
		// 名稱過濾
		if name != "" && !strings.Contains(strings.ToLower(secret.Name), strings.ToLower(name)) {
			continue
		}

		// 型別過濾
		if secretType != "" && string(secret.Type) != secretType {
			continue
		}

		item := SecretListItem{
			Name:              secret.Name,
			Namespace:         secret.Namespace,
			Type:              string(secret.Type),
			Labels:            secret.Labels,
			DataCount:         len(secret.Data),
			CreationTimestamp: secret.CreationTimestamp.Time,
			Age:               formatAge(time.Since(secret.CreationTimestamp.Time)),
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

// GetSecret 獲取Secret詳情
func (h *SecretHandler) GetSecret(c *gin.Context) {
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

	// 獲取Secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Secret失敗", "cluster", cluster.Name, "namespace", namespace, "name", name, "error", err)
		response.NotFound(c, fmt.Sprintf("Secret不存在: %v", err))
		return
	}

	// 將Data位元組陣列轉換為Base64字串
	dataStr := make(map[string]string)
	for k, v := range secret.Data {
		dataStr[k] = string(v) // 前端需要Base64解碼顯示
	}

	detail := SecretDetail{
		Name:              secret.Name,
		Namespace:         secret.Namespace,
		Type:              string(secret.Type),
		Labels:            secret.Labels,
		Annotations:       secret.Annotations,
		Data:              dataStr,
		CreationTimestamp: secret.CreationTimestamp.Time,
		Age:               formatAge(time.Since(secret.CreationTimestamp.Time)),
		ResourceVersion:   secret.ResourceVersion,
	}

	response.OK(c, detail)
}

// GetSecretNamespaces 獲取Secret所在的命名空間列表
func (h *SecretHandler) GetSecretNamespaces(c *gin.Context) {
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

	// 建立K8s客戶端
	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 獲取所有Secrets
	secrets, err := clientset.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取Secret列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取Secret列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的Secret數量
	nsMap := make(map[string]int)
	for _, secret := range secrets.Items {
		nsMap[secret.Namespace]++
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

	created, err := clientset.CoreV1().Secrets(req.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		logger.Error("建立Secret失敗", "cluster", cluster.Name, "namespace", req.Namespace, "name", req.Name, "error", err)
		if k8serrors.IsInvalid(err) || k8serrors.IsAlreadyExists(err) {
			response.BadRequest(c, fmt.Sprintf("建立Secret失敗: %v", err))
		} else {
			response.InternalError(c, fmt.Sprintf("建立Secret失敗: %v", err))
		}
		return
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
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	contentBytes, _ := json.Marshal(map[string]interface{}{"keys": keys, "note": "Secret value not stored for security"})
	var nextVer int
	h.db.Model(&models.ConfigVersion{}).
		Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ?", clusterID, "secret", namespace, name).
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
		ResourceType: "secret",
		Namespace:    namespace,
		Name:         name,
		Version:      nextVer,
		ContentJSON:  string(contentBytes),
		ChangedBy:    changedBy,
		ChangedAt:    time.Now(),
	}
	if err := h.db.Create(ver).Error; err != nil {
		logger.Warn("儲存 Secret 版本快照失敗", "error", err)
	}
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
	var versions []models.ConfigVersion
	h.db.Where("cluster_id = ? AND resource_type = ? AND namespace = ? AND name = ?",
		uint(id), "secret", namespace, name).
		Order("version DESC").
		Find(&versions)
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
