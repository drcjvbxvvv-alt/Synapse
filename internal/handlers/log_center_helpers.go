package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
)

// GetNamespacesForLogs 獲取日誌中心可用的命名空間列表
func (h *LogCenterHandler) GetNamespacesForLogs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(clusterID)
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

	// 使用 informer 獲取命名空間列表（透過獲取所有 Pod 的命名空間）
	sel := labels.Everything()
	pods, err := h.k8sMgr.PodsLister(cluster.ID).List(sel)
	if err != nil {
		response.InternalError(c, "獲取命名空間失敗: "+err.Error())
		return
	}

	// 收集所有有 Pod 的命名空間
	nsSet := make(map[string]bool)
	for _, pod := range pods {
		nsSet[pod.Namespace] = true
	}

	nsList := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		nsList = append(nsList, ns)
	}
	sort.Strings(nsList)

	response.OK(c, nsList)
}

// GetPodsForLogs 獲取指定命名空間的Pod列表（用於日誌選擇）
func (h *LogCenterHandler) GetPodsForLogs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Query("namespace")

	cluster, err := h.clusterSvc.GetCluster(clusterID)
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

	// 使用 informer 獲取 Pod 列表
	sel := labels.Everything()
	var podObjs []*corev1.Pod

	if namespace != "" {
		podObjs, err = h.k8sMgr.PodsLister(cluster.ID).Pods(namespace).List(sel)
	} else {
		podObjs, err = h.k8sMgr.PodsLister(cluster.ID).List(sel)
	}

	if err != nil {
		response.InternalError(c, "獲取Pod列表失敗: "+err.Error())
		return
	}

	type PodInfo struct {
		Name       string   `json:"name"`
		Namespace  string   `json:"namespace"`
		Status     string   `json:"status"`
		Containers []string `json:"containers"`
	}

	podList := make([]PodInfo, 0, len(podObjs))
	for _, pod := range podObjs {
		containers := make([]string, 0, len(pod.Spec.Containers))
		for _, c := range pod.Spec.Containers {
			containers = append(containers, c.Name)
		}
		podList = append(podList, PodInfo{
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			Status:     string(pod.Status.Phase),
			Containers: containers,
		})
	}

	// 按名稱排序
	sort.Slice(podList, func(i, j int) bool {
		return podList[i].Name < podList[j].Name
	})

	response.OK(c, podList)
}

// ExportLogs 匯出日誌
func (h *LogCenterHandler) ExportLogs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var query models.LogQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// 設定較大的限制用於匯出
	if query.Limit <= 0 || query.Limit > 10000 {
		query.Limit = 10000
	}

	results, _, err := h.aggregator.SearchLogs(ctx, cluster, &query)
	if err != nil {
		response.InternalError(c, "獲取日誌失敗: "+err.Error())
		return
	}

	// 構建匯出內容
	var builder strings.Builder
	for _, entry := range results {
		builder.WriteString(fmt.Sprintf("%s [%s] [%s/%s] %s\n",
			entry.Timestamp.Format(time.RFC3339),
			strings.ToUpper(entry.Level),
			entry.Namespace,
			entry.PodName,
			entry.Message,
		))
	}

	// 設定響應頭
	filename := fmt.Sprintf("logs-%s-%s.txt", cluster.Name, time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, builder.String())
}
