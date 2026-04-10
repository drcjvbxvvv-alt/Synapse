package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
)

// GetContainerLogs 獲取容器日誌
func (h *LogCenterHandler) GetContainerLogs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Query("namespace")
	podName := c.Query("pod")
	container := c.Query("container")
	tailLines, _ := strconv.ParseInt(c.DefaultQuery("tailLines", "100"), 10, 64)
	sinceSeconds, _ := strconv.ParseInt(c.DefaultQuery("sinceSeconds", "0"), 10, 64)
	previous := c.Query("previous") == "true"

	if namespace == "" || podName == "" {
		response.BadRequest(c, "namespace 和 pod 參數必填")
		return
	}

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logs, err := h.aggregator.GetContainerLogs(ctx, cluster, namespace, podName, container, tailLines, sinceSeconds, previous)
	if err != nil {
		response.InternalError(c, "獲取日誌失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"logs": logs,
	})
}

// GetEventLogs 獲取K8s事件日誌
func (h *LogCenterHandler) GetEventLogs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Query("namespace")
	resourceType := c.Query("resourceType")
	resourceName := c.Query("resourceName")
	eventType := c.Query("type") // Normal, Warning
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	cluster, err := h.clusterSvc.GetCluster(clusterID)
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

	listOpts := metav1.ListOptions{
		Limit: int64(limit),
	}

	// 構建欄位選擇器
	var fieldSelectors []string
	if resourceType != "" {
		fieldSelectors = append(fieldSelectors, fmt.Sprintf("involvedObject.kind=%s", resourceType))
	}
	if resourceName != "" {
		fieldSelectors = append(fieldSelectors, fmt.Sprintf("involvedObject.name=%s", resourceName))
	}
	if eventType != "" {
		fieldSelectors = append(fieldSelectors, fmt.Sprintf("type=%s", eventType))
	}
	if len(fieldSelectors) > 0 {
		listOpts.FieldSelector = strings.Join(fieldSelectors, ",")
	}

	events, err := k8sClient.GetClientset().CoreV1().Events(namespace).List(ctx, listOpts)
	if err != nil {
		response.InternalError(c, "獲取事件失敗: "+err.Error())
		return
	}

	// 轉換為統一格式
	eventLogs := make([]models.EventLogEntry, 0, len(events.Items))
	for _, e := range events.Items {
		eventLogs = append(eventLogs, models.EventLogEntry{
			ID:              string(e.UID),
			Type:            e.Type,
			Reason:          e.Reason,
			Message:         e.Message,
			Count:           e.Count,
			FirstTimestamp:  e.FirstTimestamp.Time,
			LastTimestamp:   e.LastTimestamp.Time,
			Namespace:       e.Namespace,
			InvolvedKind:    e.InvolvedObject.Kind,
			InvolvedName:    e.InvolvedObject.Name,
			SourceComponent: e.Source.Component,
			SourceHost:      e.Source.Host,
		})
	}

	// 按時間排序（最新的在前）
	sort.Slice(eventLogs, func(i, j int) bool {
		return eventLogs[i].LastTimestamp.After(eventLogs[j].LastTimestamp)
	})

	response.List(c, eventLogs, int64(len(eventLogs)))
}

// SearchLogs 日誌搜尋
func (h *LogCenterHandler) SearchLogs(c *gin.Context) {
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	results, total, err := h.aggregator.SearchLogs(ctx, cluster, &query)
	if err != nil {
		response.InternalError(c, "搜尋失敗: "+err.Error())
		return
	}

	response.List(c, results, int64(total))
}

// GetLogStats 獲取日誌統計
func (h *LogCenterHandler) GetLogStats(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Query("namespace")
	timeRange := c.DefaultQuery("timeRange", "1h") // 1h, 6h, 24h, 7d

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 計算時間範圍
	var since time.Duration
	switch timeRange {
	case "1h":
		since = time.Hour
	case "6h":
		since = 6 * time.Hour
	case "24h":
		since = 24 * time.Hour
	case "7d":
		since = 7 * 24 * time.Hour
	default:
		since = time.Hour
	}
	startTime := time.Now().Add(-since)

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取事件統計
	events, err := k8sClient.GetClientset().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取事件失敗: "+err.Error())
		return
	}

	stats := models.LogStats{}
	levelCount := make(map[string]int64)
	nsCount := make(map[string]int64)

	for _, e := range events.Items {
		// 過濾時間範圍
		if e.LastTimestamp.Time.Before(startTime) {
			continue
		}

		stats.TotalCount++

		if e.Type == "Warning" {
			stats.WarnCount++
			levelCount["warn"]++
		} else {
			stats.InfoCount++
			levelCount["info"]++
		}

		// 檢查是否包含錯誤關鍵詞
		lowerMsg := strings.ToLower(e.Message)
		if strings.Contains(lowerMsg, "error") ||
			strings.Contains(lowerMsg, "fail") ||
			strings.Contains(lowerMsg, "crash") {
			stats.ErrorCount++
			levelCount["error"]++
		}

		nsCount[e.Namespace]++
	}

	// 轉換為統計陣列
	for level, count := range levelCount {
		stats.LevelStats = append(stats.LevelStats, models.LevelStat{Level: level, Count: count})
	}
	for ns, count := range nsCount {
		stats.NamespaceStats = append(stats.NamespaceStats, models.NamespaceStat{Namespace: ns, Count: count})
	}

	// 按數量排序命名空間統計
	sort.Slice(stats.NamespaceStats, func(i, j int) bool {
		return stats.NamespaceStats[i].Count > stats.NamespaceStats[j].Count
	})

	response.OK(c, stats)
}
