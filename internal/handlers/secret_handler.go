package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

type SecretHandler struct {
	cfgVerSvc  *services.ConfigVersionService
	cfg        *config.Config
	clusterSvc *services.ClusterService
	k8sMgr     *k8s.ClusterInformerManager
}

func NewSecretHandler(cfgVerSvc *services.ConfigVersionService, cfg *config.Config, clusterSvc *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *SecretHandler {
	return &SecretHandler{
		cfgVerSvc:  cfgVerSvc,
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
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

	warnLargeDataset(c, total)
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
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

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 獲取所有Secrets
	secrets, err := clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
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
