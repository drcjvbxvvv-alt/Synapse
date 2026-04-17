package services

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// QueryClusterMetrics 查詢叢集監控指標（使用併發查詢最佳化效能）
func (s *PrometheusService) QueryClusterMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName string, timeRange string, step string) (*models.ClusterMetricsData, error) {
	// 解析時間範圍
	start, end, err := s.parseTimeRange(timeRange)
	if err != nil {
		return nil, fmt.Errorf("解析時間範圍失敗: %w", err)
	}

	metrics := &models.ClusterMetricsData{}

	// 構建叢集標籤選擇器
	// 如果是 prometheus，標籤不用過來
	clusterSelector := ""

	// 使用 WaitGroup 和 Mutex 進行併發查詢
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 併發查詢 CPU 使用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cpuSeries, err := s.queryMetricSeries(ctx, config, "(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[1m]))) * 100", start, end, step); err == nil {
			mu.Lock()
			metrics.CPU = cpuSeries
			mu.Unlock()
		}
	}()

	// 併發查詢記憶體使用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		if memorySeries, err := s.queryMetricSeries(ctx, config, "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100", start, end, step); err == nil {
			mu.Lock()
			metrics.Memory = memorySeries
			mu.Unlock()
		}
	}()

	// 併發查詢網路指標
	wg.Add(1)
	go func() {
		defer wg.Done()
		if networkMetrics, err := s.queryNetworkMetrics(ctx, config, clusterSelector, start, end, step); err == nil {
			mu.Lock()
			metrics.Network = networkMetrics
			mu.Unlock()
		}
	}()

	// 併發查詢 Pod 指標
	wg.Add(1)
	go func() {
		defer wg.Done()
		if podMetrics, err := s.queryPodMetrics(ctx, config, clusterSelector); err == nil {
			mu.Lock()
			metrics.Pods = podMetrics
			mu.Unlock()
		}
	}()

	// 併發查詢叢集概覽指標
	wg.Add(1)
	go func() {
		defer wg.Done()
		if clusterOverview, err := s.queryClusterOverview(ctx, config, clusterName, start, end, step); err == nil {
			mu.Lock()
			metrics.ClusterOverview = clusterOverview
			mu.Unlock()
		}
	}()

	// 併發查詢節點列表指標
	wg.Add(1)
	go func() {
		defer wg.Done()
		if nodeList, err := s.QueryNodeListMetrics(ctx, config, clusterName); err == nil {
			mu.Lock()
			metrics.NodeList = nodeList
			mu.Unlock()
		}
	}()

	// 等待所有查詢完成
	wg.Wait()

	return metrics, nil
}

// QueryNodeMetrics 查詢節點監控指標
func (s *PrometheusService) QueryNodeMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, nodeName string, timeRange string, step string) (*models.ClusterMetricsData, error) {
	// 解析時間範圍
	start, end, err := s.parseTimeRange(timeRange)
	if err != nil {
		return nil, fmt.Errorf("解析時間範圍失敗: %w", err)
	}

	metrics := &models.ClusterMetricsData{}

	// 構建節點標籤選擇器
	nodeSelector := s.buildNodeSelector(config.Labels, clusterName, nodeName)

	// 查詢節點 CPU 使用率
	if cpuSeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("rate(node_cpu_seconds_total{mode!=\"idle\",%s}[5m])", nodeSelector), start, end, step); err == nil {
		metrics.CPU = cpuSeries
	}

	// 查詢節點記憶體使用率
	if memorySeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("(1 - (node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes{%s}))", nodeSelector, nodeSelector), start, end, step); err == nil {
		metrics.Memory = memorySeries
	}

	// 查詢節點網路指標
	if networkMetrics, err := s.queryNodeNetworkMetrics(ctx, config, nodeSelector, start, end, step); err == nil {
		metrics.Network = networkMetrics
	}

	// 查詢節點儲存指標
	if storageSeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("(1 - (node_filesystem_avail_bytes{%s} / node_filesystem_size_bytes{%s}))", nodeSelector, nodeSelector), start, end, step); err == nil {
		metrics.Storage = storageSeries
	}

	return metrics, nil
}

// QueryNodeListMetrics 查詢節點列表監控指標（使用併發查詢最佳化效能）
func (s *PrometheusService) QueryNodeListMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName string) ([]models.NodeMetricItem, error) {
	nodeList := []models.NodeMetricItem{}
	now := time.Now().Unix()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var cpuResp, memResp, cpuCoresResp, totalMemResp *models.MetricsResponse

	// 併發查詢節點 CPU 使用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuQuery := "(1 - avg by (instance) (rate(node_cpu_seconds_total{mode=\"idle\"}[1m]))) * 100"
		if resp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: cpuQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil {
			mu.Lock()
			cpuResp = resp
			mu.Unlock()
		} else {
			logger.Error("查詢節點CPU使用率失敗", "error", err)
		}
	}()

	// 併發查詢節點記憶體使用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		memQuery := "(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100"
		if resp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: memQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil {
			mu.Lock()
			memResp = resp
			mu.Unlock()
		} else {
			logger.Error("查詢節點記憶體使用率失敗", "error", err)
		}
	}()

	// 併發查詢節點CPU核數
	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuCoresQuery := "machine_cpu_cores"
		if resp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: cpuCoresQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil {
			mu.Lock()
			cpuCoresResp = resp
			mu.Unlock()
		} else {
			logger.Error("查詢節點CPU核數失敗", "error", err)
		}
	}()

	// 併發查詢節點總記憶體
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalMemQuery := "machine_memory_bytes"
		if resp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: totalMemQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil {
			mu.Lock()
			totalMemResp = resp
			mu.Unlock()
		} else {
			logger.Error("查詢節點總記憶體失敗", "error", err)
		}
	}()

	// 等待所有查詢完成
	wg.Wait()

	// 構建節點對映
	nodeMap := make(map[string]*models.NodeMetricItem)

	// 處理 CPU 使用率資料
	if cpuResp != nil && len(cpuResp.Data.Result) > 0 {
		for _, result := range cpuResp.Data.Result {
			if instance, ok := result.Metric["instance"]; ok {
				nodeName := s.extractNodeName(instance)
				if _, exists := nodeMap[nodeName]; !exists {
					nodeMap[nodeName] = &models.NodeMetricItem{
						NodeName: nodeName,
						Status:   "Ready",
					}
				}
				if len(result.Values) > 0 {
					if val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64); err == nil {
						nodeMap[nodeName].CPUUsageRate = val
					}
				}
			}
		}
	}

	// 處理記憶體使用率資料
	if memResp != nil && len(memResp.Data.Result) > 0 {
		for _, result := range memResp.Data.Result {
			if instance, ok := result.Metric["instance"]; ok {
				nodeName := s.extractNodeName(instance)
				if _, exists := nodeMap[nodeName]; !exists {
					nodeMap[nodeName] = &models.NodeMetricItem{
						NodeName: nodeName,
						Status:   "Ready",
					}
				}
				if len(result.Values) > 0 {
					if val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64); err == nil {
						nodeMap[nodeName].MemoryUsageRate = val
					}
				}
			}
		}
	}

	// 處理 CPU 核數資料
	if cpuCoresResp != nil && len(cpuCoresResp.Data.Result) > 0 {
		for _, result := range cpuCoresResp.Data.Result {
			if instance, ok := result.Metric["instance"]; ok {
				nodeName := s.extractNodeName(instance)
				if _, exists := nodeMap[nodeName]; !exists {
					nodeMap[nodeName] = &models.NodeMetricItem{
						NodeName: nodeName,
						Status:   "Ready",
					}
				}
				if len(result.Values) > 0 {
					if val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64); err == nil {
						nodeMap[nodeName].CPUCores = val
					}
				}
			}
		}
	}

	// 處理總記憶體資料
	if totalMemResp != nil && len(totalMemResp.Data.Result) > 0 {
		for _, result := range totalMemResp.Data.Result {
			if instance, ok := result.Metric["instance"]; ok {
				nodeName := s.extractNodeName(instance)
				if _, exists := nodeMap[nodeName]; !exists {
					nodeMap[nodeName] = &models.NodeMetricItem{
						NodeName: nodeName,
						Status:   "Ready",
					}
				}
				if len(result.Values) > 0 {
					if val, err := strconv.ParseFloat(fmt.Sprintf("%v", result.Values[0][1]), 64); err == nil {
						nodeMap[nodeName].TotalMemory = val
					}
				}
			}
		}
	}

	// 轉換為列表
	for _, node := range nodeMap {
		nodeList = append(nodeList, *node)
	}

	return nodeList, nil
}
