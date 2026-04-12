package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// PrometheusService Prometheus 查詢服務
type PrometheusService struct {
	httpClient *http.Client
}

// NewPrometheusService 建立 Prometheus 服務
func NewPrometheusService() *PrometheusService {
	return &PrometheusService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // #nosec G402 -- 內部叢集 Prometheus 通訊，使用者可自行配置證書
				},
			},
		},
	}
}

// QueryPrometheus 查詢 Prometheus
func (s *PrometheusService) QueryPrometheus(ctx context.Context, config *models.MonitoringConfig, query *models.MetricsQuery) (*models.MetricsResponse, error) {
	if config.Type == "disabled" {
		return nil, fmt.Errorf("監控功能已禁用")
	}

	// 構建查詢 URL
	queryURL, err := s.buildQueryURL(config.Endpoint, query)
	if err != nil {
		return nil, fmt.Errorf("構建查詢URL失敗: %w", err)
	}

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("執行請求失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	// 檢查狀態碼
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查詢失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var result models.MetricsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w", err)
	}

	return &result, nil
}

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

// buildQueryURL 構建查詢 URL
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

// TestConnection 測試監控資料來源連線
func (s *PrometheusService) TestConnection(ctx context.Context, config *models.MonitoringConfig) error {
	if config.Type == "disabled" {
		return fmt.Errorf("監控功能已禁用")
	}

	// 構建測試查詢 URL
	testURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("無效的監控端點: %w", err)
	}
	testURL.Path = "/api/v1/query"
	testURL.RawQuery = "query=up"

	// 建立測試請求
	req, err := http.NewRequestWithContext(ctx, "GET", testURL.String(), nil)
	if err != nil {
		return fmt.Errorf("建立測試請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行測試請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("連線測試失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("監控資料來源響應異常: %s", string(body))
	}

	return nil
}

// queryPodNetworkPPS 查詢 Pod 網路PPS指標
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

// extractNodeName 從 instance 標籤中提取節點名稱
// ── Instant query helper ────────────────────────────────────────────────────

// instantResult is the minimal shape of a Prometheus /api/v1/query response.
type instantResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value []interface{} `json:"value"` // [unixtime, "value_string"]
		} `json:"result"`
	} `json:"data"`
}

// QueryInstantScalar executes a PromQL instant query and returns the first scalar result.
// Returns (math.NaN(), nil) when the result set is empty (metric absent / no data).
func (s *PrometheusService) QueryInstantScalar(ctx context.Context, config *models.MonitoringConfig, expr string) (float64, error) {
	if config == nil || config.Type == "disabled" {
		return math.NaN(), fmt.Errorf("monitoring disabled or config nil")
	}

	base, err := url.Parse(config.Endpoint)
	if err != nil {
		return math.NaN(), fmt.Errorf("parse endpoint: %w", err)
	}
	base.Path = "/api/v1/query"
	q := base.Query()
	q.Set("query", expr)
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", base.String(), nil)
	if err != nil {
		return math.NaN(), fmt.Errorf("build request: %w", err)
	}
	if err := s.setAuth(req, config.Auth); err != nil {
		return math.NaN(), fmt.Errorf("set auth: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return math.NaN(), fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return math.NaN(), fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return math.NaN(), fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, string(body))
	}

	var result instantResult
	if err := json.Unmarshal(body, &result); err != nil {
		return math.NaN(), fmt.Errorf("parse response: %w", err)
	}
	if result.Status != "success" {
		return math.NaN(), fmt.Errorf("prometheus status: %s", result.Status)
	}
	if len(result.Data.Result) == 0 {
		return math.NaN(), nil // no data — caller decides what to do
	}

	vals := result.Data.Result[0].Value
	if len(vals) < 2 {
		return math.NaN(), fmt.Errorf("unexpected value array length %d", len(vals))
	}
	raw, ok := vals[1].(string)
	if !ok {
		return math.NaN(), fmt.Errorf("value is not a string")
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return math.NaN(), fmt.Errorf("parse float %q: %w", raw, err)
	}
	return v, nil
}
