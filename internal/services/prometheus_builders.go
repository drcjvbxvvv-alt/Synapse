package services

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
)

// ── Label-selector & URL builders ──────────────────────────────────────────
// Methods on *PrometheusService that build query URLs, selectors, and handle auth.
// Extracted from prometheus_service.go to reduce file size.

func (s *PrometheusService) buildQueryURL(endpoint string, query *models.MetricsQuery) (*url.URL, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	// 設定查詢路徑
	baseURL.Path = "/api/v1/query_range"

	// 設定查詢參數
	params := url.Values{}
	params.Set("query", query.Query)
	params.Set("start", strconv.FormatInt(query.Start, 10))
	params.Set("end", strconv.FormatInt(query.End, 10))
	params.Set("step", query.Step)

	if query.Timeout != "" {
		params.Set("timeout", query.Timeout)
	}

	baseURL.RawQuery = params.Encode()
	return baseURL, nil
}

// setAuth 設定認證
func (s *PrometheusService) setAuth(req *http.Request, auth *models.MonitoringAuth) error {
	return SetMonitoringAuth(req, auth)
}

// parseTimeRange 解析時間範圍
func (s *PrometheusService) parseTimeRange(timeRange string) (int64, int64, error) {
	now := time.Now()
	var duration time.Duration
	var err error

	switch timeRange {
	case "1h":
		duration = time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "24h", "1d":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		duration, err = time.ParseDuration(timeRange)
		if err != nil {
			return 0, 0, fmt.Errorf("無效的時間範圍: %s", timeRange)
		}
	}

	end := now.Unix()
	start := now.Add(-duration).Unix()
	return start, end, nil
}

// buildClusterSelector 構建叢集標籤選擇器
//
//nolint:unused // 保留用於未來使用
func (s *PrometheusService) buildClusterSelector(labels map[string]string, clusterName string) string {
	selectors := []string{}

	// 新增叢集標籤
	if clusterName != "" {
		selectors = append(selectors, fmt.Sprintf("cluster=\"%s\"", clusterName))
	}

	// 新增自定義標籤
	for key, value := range labels {
		selectors = append(selectors, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return strings.Join(selectors, ",")
}

// buildNodeSelector 構建節點標籤選擇器
func (s *PrometheusService) buildNodeSelector(labels map[string]string, clusterName, nodeName string) string {
	selectors := []string{}

	// 新增叢集標籤
	if clusterName != "" {
		selectors = append(selectors, fmt.Sprintf("cluster=\"%s\"", clusterName))
	}

	// 新增節點標籤
	if nodeName != "" {
		selectors = append(selectors, fmt.Sprintf("instance=~\".*%s.*\"", nodeName))
	}

	// 新增自定義標籤
	for key, value := range labels {
		selectors = append(selectors, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return strings.Join(selectors, ",")
}

// buildPodSelector 構建 Pod 標籤選擇器
func (s *PrometheusService) buildPodSelector(labels map[string]string, clusterName, namespace, podName string) string {
	selectors := []string{}

	// 新增叢集標籤
	if clusterName != "" {
		selectors = append(selectors, fmt.Sprintf("cluster=\"%s\"", clusterName))
	}

	// 新增命名空間標籤
	if namespace != "" {
		selectors = append(selectors, fmt.Sprintf("namespace=\"%s\"", namespace))
	}

	// 新增 Pod 標籤
	if podName != "" {
		selectors = append(selectors, fmt.Sprintf("pod=\"%s\"", podName))
	}

	// 新增自定義標籤
	for key, value := range labels {
		selectors = append(selectors, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return strings.Join(selectors, ",")
}

// buildWorkloadSelector 構建工作負載標籤選擇器（使用正規表示式匹配pod名稱）
func (s *PrometheusService) buildWorkloadSelector(labels map[string]string, clusterName, namespace, workloadName string) string {
	selectors := []string{}

	// 新增叢集標籤
	if clusterName != "" {
		selectors = append(selectors, fmt.Sprintf("cluster=\"%s\"", clusterName))
	}

	// 新增命名空間標籤
	if namespace != "" {
		selectors = append(selectors, fmt.Sprintf("namespace=\"%s\"", namespace))
	}

	// 使用正規表示式匹配工作負載的Pod名稱
	// Deployment: deployment-name-xxx-xxx
	// StatefulSet: statefulset-name-0, statefulset-name-1, ...
	// DaemonSet: daemonset-name-xxx
	// ReplicaSet: replicaset-name-xxx
	if workloadName != "" {
		selectors = append(selectors, fmt.Sprintf("pod=~\"%s-.*\"", workloadName))
	}

	// 新增自定義標籤
	for key, value := range labels {
		selectors = append(selectors, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return strings.Join(selectors, ",")
}

