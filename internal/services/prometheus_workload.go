package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// QueryPodMetrics 查詢 Pod 監控指標
func (s *PrometheusService) QueryPodMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, namespace, podName string, timeRange string, step string) (*models.ClusterMetricsData, error) {
	// 解析時間範圍
	start, end, err := s.parseTimeRange(timeRange)
	if err != nil {
		return nil, fmt.Errorf("解析時間範圍失敗: %w", err)
	}

	metrics := &models.ClusterMetricsData{}

	// 構建 Pod 標籤選擇器
	podSelector := s.buildPodSelector(config.Labels, clusterName, namespace, podName)

	// 查詢 Pod CPU 使用率
	if cpuSeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum (rate(container_cpu_usage_seconds_total{container!=\"\",%s}[1m])) by(pod) /( sum (kube_pod_container_resource_limits{container!=\"\",resource=\"cpu\",%s}) by(pod) ) * 100", podSelector, podSelector), start, end, step); err == nil {
		metrics.CPU = cpuSeries
	}

	// 查詢 Pod 記憶體使用率
	if memorySeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\",%s}) by(pod)/sum(kube_pod_container_resource_limits{container!=\"\",container!=\"POD\",resource=\"memory\",%s}) by (pod) * 100", podSelector, podSelector), start, end, step); err == nil {
		// if memorySeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("container_memory_working_set_bytes{%s}", podSelector), start, end, step); err == nil {
		metrics.Memory = memorySeries
	}

	// 查詢 Pod 網路指標
	if networkMetrics, err := s.queryPodNetworkMetrics(ctx, config, podSelector, start, end, step); err == nil {
		metrics.Network = networkMetrics
	}

	// 查詢 CPU Request（固定值）
	if cpuRequest, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_requests{resource=\"cpu\",%s}) by (pod)", podSelector), start, end, step); err == nil {
		metrics.CPURequest = cpuRequest
	}

	// 查詢 CPU Limit（固定值）
	if cpuLimit, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_limits{resource=\"cpu\",%s}) by (pod)", podSelector), start, end, step); err == nil {
		metrics.CPULimit = cpuLimit
	}

	// 查詢 Memory Request（固定值）
	if memoryRequest, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_requests{resource=\"memory\",%s}) by (pod)", podSelector), start, end, step); err == nil {
		metrics.MemoryRequest = memoryRequest
	}

	// 查詢 Memory Limit（固定值）
	if memoryLimit, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_limits{resource=\"memory\",%s}) by (pod)", podSelector), start, end, step); err == nil {
		metrics.MemoryLimit = memoryLimit
	}

	// 查詢健康檢查失敗次數
	if probeFailures, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("increase(prober_probe_total{result='failed',%s}[1m])", podSelector), start, end, step); err == nil {
		metrics.ProbeFailures = probeFailures
	}

	// 查詢容器重啟次數
	if restarts, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("kube_pod_container_status_restarts_total{%s}", podSelector), start, end, step); err == nil {
		metrics.ContainerRestarts = restarts
	}

	// 查詢網路PPS
	if networkPPS, err := s.queryPodNetworkPPS(ctx, config, podSelector, start, end, step); err == nil {
		metrics.NetworkPPS = networkPPS
	}

	// 查詢執行緒數
	if threads, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_threads{container!=\"\",container!=\"POD\",%s})", podSelector), start, end, step); err == nil {
		metrics.Threads = threads
	}

	// 查詢網絡卡丟包情況
	if networkDrops, err := s.queryPodNetworkDrops(ctx, config, podSelector, start, end, step); err == nil {
		metrics.NetworkDrops = networkDrops
	}

	// 查詢 CPU 限流比例
	if cpuThrottling, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(rate(container_cpu_cfs_throttled_periods_total{%s}[1m])) / sum(rate(container_cpu_cfs_periods_total{%s}[5m])) * 100", podSelector, podSelector), start, end, step); err == nil {
		metrics.CPUThrottling = cpuThrottling
	}

	// 查詢 CPU 限流時間
	if cpuThrottlingTime, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(rate(container_cpu_cfs_throttled_seconds_total{%s}[1m]))", podSelector), start, end, step); err == nil {
		metrics.CPUThrottlingTime = cpuThrottlingTime
	}

	// 查詢磁碟 IOPS
	if diskIOPS, err := s.queryPodDiskIOPS(ctx, config, podSelector, start, end, step); err == nil {
		metrics.DiskIOPS = diskIOPS
	}

	// 查詢磁碟吞吐量
	if diskThroughput, err := s.queryPodDiskThroughput(ctx, config, podSelector, start, end, step); err == nil {
		metrics.DiskThroughput = diskThroughput
	}

	// 查詢 CPU 實際使用量（cores）
	if cpuAbsolute, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\",%s}[1m]))", podSelector), start, end, step); err == nil {
		metrics.CPUUsageAbsolute = cpuAbsolute
	}

	// 查詢記憶體實際使用量（bytes）
	if memoryBytes, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\",%s})", podSelector), start, end, step); err == nil {
		metrics.MemoryUsageBytes = memoryBytes
	}

	// 查詢 OOM Kill 次數
	if oomKills, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_oom_events_total{container!=\"\",container!=\"POD\",%s})", podSelector), start, end, step); err == nil {
		metrics.OOMKills = oomKills
	}

	return metrics, nil
}

// QueryWorkloadMetrics 查詢工作負載監控指標（聚合多個Pod的資料）
func (s *PrometheusService) QueryWorkloadMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, namespace, workloadName string, timeRange string, step string) (*models.ClusterMetricsData, error) {
	// 解析時間範圍
	start, end, err := s.parseTimeRange(timeRange)
	if err != nil {
		return nil, fmt.Errorf("解析時間範圍失敗: %w", err)
	}

	metrics := &models.ClusterMetricsData{}

	// 構建工作負載標籤選擇器（使用正規表示式匹配pod名稱）
	workloadSelector := s.buildWorkloadSelector(config.Labels, clusterName, namespace, workloadName)

	// 查詢工作負載 CPU 使用率
	if cpuSeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum (rate(container_cpu_usage_seconds_total{container!=\"\",%s}[1m])) /( sum (kube_pod_container_resource_limits{container!=\"\",resource=\"cpu\",%s}) ) * 100", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.CPU = cpuSeries
	}

	// 查詢工作負載記憶體使用率
	if memorySeries, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\",%s})/sum(kube_pod_container_resource_limits{container!=\"\",container!=\"POD\",resource=\"memory\",%s}) * 100", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.Memory = memorySeries
	}

	// 查詢工作負載網路指標
	if networkMetrics, err := s.queryWorkloadNetworkMetrics(ctx, config, workloadSelector, start, end, step); err == nil {
		metrics.Network = networkMetrics
	}

	// 查詢 CPU Request（固定值）
	if cpuRequest, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_requests{resource=\"cpu\",%s})/count(kube_pod_container_resource_requests{resource=\"cpu\",%s})", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.CPURequest = cpuRequest
	}

	// 查詢 CPU Limit（固定值）
	if cpuLimit, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_limits{resource=\"cpu\",%s})/count(kube_pod_container_resource_limits{resource=\"cpu\",%s})", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.CPULimit = cpuLimit
	}

	// 查詢 Memory Request（固定值）
	if memoryRequest, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_requests{resource=\"memory\",%s})/count(kube_pod_container_resource_requests{resource=\"memory\",%s})", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.MemoryRequest = memoryRequest
	}

	// 查詢 Memory Limit（固定值）
	if memoryLimit, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_resource_limits{resource=\"memory\",%s})/count(kube_pod_container_resource_limits{resource=\"memory\",%s})", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.MemoryLimit = memoryLimit
	}

	// 查詢健康檢查失敗次數
	if probeFailures, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(increase(prober_probe_total{result='failed',%s}[1m]))", workloadSelector), start, end, step); err == nil {
		metrics.ProbeFailures = probeFailures
	}

	// 查詢容器重啟次數（總和）
	if restarts, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(kube_pod_container_status_restarts_total{%s})", workloadSelector), start, end, step); err == nil {
		metrics.ContainerRestarts = restarts
	}

	// 查詢網路PPS
	if networkPPS, err := s.queryWorkloadNetworkPPS(ctx, config, workloadSelector, start, end, step); err == nil {
		metrics.NetworkPPS = networkPPS
	}

	// 查詢執行緒數（總和）
	if threads, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_threads{container!=\"\",container!=\"POD\",%s})", workloadSelector), start, end, step); err == nil {
		metrics.Threads = threads
	}

	// 查詢網絡卡丟包情況
	if networkDrops, err := s.queryWorkloadNetworkDrops(ctx, config, workloadSelector, start, end, step); err == nil {
		metrics.NetworkDrops = networkDrops
	}

	// 查詢 CPU 限流比例
	if cpuThrottling, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(rate(container_cpu_cfs_throttled_periods_total{%s}[1m])) / sum(rate(container_cpu_cfs_periods_total{%s}[5m])) * 100", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.CPUThrottling = cpuThrottling
	}

	// 查詢 CPU 限流時間
	if cpuThrottlingTime, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(rate(container_cpu_cfs_throttled_seconds_total{%s}[1m]))", workloadSelector), start, end, step); err == nil {
		metrics.CPUThrottlingTime = cpuThrottlingTime
	}

	// 查詢磁碟 IOPS
	if diskIOPS, err := s.queryWorkloadDiskIOPS(ctx, config, workloadSelector, start, end, step); err == nil {
		metrics.DiskIOPS = diskIOPS
	}

	// 查詢磁碟吞吐量
	if diskThroughput, err := s.queryWorkloadDiskThroughput(ctx, config, workloadSelector, start, end, step); err == nil {
		metrics.DiskThroughput = diskThroughput
	}

	// 查詢 CPU 實際使用量（cores）
	if cpuAbsolute, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\",%s}[1m]))", workloadSelector), start, end, step); err == nil {
		metrics.CPUUsageAbsolute = cpuAbsolute
	}

	// 查詢記憶體實際使用量（bytes）
	if memoryBytes, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_memory_working_set_bytes{container!=\"\",container!=\"POD\",%s})", workloadSelector), start, end, step); err == nil {
		metrics.MemoryUsageBytes = memoryBytes
	}

	// 查詢 OOM Kill 次數（總和）
	if oomKills, err := s.queryMetricSeries(ctx, config, fmt.Sprintf("sum(container_oom_events_total{container!=\"\",container!=\"POD\",%s})", workloadSelector), start, end, step); err == nil {
		metrics.OOMKills = oomKills
	}

	// 查詢多Pod時間序列資料（用於展示多條曲線）
	// CPU使用率（每個Pod獨立）
	if cpuMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum (rate(container_cpu_usage_seconds_total{container!=\"\",%s}[1m])) by(pod) /( sum (kube_pod_container_resource_limits{container!=\"\",resource=\"cpu\",%s}) by(pod) ) * 100", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.CPUMulti = cpuMulti
	}

	// 記憶體使用率（每個Pod獨立）
	if memoryMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(container_memory_working_set_bytes{container!=\"\",%s}) by(pod) / sum(kube_pod_container_resource_limits{container!=\"\",container!=\"POD\",resource=\"memory\",%s}) by(pod) * 100", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.MemoryMulti = memoryMulti
	}

	// 查詢容器重啟次數（多Pod）
	if containerRestartsMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(kube_pod_container_status_restarts_total{%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.ContainerRestartsMulti = containerRestartsMulti
	}

	// 查詢 OOM Kill 次數（多Pod）
	if oomKillsMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(container_oom_events_total{container!=\"\",container!=\"POD\",%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.OOMKillsMulti = oomKillsMulti
	}

	// 查詢網路PPS（多Pod）
	if networkPPSMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(network_packets_received_total{%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.NetworkPPSMulti = networkPPSMulti
	}

	// 查詢執行緒數（多Pod）
	if threadsMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(container_threads{container!=\"\",container!=\"POD\",%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.ThreadsMulti = threadsMulti
	}

	// 查詢網絡卡丟包情況（多Pod）
	if networkDropsMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(network_packets_dropped_total{%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.NetworkDropsMulti = networkDropsMulti
	}

	// 查詢 CPU 限流比例（多Pod）
	if cpuThrottlingMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(rate(container_cpu_cfs_throttled_periods_total{%s}[1m])) by(pod) / sum(rate(container_cpu_cfs_periods_total{%s}[5m])) by(pod) * 100", workloadSelector, workloadSelector), start, end, step); err == nil {
		metrics.CPUThrottlingMulti = cpuThrottlingMulti
	}

	// 查詢 CPU 限流時間（多Pod）
	if cpuThrottlingTimeMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(rate(container_cpu_cfs_throttled_seconds_total{%s}[1m])) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.CPUThrottlingTimeMulti = cpuThrottlingTimeMulti
	}

	// 查詢磁碟 IOPS（多Pod）
	if diskIOPSMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(disk_io_now{%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.DiskIOPSMulti = diskIOPSMulti
	}

	// 查詢磁碟吞吐量（多Pod）
	if diskThroughputMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(disk_io_bytes_total{%s}) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.DiskThroughputMulti = diskThroughputMulti
	}

	// 查詢健康檢查失敗次數（多Pod）
	if probeFailuresMulti, err := s.queryMultiSeriesMetric(ctx, config, fmt.Sprintf("sum(increase(prober_probe_total{result='failed',%s}[1m])) by(pod)", workloadSelector), start, end, step); err == nil {
		metrics.ProbeFailuresMulti = probeFailuresMulti
	}

	return metrics, nil
}

// QueryContainerSubnetIPs 查詢容器子網IP資訊
func (s *PrometheusService) QueryContainerSubnetIPs(ctx context.Context, config *models.MonitoringConfig) (*models.ContainerSubnetIPs, error) {
	if config.Type == "disabled" {
		return nil, fmt.Errorf("監控功能已禁用")
	}

	// 查詢總IP數
	totalIPsQuery := "sum(ipam_ippool_size)"
	totalResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
		Query: totalIPsQuery,
		Start: time.Now().Unix(),
		End:   time.Now().Unix(),
		Step:  "1m",
	})
	if err != nil {
		logger.Error("查詢總IP數失敗", "error", err)
		return &models.ContainerSubnetIPs{}, nil
	}

	totalIPs := 0
	if len(totalResp.Data.Result) > 0 && len(totalResp.Data.Result[0].Values) > 0 {
		if val, err := strconv.ParseFloat(fmt.Sprintf("%v", totalResp.Data.Result[0].Values[0][1]), 64); err == nil {
			totalIPs = int(val)
		}
	}

	// 查詢已使用IP數
	usedIPsQuery := "sum(ipam_allocations_in_use)"
	usedResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
		Query: usedIPsQuery,
		Start: time.Now().Unix(),
		End:   time.Now().Unix(),
		Step:  "1m",
	})
	if err != nil {
		logger.Error("查詢已使用IP數失敗", "error", err)
		return &models.ContainerSubnetIPs{TotalIPs: totalIPs}, nil
	}

	usedIPs := 0
	if len(usedResp.Data.Result) > 0 && len(usedResp.Data.Result[0].Values) > 0 {
		if val, err := strconv.ParseFloat(fmt.Sprintf("%v", usedResp.Data.Result[0].Values[0][1]), 64); err == nil {
			usedIPs = int(val)
		}
	}

	// 計算可用IP數
	availableIPs := totalIPs - usedIPs
	if availableIPs < 0 {
		availableIPs = 0
	}

	return &models.ContainerSubnetIPs{
		TotalIPs:     totalIPs,
		UsedIPs:      usedIPs,
		AvailableIPs: availableIPs,
	}, nil
}
