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

	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	sigsyaml "sigs.k8s.io/yaml"
)

type CronJobHandler struct {
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

func NewCronJobHandler(cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *CronJobHandler {
	return &CronJobHandler{cfg: cfg, clusterService: clusterService, k8sMgr: k8sMgr}
}

type CronJobInfo struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Type             string            `json:"type"`
	Status           string            `json:"status"`
	Schedule         string            `json:"schedule"`
	Suspend          bool              `json:"suspend"`
	Active           int               `json:"active"`
	LastScheduleTime *time.Time        `json:"lastScheduleTime"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	CreatedAt        time.Time         `json:"createdAt"`
}

func (h *CronJobHandler) ListCronJobs(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Query("namespace")
	searchName := c.Query("search")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	logger.Info("獲取CronJob列表: cluster=%s, namespace=%s, search=%s", clusterId, namespace, searchName)

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

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	var cronJobList *batchv1.CronJobList
	if namespace != "" {
		cronJobList, err = clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	} else {
		cronJobList, err = clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		response.InternalError(c, "獲取CronJob列表失敗: "+err.Error())
		return
	}

	var cronJobs []CronJobInfo
	for _, cj := range cronJobList.Items {
		cronJobs = append(cronJobs, h.convertToCronJobInfo(&cj))
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		cronJobs = middleware.FilterResourcesByNamespace(c, cronJobs, func(cj CronJobInfo) string {
			return cj.Namespace
		})
	}

	if searchName != "" {
		var filtered []CronJobInfo
		searchLower := strings.ToLower(searchName)
		for _, cj := range cronJobs {
			if strings.Contains(strings.ToLower(cj.Name), searchLower) {
				filtered = append(filtered, cj)
			}
		}
		cronJobs = filtered
	}

	sort.Slice(cronJobs, func(i, j int) bool {
		return cronJobs[i].CreatedAt.After(cronJobs[j].CreatedAt)
	})

	total := len(cronJobs)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedCronJobs := cronJobs[start:end]

	response.PagedList(c, pagedCronJobs, int64(total), page, pageSize)
}

func (h *CronJobHandler) GetCronJob(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取CronJob詳情: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	cronJob, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "CronJob不存在: "+err.Error())
		return
	}

	// 獲取關聯的Jobs
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取CronJob關聯Jobs失敗", "error", err)
	}

	// 清理 managed fields 以生成更乾淨的 YAML
	cleanCronJob := cronJob.DeepCopy()
	cleanCronJob.ManagedFields = nil
	cleanCronJob.Annotations = filterAnnotations(cleanCronJob.Annotations)
	// 設定 TypeMeta（client-go 返回的物件預設不包含 apiVersion 和 kind）
	cleanCronJob.APIVersion = "batch/v1"
	cleanCronJob.Kind = "CronJob"
	// 將 CronJob 物件轉換為 YAML 字串
	yamlBytes, yamlErr := sigsyaml.Marshal(cleanCronJob)
	var yamlString string
	if yamlErr == nil {
		yamlString = string(yamlBytes)
	} else {
		logger.Error("轉換CronJob為YAML失敗", "error", yamlErr)
		yamlString = ""
	}

	response.OK(c, gin.H{
		"workload": h.convertToCronJobInfo(cronJob),
		"raw":      cronJob,
		"yaml":     yamlString,
		"jobs":     jobs,
	})
}

func (h *CronJobHandler) GetCronJobNamespaces(c *gin.Context) {
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

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	cronJobList, err := clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取CronJob列表失敗: "+err.Error())
		return
	}

	nsCount := make(map[string]int)
	for _, cj := range cronJobList.Items {
		nsCount[cj.Namespace]++
	}

	type NamespaceInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceInfo
	for ns, count := range nsCount {
		namespaces = append(namespaces, NamespaceInfo{Name: ns, Count: count})
	}

	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	response.OK(c, namespaces)
}

func (h *CronJobHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")
	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("應用CronJob YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var objMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &objMap); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if objMap["apiVersion"] == nil || objMap["kind"] == nil {
		response.BadRequest(c, "YAML缺少必要欄位: apiVersion 或 kind")
		return
	}

	kind := objMap["kind"].(string)
	if kind != "CronJob" {
		response.BadRequest(c, "YAML型別錯誤，期望CronJob，實際為: "+kind)
		return
	}

	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		response.BadRequest(c, "YAML缺少 metadata 欄位")
		return
	}

	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	result, err := h.applyYAML(ctx, k8sClient, req.YAML, namespace, req.DryRun)
	if err != nil {
		response.InternalError(c, "YAML應用失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}

func (h *CronJobHandler) DeleteCronJob(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("刪除CronJob: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	err = clientset.BatchV1().CronJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}

func (h *CronJobHandler) convertToCronJobInfo(cj *batchv1.CronJob) CronJobInfo {
	status := "Active"
	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		status = "Suspended"
	}

	suspend := false
	if cj.Spec.Suspend != nil {
		suspend = *cj.Spec.Suspend
	}

	var lastScheduleTime *time.Time
	if cj.Status.LastScheduleTime != nil {
		t := cj.Status.LastScheduleTime.Time
		lastScheduleTime = &t
	}

	return CronJobInfo{
		ID:               fmt.Sprintf("%s/%s", cj.Namespace, cj.Name),
		Name:             cj.Name,
		Namespace:        cj.Namespace,
		Type:             "CronJob",
		Status:           status,
		Schedule:         cj.Spec.Schedule,
		Suspend:          suspend,
		Active:           len(cj.Status.Active),
		LastScheduleTime: lastScheduleTime,
		Labels:           cj.Labels,
		Annotations:      cj.Annotations,
		CreatedAt:        cj.CreationTimestamp.Time,
	}
}

func (h *CronJobHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	cronJob, ok := obj.(*batchv1.CronJob)
	if !ok {
		return nil, fmt.Errorf("無法轉換為CronJob型別")
	}

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.BatchV1().CronJobs(cronJob.Namespace).Get(ctx, cronJob.Name, metav1.GetOptions{})
	if err == nil {
		cronJob.ResourceVersion = existing.ResourceVersion
		result, err := clientset.BatchV1().CronJobs(cronJob.Namespace).Update(ctx, cronJob, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	result, err := clientset.BatchV1().CronJobs(cronJob.Namespace).Create(ctx, cronJob, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}
