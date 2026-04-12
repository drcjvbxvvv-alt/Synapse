package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ── Resource Top & Control Plane ───────────────────────────────────────────
// Methods on *OMService for resource usage ranking and control plane health.
// Extracted from om_service.go to reduce file size.

func (s *OMService) GetResourceTop(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint, req *models.ResourceTopRequest) (*models.ResourceTopResponse, error) {
	response := &models.ResourceTopResponse{
		Type:      req.Type,
		Level:     req.Level,
		Items:     []models.ResourceTopItem{},
		QueryTime: time.Now().Unix(),
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	// 獲取監控配置
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err != nil || config.Type == "disabled" {
		// 沒有監控資料，從 K8s 獲取基本資訊
		return s.getResourceTopFromK8s(ctx, clientset, req, limit)
	}

	now := time.Now().Unix()

	// 根據資源型別和級別構建查詢
	var query string
	var unit string

	switch req.Type {
	case "cpu":
		unit = "cores"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])) by (namespace))"
		case "workload":
			query = "topk(100, sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])) by (namespace, pod))"
		}
	case "memory":
		unit = "bytes"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}) by (namespace))"
		case "workload":
			query = "topk(100, sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		}
	case "network":
		unit = "bytes/s"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(rate(container_network_receive_bytes_total[5m]) + rate(container_network_transmit_bytes_total[5m])) by (namespace))"
		case "workload":
			query = "topk(100, sum(rate(container_network_receive_bytes_total[5m]) + rate(container_network_transmit_bytes_total[5m])) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(rate(container_network_receive_bytes_total[5m]) + rate(container_network_transmit_bytes_total[5m])) by (namespace, pod))"
		}
	case "disk":
		unit = "bytes"
		switch req.Level {
		case "namespace":
			query = "topk(100, sum(container_fs_usage_bytes{container!=\"\",container!=\"POD\"}) by (namespace))"
		case "workload":
			query = "topk(100, sum(container_fs_usage_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		case "pod":
			query = "topk(100, sum(container_fs_usage_bytes{container!=\"\",container!=\"POD\"}) by (namespace, pod))"
		}
	}

	resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
		Query: query,
		Start: now,
		End:   now,
		Step:  "1m",
	})
	if err != nil {
		logger.Error("查詢資源 Top N 失敗", "error", err)
		return response, nil
	}

	// 解析結果
	type resultItem struct {
		name      string
		namespace string
		usage     float64
	}
	var items []resultItem

	for _, result := range resp.Data.Result {
		if len(result.Values) == 0 {
			continue
		}

		name := ""
		namespace := ""

		if ns, ok := result.Metric["namespace"]; ok {
			namespace = ns
		}
		if pod, ok := result.Metric["pod"]; ok {
			name = pod
		} else if namespace != "" {
			name = namespace
		}

		if name == "" {
			continue
		}

		val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64)
		if err != nil {
			continue
		}

		items = append(items, resultItem{
			name:      name,
			namespace: namespace,
			usage:     val,
		})
	}

	// 按使用量排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].usage > items[j].usage
	})

	// 取 Top N
	for i := 0; i < len(items) && i < limit; i++ {
		item := items[i]
		topItem := models.ResourceTopItem{
			Rank:      i + 1,
			Name:      item.name,
			Namespace: item.namespace,
			Usage:     item.usage,
			Unit:      unit,
		}

		// 計算使用率（如果有 limit 資料）
		// 這裡簡化處理，可以後續擴充套件查詢 limit 資料
		response.Items = append(response.Items, topItem)
	}

	return response, nil
}

// getResourceTopFromK8s 從 K8s 獲取資源 Top N（無監控資料時）
func (s *OMService) getResourceTopFromK8s(ctx context.Context, clientset *kubernetes.Clientset, req *models.ResourceTopRequest, limit int) (*models.ResourceTopResponse, error) {
	response := &models.ResourceTopResponse{
		Type:      req.Type,
		Level:     req.Level,
		Items:     []models.ResourceTopItem{},
		QueryTime: time.Now().Unix(),
	}

	// 獲取 Pod 列表
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return response, err
	}

	type usageData struct {
		name      string
		namespace string
		request   int64
		limit     int64
	}

	var items []usageData

	switch req.Type {
	case "cpu", "memory":
		resourceName := corev1.ResourceCPU
		if req.Type == "memory" {
			resourceName = corev1.ResourceMemory
		}

		switch req.Level {
		case "namespace":
			nsUsage := make(map[string]*usageData)
			for _, pod := range pods.Items {
				if _, ok := nsUsage[pod.Namespace]; !ok {
					nsUsage[pod.Namespace] = &usageData{
						name:      pod.Namespace,
						namespace: pod.Namespace,
					}
				}
				for _, container := range pod.Spec.Containers {
					if req := container.Resources.Requests[resourceName]; !req.IsZero() {
						nsUsage[pod.Namespace].request += req.Value()
					}
					if lim := container.Resources.Limits[resourceName]; !lim.IsZero() {
						nsUsage[pod.Namespace].limit += lim.Value()
					}
				}
			}
			for _, v := range nsUsage {
				items = append(items, *v)
			}

		case "pod":
			for _, pod := range pods.Items {
				item := usageData{
					name:      pod.Name,
					namespace: pod.Namespace,
				}
				for _, container := range pod.Spec.Containers {
					if req := container.Resources.Requests[resourceName]; !req.IsZero() {
						item.request += req.Value()
					}
					if lim := container.Resources.Limits[resourceName]; !lim.IsZero() {
						item.limit += lim.Value()
					}
				}
				if item.request > 0 || item.limit > 0 {
					items = append(items, item)
				}
			}
		}
	}

	// 按 request 值排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].request > items[j].request
	})

	// 取 Top N
	unit := "cores"
	if req.Type == "memory" {
		unit = "bytes"
	}

	for i := 0; i < len(items) && i < limit; i++ {
		item := items[i]
		topItem := models.ResourceTopItem{
			Rank:      i + 1,
			Name:      item.name,
			Namespace: item.namespace,
			Request:   float64(item.request),
			Limit:     float64(item.limit),
			Usage:     float64(item.request), // 無監控資料時用 request 代替
			Unit:      unit,
		}
		response.Items = append(response.Items, topItem)
	}

	return response, nil
}

// GetControlPlaneStatus 獲取控制面元件狀態
func (s *OMService) GetControlPlaneStatus(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.ControlPlaneStatusResponse, error) {
	response := &models.ControlPlaneStatusResponse{
		Overall:    "healthy",
		Components: []models.ControlPlaneComponent{},
		CheckTime:  time.Now().Unix(),
	}

	// 獲取 kube-system 命名空間下的 Pod
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取 kube-system Pod 列表失敗", "error", err)
		return response, nil
	}

	// 定義要檢查的控制面元件
	componentTypes := []string{"kube-apiserver", "kube-scheduler", "kube-controller-manager", "etcd"}

	componentsMap := make(map[string]*models.ControlPlaneComponent)

	for _, componentType := range componentTypes {
		componentsMap[componentType] = &models.ControlPlaneComponent{
			Name:          componentType,
			Type:          strings.TrimPrefix(componentType, "kube-"),
			Status:        "unknown",
			Message:       "未檢測到該元件",
			LastCheckTime: time.Now().Unix(),
			Instances:     []models.ComponentInstance{},
		}
	}

	// 遍歷 Pod，匹配控制面元件
	for _, pod := range pods.Items {
		for _, componentType := range componentTypes {
			if strings.Contains(pod.Name, componentType) {
				component := componentsMap[componentType]

				instance := models.ComponentInstance{
					Name:   pod.Name,
					Node:   pod.Spec.NodeName,
					Status: string(pod.Status.Phase),
					IP:     pod.Status.PodIP,
				}
				if pod.Status.StartTime != nil {
					instance.StartTime = pod.Status.StartTime.Unix()
				}
				component.Instances = append(component.Instances, instance)

				// 更新元件整體狀態
				if pod.Status.Phase == corev1.PodRunning {
					allReady := true
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
							allReady = false
							break
						}
					}
					if allReady {
						component.Status = "healthy"
						component.Message = "元件執行正常"
					} else {
						component.Status = "unhealthy"
						component.Message = "元件 Pod 未就緒"
					}
				} else {
					component.Status = "unhealthy"
					component.Message = fmt.Sprintf("元件 Pod 狀態: %s", pod.Status.Phase)
				}
				break
			}
		}
	}

	// 獲取監控配置，查詢元件指標
	config, err := s.monitoringConfigSvc.GetMonitoringConfig(clusterID)
	if err == nil && config.Type != "disabled" {
		s.enrichComponentMetrics(ctx, config, componentsMap)
	}

	// 組裝響應
	unhealthyCount := 0
	for _, component := range componentsMap {
		if component.Status == "unhealthy" {
			unhealthyCount++
		}
		response.Components = append(response.Components, *component)
	}

	// 確定整體狀態
	if unhealthyCount > 0 {
		if unhealthyCount >= len(componentTypes)/2 {
			response.Overall = "unhealthy"
		} else {
			response.Overall = "degraded"
		}
	}

	return response, nil
}

// enrichComponentMetrics 從 Prometheus 獲取元件指標
func (s *OMService) enrichComponentMetrics(ctx context.Context, config *models.MonitoringConfig, componentsMap map[string]*models.ControlPlaneComponent) {
	now := time.Now().Unix()

	// API Server 指標
	if apiserver, ok := componentsMap["kube-apiserver"]; ok {
		apiserver.Metrics = &models.ComponentMetrics{}

		// 請求速率
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(rate(apiserver_request_total[5m]))",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.RequestRate = val
			}
		}

		// 錯誤率
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(rate(apiserver_request_total{code=~\"5..\"}[5m])) / sum(rate(apiserver_request_total[5m])) * 100",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.ErrorRate = val
			}
		}

		// 延遲
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket[5m])) by (le)) * 1000",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				apiserver.Metrics.Latency = val
			}
		}
	}

	// Etcd 指標
	if etcd, ok := componentsMap["etcd"]; ok {
		etcd.Metrics = &models.ComponentMetrics{}

		// Leader 狀態
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "max(etcd_server_has_leader)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.LeaderStatus = val == 1
			}
		}

		// 資料庫大小
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(etcd_mvcc_db_total_size_in_bytes)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.DBSize = val
			}
		}

		// 成員數量
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "count(etcd_server_has_leader)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				etcd.Metrics.MemberCount = int(val)
			}
		}
	}

	// Scheduler 指標
	if scheduler, ok := componentsMap["kube-scheduler"]; ok {
		scheduler.Metrics = &models.ComponentMetrics{}

		// 佇列長度
		if resp, err := s.prometheusSvc.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: "sum(scheduler_pending_pods)",
			Start: now, End: now, Step: "1m",
		}); err == nil && len(resp.Data.Result) > 0 && len(resp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", resp.Data.Result[0].Values[0][1]), 64); err == nil {
				scheduler.Metrics.QueueLength = int(val)
			}
		}
	}
}

// min 返回兩個整數中的較小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
