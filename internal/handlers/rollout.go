package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	rollouts "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	sigsyaml "sigs.k8s.io/yaml"
)

func init() {
	// 註冊Argo Rollouts型別到scheme
	_ = rollouts.AddToScheme(scheme.Scheme)
}

// RolloutHandler Rollout處理器
type RolloutHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewRolloutHandler 建立Rollout處理器
func NewRolloutHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *RolloutHandler {
	return &RolloutHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// RolloutInfo Rollout資訊
type RolloutInfo struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              string            `json:"type"`
	Status            string            `json:"status"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"readyReplicas"`
	AvailableReplicas int32             `json:"availableReplicas"`
	UpdatedReplicas   int32             `json:"updatedReplicas"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreatedAt         time.Time         `json:"createdAt"`
	Images            []string          `json:"images"`
	Selector          map[string]string `json:"selector"`
	Strategy          string            `json:"strategy"`
}

// CheckRolloutCRD 檢查叢集是否安裝了 Argo Rollouts CRD
func (h *RolloutHandler) CheckRolloutCRD(c *gin.Context) {
	clusterId := c.Param("clusterID")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 檢查 RolloutsLister 是否可用
	lister := h.k8sMgr.RolloutsLister(cluster.ID)
	enabled := lister != nil

	response.OK(c, gin.H{"enabled": enabled})
}

// ListRollouts 獲取Rollout列表
func (h *RolloutHandler) ListRollouts(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	searchName := c.Query("search")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	logger.Info("獲取Rollout列表: cluster=%s, namespace=%s, search=%s", clusterId, namespace, searchName)

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	var rolloutList []RolloutInfo
	sel := labels.Everything()

	// 檢查 Argo Rollouts CRD 是否存在
	lister := h.k8sMgr.RolloutsLister(cluster.ID)
	if lister == nil {
		// 叢集未安裝 Argo Rollouts CRD，返回空列表
		response.OK(c, gin.H{
			"items":           []RolloutInfo{},
			"total":           0,
			"page":            page,
			"pageSize":        pageSize,
			"rolloutEnabled":  false,
			"rolloutDisabled": true,
		})
		return
	}

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	// 從Informer快取讀取
	if namespace != "" {
		rs, err := lister.Rollouts(namespace).List(sel)
		if err != nil {
			logger.Error("讀取Rollout快取失敗", "error", err)
		} else {
			for _, r := range rs {
				rolloutList = append(rolloutList, h.convertToRolloutInfo(r))
			}
		}
	} else {
		rs, err := lister.List(sel)
		if err != nil {
			logger.Error("讀取Rollout快取失敗", "error", err)
		} else {
			for _, r := range rs {
				rolloutList = append(rolloutList, h.convertToRolloutInfo(r))
			}
		}
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		rolloutList = middleware.FilterResourcesByNamespace(c, rolloutList, func(ro RolloutInfo) string {
			return ro.Namespace
		})
	}

	// 搜尋過濾
	if searchName != "" {
		var filtered []RolloutInfo
		searchLower := strings.ToLower(searchName)
		for _, ro := range rolloutList {
			if strings.Contains(strings.ToLower(ro.Name), searchLower) {
				filtered = append(filtered, ro)
			}
		}
		rolloutList = filtered
	}

	// 排序
	sort.Slice(rolloutList, func(i, j int) bool {
		return rolloutList[i].CreatedAt.After(rolloutList[j].CreatedAt)
	})

	// 分頁
	total := len(rolloutList)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedRollouts := rolloutList[start:end]

	response.PagedList(c, pagedRollouts, int64(total), page, pageSize)
}

// GetRollout 獲取Rollout詳情
func (h *RolloutHandler) GetRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout詳情: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 獲取關聯的Pods
	clientset := k8sClient.GetClientset()
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(rollout.Spec.Selector),
	})
	if err != nil {
		logger.Error("獲取Rollout關聯Pods失敗", "error", err)
	}

	// 清理 managed fields 以生成更乾淨的 YAML
	cleanRollout := rollout.DeepCopy()
	cleanRollout.ManagedFields = nil
	// 設定 TypeMeta（client-go 返回的物件預設不包含 apiVersion 和 kind）
	cleanRollout.APIVersion = "argoproj.io/v1alpha1"
	cleanRollout.Kind = "Rollout"
	// 將 Rollout 物件轉換為 YAML 字串
	yamlBytes, yamlErr := sigsyaml.Marshal(cleanRollout)
	var yamlString string
	if yamlErr == nil {
		yamlString = string(yamlBytes)
	} else {
		logger.Error("轉換Rollout為YAML失敗", "error", yamlErr)
		yamlString = ""
	}

	response.OK(c, gin.H{
		"workload": h.convertToRolloutInfo(rollout),
		"raw":      cleanRollout,
		"yaml":     yamlString,
		"pods":     pods,
	})
}

// GetRolloutNamespaces 獲取包含Rollout的命名空間列表
func (h *RolloutHandler) GetRolloutNamespaces(c *gin.Context) {
	clusterId := c.Param("clusterID")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 確保 informer 快取就緒
	if _, err := h.k8sMgr.EnsureAndWait(ctx, cluster, 5*time.Second); err != nil {
		response.ServiceUnavailable(c, "informer 未就緒: "+err.Error())
		return
	}

	// 從Informer讀取所有Rollouts並統計命名空間
	sel := labels.Everything()
	lister := h.k8sMgr.RolloutsLister(cluster.ID)
	if lister == nil {
		// 叢集未安裝 Argo Rollouts CRD，返回空列表
		response.OK(c, []interface{}{})
		return
	}
	rs, err := lister.List(sel)
	if err != nil {
		response.InternalError(c, "讀取Rollout快取失敗: "+err.Error())
		return
	}

	// 統計每個命名空間的Rollout數量
	nsCount := make(map[string]int)
	for _, ro := range rs {
		nsCount[ro.Namespace]++
	}

	// 轉換為列表格式
	type NamespaceInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceInfo
	for ns, count := range nsCount {
		namespaces = append(namespaces, NamespaceInfo{
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

// ScaleRollout 擴縮容Rollout
func (h *RolloutHandler) ScaleRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("擴縮容Rollout: %s/%s/%s to %d", clusterId, namespace, name, req.Replicas)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	// 獲取Rollout
	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "獲取Rollout失敗: "+err.Error())
		return
	}

	// 更新副本數
	rollout.Spec.Replicas = &req.Replicas
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Update(ctx, rollout, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "擴縮容失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "擴縮容成功"})
}

// ApplyYAML 應用Rollout YAML
func (h *RolloutHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("應用Rollout YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 解析YAML
	var objMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &objMap); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	// 驗證必要欄位
	if objMap["apiVersion"] == nil || objMap["kind"] == nil {
		response.BadRequest(c, "YAML缺少必要欄位: apiVersion 或 kind")
		return
	}

	kind := objMap["kind"].(string)
	if kind != "Rollout" {
		response.BadRequest(c, "YAML型別錯誤，期望Rollout，實際為: "+kind)
		return
	}

	// 獲取metadata
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		response.BadRequest(c, "YAML缺少 metadata 欄位")
		return
	}

	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	// 檢查 Argo Rollouts CRD 是否已安裝
	if h.k8sMgr.RolloutsLister(cluster.ID) == nil {
		response.BadRequest(c, "叢集未安裝 Argo Rollouts，請先安裝後再建立 Rollout 資源")
		return
	}

	// 應用YAML
	result, err := h.applyYAML(ctx, k8sClient, req.YAML, namespace, req.DryRun)
	if err != nil {
		response.InternalError(c, "YAML應用失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}

// DeleteRollout 刪除Rollout
func (h *RolloutHandler) DeleteRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("刪除Rollout: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}

// 輔助方法：轉換Rollout到RolloutInfo
func (h *RolloutHandler) convertToRolloutInfo(r *rollouts.Rollout) RolloutInfo {
	status := "Healthy"
	if r.Status.Replicas == 0 {
		status = "Stopped"
	} else if r.Status.AvailableReplicas < r.Status.Replicas {
		status = "Degraded"
	}

	// 提取映像列表
	var images []string
	for _, container := range r.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	// 策略
	strategy := "Canary"
	if r.Spec.Strategy.BlueGreen != nil {
		strategy = "BlueGreen"
	}

	replicas := int32(0)
	if r.Spec.Replicas != nil {
		replicas = *r.Spec.Replicas
	}

	return RolloutInfo{
		ID:                fmt.Sprintf("%s/%s", r.Namespace, r.Name),
		Name:              r.Name,
		Namespace:         r.Namespace,
		Type:              "Rollout",
		Status:            status,
		Replicas:          replicas,
		ReadyReplicas:     r.Status.ReadyReplicas,
		AvailableReplicas: r.Status.AvailableReplicas,
		UpdatedReplicas:   r.Status.UpdatedReplicas,
		Labels:            r.Labels,
		Annotations:       r.Annotations,
		CreatedAt:         r.CreationTimestamp.Time,
		Images:            images,
		Selector:          r.Spec.Selector.MatchLabels,
		Strategy:          strategy,
	}
}

// 輔助方法：應用YAML
func (h *RolloutHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	// 使用 sigsyaml 直接解析為 Rollout 結構（比 runtime serializer 更適合 CRD 類型）
	rollout := &rollouts.Rollout{}
	if err := sigsyaml.Unmarshal([]byte(yamlContent), rollout); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}
	if rollout.Name == "" || rollout.Kind == "" {
		return nil, fmt.Errorf("YAML缺少必要欄位 (name 或 kind)")
	}

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		return nil, fmt.Errorf("獲取Rollout客戶端失敗: %w", err)
	}

	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	// 嘗試獲取現有資源
	existing, err := rolloutClient.ArgoprojV1alpha1().Rollouts(rollout.Namespace).Get(ctx, rollout.Name, metav1.GetOptions{})
	if err == nil {
		// 資源存在，執行更新
		rollout.ResourceVersion = existing.ResourceVersion
		result, err := rolloutClient.ArgoprojV1alpha1().Rollouts(rollout.Namespace).Update(ctx, rollout, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// 資源不存在，執行建立
	result, err := rolloutClient.ArgoprojV1alpha1().Rollouts(rollout.Namespace).Create(ctx, rollout, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetRolloutPods 獲取Rollout關聯的Pods
func (h *RolloutHandler) GetRolloutPods(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的Pods: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 獲取關聯的Pods
	clientset := k8sClient.GetClientset()
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(rollout.Spec.Selector),
	})
	if err != nil {
		response.InternalError(c, "獲取Pods失敗: "+err.Error())
		return
	}

	// 轉換Pod資訊
	pods := make([]map[string]interface{}, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podInfo := map[string]interface{}{
			"name":         pod.Name,
			"namespace":    pod.Namespace,
			"phase":        string(pod.Status.Phase),
			"nodeName":     pod.Spec.NodeName,
			"nodeIP":       pod.Status.HostIP,
			"podIP":        pod.Status.PodIP,
			"restartCount": 0,
			"createdAt":    pod.CreationTimestamp.Time,
		}

		// 計算重啟次數和提取資源限制
		var totalRestarts int32
		var cpuRequest, cpuLimit, memoryRequest, memoryLimit string
		for _, container := range pod.Spec.Containers {
			// 資源限制
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests["cpu"]; ok {
					cpuRequest = cpu.String()
				}
				if mem, ok := container.Resources.Requests["memory"]; ok {
					memoryRequest = mem.String()
				}
			}
			if container.Resources.Limits != nil {
				if cpu, ok := container.Resources.Limits["cpu"]; ok {
					cpuLimit = cpu.String()
				}
				if mem, ok := container.Resources.Limits["memory"]; ok {
					memoryLimit = mem.String()
				}
			}
		}

		// 統計重啟次數
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}

		podInfo["restartCount"] = totalRestarts
		podInfo["cpuRequest"] = cpuRequest
		podInfo["cpuLimit"] = cpuLimit
		podInfo["memoryRequest"] = memoryRequest
		podInfo["memoryLimit"] = memoryLimit

		pods = append(pods, podInfo)
	}

	response.List(c, pods, int64(len(pods)))
}

// GetRolloutServices 獲取Rollout關聯的Services
func (h *RolloutHandler) GetRolloutServices(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的Services: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 獲取所有Services
	clientset := k8sClient.GetClientset()
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Services失敗: "+err.Error())
		return
	}

	// 篩選匹配的Services
	rolloutLabels := rollout.Spec.Selector.MatchLabels
	matchedServices := make([]map[string]interface{}, 0)
	for _, svc := range serviceList.Items {
		// 檢查Service的selector是否匹配Rollout的labels
		matches := true
		for key, value := range svc.Spec.Selector {
			if rolloutLabels[key] != value {
				matches = false
				break
			}
		}

		if matches {
			ports := make([]map[string]interface{}, 0, len(svc.Spec.Ports))
			for _, port := range svc.Spec.Ports {
				ports = append(ports, map[string]interface{}{
					"name":       port.Name,
					"protocol":   port.Protocol,
					"port":       port.Port,
					"targetPort": port.TargetPort.String(),
					"nodePort":   port.NodePort,
				})
			}

			serviceInfo := map[string]interface{}{
				"name":        svc.Name,
				"namespace":   svc.Namespace,
				"type":        string(svc.Spec.Type),
				"clusterIP":   svc.Spec.ClusterIP,
				"externalIPs": svc.Spec.ExternalIPs,
				"ports":       ports,
				"selector":    svc.Spec.Selector,
				"createdAt":   svc.CreationTimestamp.Time,
			}
			matchedServices = append(matchedServices, serviceInfo)
		}
	}

	response.List(c, matchedServices, int64(len(matchedServices)))
}

// GetRolloutIngresses 獲取Rollout關聯的Ingresses
func (h *RolloutHandler) GetRolloutIngresses(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的Ingresses: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout物件
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 收集Rollout關聯的Service名稱
	relatedServices := make(map[string]bool)

	// 從Canary策略獲取關聯的Service
	if rollout.Spec.Strategy.Canary != nil {
		if rollout.Spec.Strategy.Canary.StableService != "" {
			relatedServices[rollout.Spec.Strategy.Canary.StableService] = true
		}
		if rollout.Spec.Strategy.Canary.CanaryService != "" {
			relatedServices[rollout.Spec.Strategy.Canary.CanaryService] = true
		}
		// 檢查TrafficRouting中的Ingress配置
		if rollout.Spec.Strategy.Canary.TrafficRouting != nil {
			if rollout.Spec.Strategy.Canary.TrafficRouting.Nginx != nil {
				if rollout.Spec.Strategy.Canary.TrafficRouting.Nginx.StableIngress != "" {
					// 記錄Nginx Ingress名稱，後續直接匹配
					relatedServices["__nginx_ingress__:"+rollout.Spec.Strategy.Canary.TrafficRouting.Nginx.StableIngress] = true
				}
			}
			if rollout.Spec.Strategy.Canary.TrafficRouting.ALB != nil {
				if rollout.Spec.Strategy.Canary.TrafficRouting.ALB.Ingress != "" {
					relatedServices["__alb_ingress__:"+rollout.Spec.Strategy.Canary.TrafficRouting.ALB.Ingress] = true
				}
			}
		}
	}

	// 從BlueGreen策略獲取關聯的Service
	if rollout.Spec.Strategy.BlueGreen != nil {
		if rollout.Spec.Strategy.BlueGreen.ActiveService != "" {
			relatedServices[rollout.Spec.Strategy.BlueGreen.ActiveService] = true
		}
		if rollout.Spec.Strategy.BlueGreen.PreviewService != "" {
			relatedServices[rollout.Spec.Strategy.BlueGreen.PreviewService] = true
		}
	}

	// 同時透過Selector匹配獲取關聯的Services
	clientset := k8sClient.GetClientset()
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rolloutLabels := rollout.Spec.Selector.MatchLabels
		for _, svc := range serviceList.Items {
			matches := true
			for key, value := range svc.Spec.Selector {
				if rolloutLabels[key] != value {
					matches = false
					break
				}
			}
			if matches {
				relatedServices[svc.Name] = true
			}
		}
	}

	// 獲取Ingresses
	ingressList, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Ingresses失敗: "+err.Error())
		return
	}

	// 篩選與Rollout關聯的Ingresses
	matchedIngresses := make([]map[string]interface{}, 0)
	for _, ingress := range ingressList.Items {
		isRelated := false

		// 檢查是否是TrafficRouting直接配置的Ingress
		if relatedServices["__nginx_ingress__:"+ingress.Name] || relatedServices["__alb_ingress__:"+ingress.Name] {
			isRelated = true
		}

		// 檢查Ingress的backend是否指向關聯的Service
		if !isRelated {
			for _, rule := range ingress.Spec.Rules {
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						if path.Backend.Service != nil && relatedServices[path.Backend.Service.Name] {
							isRelated = true
							break
						}
					}
				}
				if isRelated {
					break
				}
			}
		}

		// 檢查預設backend
		if !isRelated && ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
			if relatedServices[ingress.Spec.DefaultBackend.Service.Name] {
				isRelated = true
			}
		}

		if isRelated {
			rules := make([]map[string]interface{}, 0, len(ingress.Spec.Rules))
			for _, rule := range ingress.Spec.Rules {
				paths := make([]map[string]interface{}, 0)
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						paths = append(paths, map[string]interface{}{
							"path":     path.Path,
							"pathType": string(*path.PathType),
							"backend": map[string]interface{}{
								"serviceName": path.Backend.Service.Name,
								"servicePort": path.Backend.Service.Port.Number,
							},
						})
					}
				}
				rules = append(rules, map[string]interface{}{
					"host":  rule.Host,
					"paths": paths,
				})
			}

			ingressInfo := map[string]interface{}{
				"name":             ingress.Name,
				"namespace":        ingress.Namespace,
				"ingressClassName": ingress.Spec.IngressClassName,
				"rules":            rules,
				"createdAt":        ingress.CreationTimestamp.Time,
			}
			matchedIngresses = append(matchedIngresses, ingressInfo)
		}
	}

	response.List(c, matchedIngresses, int64(len(matchedIngresses)))
}

// GetRolloutHPA 獲取Rollout關聯的HPA
func (h *RolloutHandler) GetRolloutHPA(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的HPA: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	hpaList, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取HPA失敗: "+err.Error())
		return
	}

	// 查詢與Rollout關聯的HPA
	var targetHPA interface{}
	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Rollout" && hpa.Spec.ScaleTargetRef.Name == name {
			targetHPA = hpa
			break
		}
	}

	response.OK(c, targetHPA)
}

// GetRolloutReplicaSets 獲取Rollout關聯的ReplicaSets
func (h *RolloutHandler) GetRolloutReplicaSets(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的ReplicaSets: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	replicaSets, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取ReplicaSets失敗: "+err.Error())
		return
	}

	// 篩選由Rollout管理的ReplicaSets
	var relatedReplicaSets []interface{}
	for _, rs := range replicaSets.Items {
		for _, ownerRef := range rs.OwnerReferences {
			if ownerRef.Kind == "Rollout" && ownerRef.Name == name {
				relatedReplicaSets = append(relatedReplicaSets, rs)
				break
			}
		}
	}

	response.OK(c, relatedReplicaSets)
}

// GetRolloutEvents 獲取Rollout相關的Events
func (h *RolloutHandler) GetRolloutEvents(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout相關的Events: %s/%s/%s", clusterId, namespace, name)

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Rollout", name),
	})
	if err != nil {
		response.InternalError(c, "獲取Events失敗: "+err.Error())
		return
	}

	response.OK(c, events)
}

// PromoteRollout 推進 Rollout 一個步驟（解除 pause）
func (h *RolloutHandler) PromoteRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	patch := []byte(`{"spec":{"paused":false}}`)
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		response.InternalError(c, "Promote 失敗: "+err.Error())
		return
	}
	logger.Info("Promote Rollout", "cluster", clusterId, "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "Promote 成功"})
}

// PromoteFullRollout 全量推進 Rollout（跳過所有 pause 和 analysis）
func (h *RolloutHandler) PromoteFullRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 先取得現有 Rollout，設定 status.promoteFull = true 並透過 UpdateStatus 更新
	existing, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "取得 Rollout 失敗: "+err.Error())
		return
	}
	existing.Status.PromoteFull = true
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).UpdateStatus(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "Promote Full 失敗: "+err.Error())
		return
	}
	logger.Info("PromoteFull Rollout", "cluster", clusterId, "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "Promote Full 成功"})
}

// AbortRollout 中止 Rollout
func (h *RolloutHandler) AbortRollout(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	existing, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "取得 Rollout 失敗: "+err.Error())
		return
	}
	existing.Status.Abort = true
	_, err = rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).UpdateStatus(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "Abort 失敗: "+err.Error())
		return
	}
	logger.Info("Abort Rollout", "cluster", clusterId, "namespace", namespace, "name", name)
	response.OK(c, gin.H{"message": "Abort 成功"})
}

// GetRolloutAnalysisRuns 取得 Rollout 相關的 AnalysisRun 列表
func (h *RolloutHandler) GetRolloutAnalysisRuns(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// List AnalysisRuns 以 rollout owner label 過濾
	labelSelector := fmt.Sprintf("rollouts-pod-template-hash,rollout-type")
	_ = labelSelector // 改用 List all + filter by owner ref
	allRuns, err := rolloutClient.ArgoprojV1alpha1().AnalysisRuns(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 AnalysisRun 失敗: "+err.Error())
		return
	}

	// 篩選屬於此 Rollout 的 AnalysisRun（透過 ownerReferences）
	result := make([]map[string]interface{}, 0)
	for _, ar := range allRuns.Items {
		for _, ref := range ar.OwnerReferences {
			if ref.Kind == "Rollout" && ref.Name == name {
				result = append(result, map[string]interface{}{
					"name":      ar.Name,
					"namespace": ar.Namespace,
					"phase":     string(ar.Status.Phase),
					"message":   ar.Status.Message,
					"startedAt": ar.CreationTimestamp.Time,
				})
				break
			}
		}
	}

	response.OK(c, gin.H{"items": result, "total": len(result)})
}
