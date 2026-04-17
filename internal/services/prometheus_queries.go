package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ── Internal query helpers ──────────────────────────────────────────────────
// Private methods on *PrometheusService that perform individual Prometheus calls.
// Extracted from prometheus_service.go to reduce file size.

// queryMetricSeries 查詢指標時間序列
func (s *PrometheusService) queryMetricSeries(ctx context.Context, config *models.MonitoringConfig, query string, start, end int64, step string) (*models.MetricSeries, error) {
	logger.Debug("query: %s", query)
	metricsQuery := &models.MetricsQuery{
		Query: query,
		Start: start,
		End:   end,
		Step:  step,
	}

	resp, err := s.QueryPrometheus(ctx, config, metricsQuery)
	if err != nil {
		return nil, err
	}

	if len(resp.Data.Result) == 0 {
		return &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}, nil
	}

	// 處理第一個結果
	result := resp.Data.Result[0]
	var series []models.DataPoint
	var current float64

	if len(result.Values) > 0 {
		// 時間序列資料
		for _, value := range result.Values {
			if len(value) >= 2 {
				timestamp, _ := strconv.ParseInt(fmt.Sprintf("%.0f", value[0]), 10, 64)
				val, _ := strconv.ParseFloat(fmt.Sprintf("%v", value[1]), 64)
				series = append(series, models.DataPoint{
					Timestamp: timestamp,
					Value:     val,
				})
			}
		}
		// 當前值取最後一個
		if len(series) > 0 {
			current = series[len(series)-1].Value
		}
	} else if len(result.Value) >= 2 {
		// 即時查詢資料
		timestamp, _ := strconv.ParseInt(fmt.Sprintf("%.0f", result.Value[0]), 10, 64)
		val, _ := strconv.ParseFloat(fmt.Sprintf("%v", result.Value[1]), 64)
		series = append(series, models.DataPoint{
			Timestamp: timestamp,
			Value:     val,
		})
		current = val
	}

	return &models.MetricSeries{
		Current: current,
		Series:  series,
	}, nil
}

// queryMultiSeriesMetric 查詢多時間序列指標（每個Pod一條獨立曲線）
func (s *PrometheusService) queryMultiSeriesMetric(ctx context.Context, config *models.MonitoringConfig, query string, start, end int64, step string) (*models.MultiSeriesMetric, error) {
	logger.Debug("query multi-series: %s", query)
	metricsQuery := &models.MetricsQuery{
		Query: query,
		Start: start,
		End:   end,
		Step:  step,
	}

	resp, err := s.QueryPrometheus(ctx, config, metricsQuery)
	if err != nil {
		return nil, err
	}

	if len(resp.Data.Result) == 0 {
		return &models.MultiSeriesMetric{Series: []models.MultiSeriesDataPoint{}}, nil
	}

	// 構建時間戳到資料點的對映
	timestampMap := make(map[int64]map[string]float64)

	// 遍歷所有結果（每個結果代表一個Pod）
	for _, result := range resp.Data.Result {
		// 獲取 pod 名稱
		podName := ""
		if metric, ok := result.Metric["pod"]; ok {
			podName = fmt.Sprintf("%v", metric)
		}
		if podName == "" {
			continue
		}

		// 處理時間序列資料
		if len(result.Values) > 0 {
			for _, value := range result.Values {
				if len(value) >= 2 {
					timestamp, _ := strconv.ParseInt(fmt.Sprintf("%.0f", value[0]), 10, 64)
					valStr := fmt.Sprintf("%v", value[1])

					// 跳過無效值（NaN, +Inf, -Inf等）
					if valStr == "NaN" || valStr == "+Inf" || valStr == "-Inf" || valStr == "null" {
						continue
					}

					val, err := strconv.ParseFloat(valStr, 64)
					if err != nil {
						continue
					}

					// 再次檢查值是否有效
					if math.IsNaN(val) || math.IsInf(val, 0) {
						continue
					}

					if timestampMap[timestamp] == nil {
						timestampMap[timestamp] = make(map[string]float64)
					}
					timestampMap[timestamp][podName] = val
				}
			}
		}
	}

	// 將map轉換為有序切片
	var timestamps []int64
	for ts := range timestampMap {
		timestamps = append(timestamps, ts)
	}

	// 排序時間戳
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})

	// 構建最終的時間序列資料
	var series []models.MultiSeriesDataPoint
	for _, ts := range timestamps {
		series = append(series, models.MultiSeriesDataPoint{
			Timestamp: ts,
			Values:    timestampMap[ts],
		})
	}

	return &models.MultiSeriesMetric{
		Series: series,
	}, nil
}

// queryNetworkMetrics 查詢網路指標（使用併發查詢最佳化效能）
func (s *PrometheusService) queryNetworkMetrics(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkMetrics, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	networkMetrics := &models.NetworkMetrics{}

	// 併發查詢入站流量
	wg.Add(1)
	go func() {
		defer wg.Done()
		inQuery := fmt.Sprintf("sum(rate(container_network_receive_bytes_total{%s}[5m]))", selector)
		if inSeries, err := s.queryMetricSeries(ctx, config, inQuery, start, end, step); err == nil {
			mu.Lock()
			networkMetrics.In = inSeries
			mu.Unlock()
		} else {
			logger.Error("查詢入站網路指標失敗", "error", err)
			mu.Lock()
			networkMetrics.In = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
			mu.Unlock()
		}
	}()

	// 併發查詢出站流量
	wg.Add(1)
	go func() {
		defer wg.Done()
		outQuery := fmt.Sprintf("sum(rate(container_network_transmit_bytes_total{%s}[5m]))", selector)
		if outSeries, err := s.queryMetricSeries(ctx, config, outQuery, start, end, step); err == nil {
			mu.Lock()
			networkMetrics.Out = outSeries
			mu.Unlock()
		} else {
			logger.Error("查詢出站網路指標失敗", "error", err)
			mu.Lock()
			networkMetrics.Out = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
			mu.Unlock()
		}
	}()

	wg.Wait()
	return networkMetrics, nil
}

// queryNodeNetworkMetrics 查詢節點網路指標
func (s *PrometheusService) queryNodeNetworkMetrics(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkMetrics, error) {
	// 查詢入站流量
	inQuery := fmt.Sprintf("rate(node_network_receive_bytes_total{%s}[5m])", selector)
	inSeries, err := s.queryMetricSeries(ctx, config, inQuery, start, end, step)
	if err != nil {
		logger.Error("查詢節點入站網路指標失敗", "error", err)
		inSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢出站流量
	outQuery := fmt.Sprintf("rate(node_network_transmit_bytes_total{%s}[5m])", selector)
	outSeries, err := s.queryMetricSeries(ctx, config, outQuery, start, end, step)
	if err != nil {
		logger.Error("查詢節點出站網路指標失敗", "error", err)
		outSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkMetrics{
		In:  inSeries,
		Out: outSeries,
	}, nil
}

// queryClusterOverview 查詢叢集概覽指標（使用併發查詢最佳化效能）
func (s *PrometheusService) queryClusterOverview(ctx context.Context, config *models.MonitoringConfig, clusterName string, start, end int64, step string) (*models.ClusterOverview, error) {
	overview := &models.ClusterOverview{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 併發查詢 CPU 總核數
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalCPUQuery := "sum(machine_cpu_cores)"
		if cpuResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: totalCPUQuery,
			Start: end,
			End:   end,
			Step:  "1m",
		}); err == nil && len(cpuResp.Data.Result) > 0 && len(cpuResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", cpuResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				overview.TotalCPUCores = val
				mu.Unlock()
			}
		}
	}()

	// 併發查詢記憶體總數
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalMemQuery := "sum(machine_memory_bytes)"
		if memResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: totalMemQuery,
			Start: end,
			End:   end,
			Step:  "1m",
		}); err == nil && len(memResp.Data.Result) > 0 && len(memResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", memResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				overview.TotalMemory = val
				mu.Unlock()
			}
		}
	}()

	// 併發查詢叢集 CPU 使用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuUsageQuery := "(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[1m]))) * 100"
		if cpuUsageSeries, err := s.queryMetricSeries(ctx, config, cpuUsageQuery, start, end, step); err == nil {
			mu.Lock()
			overview.CPUUsageRate = cpuUsageSeries
			mu.Unlock()
		}
	}()

	// 併發查詢叢集記憶體使用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		memUsageQuery := "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100"
		if memUsageSeries, err := s.queryMetricSeries(ctx, config, memUsageQuery, start, end, step); err == nil {
			mu.Lock()
			overview.MemoryUsageRate = memUsageSeries
			mu.Unlock()
		}
	}()

	// 併發查詢 Pod 最大可建立數
	wg.Add(1)
	go func() {
		defer wg.Done()
		maxPodsQuery := "sum(kube_node_status_capacity{resource=\"pods\"} unless on(node) kube_node_role)"
		if maxPodsResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: maxPodsQuery,
			Start: end,
			End:   end,
			Step:  "1m",
		}); err == nil && len(maxPodsResp.Data.Result) > 0 && len(maxPodsResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", maxPodsResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				overview.MaxPods = int(val)
				mu.Unlock()
			}
		}
	}()

	// 併發查詢 Pod 已建立數
	wg.Add(1)
	go func() {
		defer wg.Done()
		createdPodsQuery := "sum(kube_pod_info)"
		if createdPodsResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: createdPodsQuery,
			Start: end,
			End:   end,
			Step:  "1m",
		}); err == nil && len(createdPodsResp.Data.Result) > 0 && len(createdPodsResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", createdPodsResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				overview.CreatedPods = int(val)
				mu.Unlock()
			}
		}
	}()

	// 併發查詢 Etcd 是否有 Leader
	wg.Add(1)
	go func() {
		defer wg.Done()
		etcdLeaderQuery := "etcd_server_has_leader"
		if etcdResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: etcdLeaderQuery,
			Start: end,
			End:   end,
			Step:  "1m",
		}); err == nil && len(etcdResp.Data.Result) > 0 && len(etcdResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", etcdResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				overview.EtcdHasLeader = val == 1
				mu.Unlock()
			}
		}
	}()

	// 併發查詢 ApiServer 近30天可用率
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiAvailabilityQuery := "apiserver_request:availability30d{verb=\"all\"}"
		if apiResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: apiAvailabilityQuery,
			Start: end,
			End:   end,
			Step:  "1m",
		}); err == nil && len(apiResp.Data.Result) > 0 && len(apiResp.Data.Result[0].Values) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", apiResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				overview.ApiServerAvailability = val * 100
				mu.Unlock()
			}
		}
	}()

	// 併發查詢 CPU Request 比值
	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuRequestQuery := "sum(namespace_cpu:kube_pod_container_resource_requests:sum) / sum(kube_node_status_allocatable{resource=\"cpu\"} unless on(node) kube_node_role) * 100"
		if cpuReqSeries, err := s.queryMetricSeries(ctx, config, cpuRequestQuery, start, end, step); err == nil {
			mu.Lock()
			overview.CPURequestRatio = cpuReqSeries
			mu.Unlock()
		}
	}()

	// 併發查詢 CPU Limit 比值
	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuLimitQuery := "sum(namespace_cpu:kube_pod_container_resource_limits:sum) / sum(kube_node_status_allocatable{resource=\"cpu\"} unless on(node) kube_node_role) * 100"
		if cpuLimitSeries, err := s.queryMetricSeries(ctx, config, cpuLimitQuery, start, end, step); err == nil {
			mu.Lock()
			overview.CPULimitRatio = cpuLimitSeries
			mu.Unlock()
		}
	}()

	// 併發查詢記憶體 Request 比值
	wg.Add(1)
	go func() {
		defer wg.Done()
		memRequestQuery := "sum(namespace_memory:kube_pod_container_resource_requests:sum) / sum(kube_node_status_allocatable{resource=\"memory\"} unless on(node) kube_node_role) * 100"
		if memReqSeries, err := s.queryMetricSeries(ctx, config, memRequestQuery, start, end, step); err == nil {
			mu.Lock()
			overview.MemRequestRatio = memReqSeries
			mu.Unlock()
		}
	}()

	// 併發查詢記憶體 Limit 比值
	wg.Add(1)
	go func() {
		defer wg.Done()
		memLimitQuery := "sum(namespace_memory:kube_pod_container_resource_limits:sum) / sum(kube_node_status_allocatable{resource=\"memory\"} unless on(node) kube_node_role) * 100"
		if memLimitSeries, err := s.queryMetricSeries(ctx, config, memLimitQuery, start, end, step); err == nil {
			mu.Lock()
			overview.MemLimitRatio = memLimitSeries
			mu.Unlock()
		}
	}()

	// 併發查詢 ApiServer 總請求量
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiRequestQuery := "sum(rate(apiserver_request_total[5m]))"
		if apiReqSeries, err := s.queryMetricSeries(ctx, config, apiRequestQuery, start, end, step); err == nil {
			mu.Lock()
			overview.ApiServerRequestRate = apiReqSeries
			mu.Unlock()
		}
	}()

	// 等待所有查詢完成
	wg.Wait()

	// 計算 Pod 可建立數和使用率（需要等待 MaxPods 和 CreatedPods 查詢完成）
	overview.AvailablePods = overview.MaxPods - overview.CreatedPods
	if overview.MaxPods > 0 {
		overview.PodUsageRate = float64(overview.CreatedPods) / float64(overview.MaxPods) * 100
	}

	return overview, nil
}

// extractNodeName 從 instance 標籤中提取節點名稱
func (s *PrometheusService) extractNodeName(instance string) string {
	// instance 格式可能是 "node-name:9100" 或 "192.168.1.1:9100"
	// 簡單處理：去除連接埠號
	parts := strings.Split(instance, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return instance
}

