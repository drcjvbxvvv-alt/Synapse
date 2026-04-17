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

// queryPodMetrics 查詢 Pod 統計指標（使用併發查詢最佳化效能）
func (s *PrometheusService) queryPodMetrics(ctx context.Context, config *models.MonitoringConfig, selector string) (*models.PodMetrics, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	podMetrics := &models.PodMetrics{}
	now := time.Now().Unix()

	// 併發查詢總 Pod 數
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalQuery := fmt.Sprintf("sum(kube_pod_info{%s})", selector)
		if totalResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: totalQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil && len(totalResp.Data.Result) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", totalResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				podMetrics.Total = int(val)
				mu.Unlock()
			}
		} else if err != nil {
			logger.Error("查詢Pod總數失敗", "error", err)
		}
	}()

	// 併發查詢執行中 Pod 數
	wg.Add(1)
	go func() {
		defer wg.Done()
		runningQuery := fmt.Sprintf("sum(kube_pod_status_phase{phase=\"Running\",%s})", selector)
		if runningResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: runningQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil && len(runningResp.Data.Result) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", runningResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				podMetrics.Running = int(val)
				mu.Unlock()
			}
		} else if err != nil {
			logger.Error("查詢執行中Pod數失敗", "error", err)
		}
	}()

	// 併發查詢 Pending Pod 數
	wg.Add(1)
	go func() {
		defer wg.Done()
		pendingQuery := fmt.Sprintf("sum(kube_pod_status_phase{phase=\"Pending\",%s})", selector)
		if pendingResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: pendingQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil && len(pendingResp.Data.Result) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", pendingResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				podMetrics.Pending = int(val)
				mu.Unlock()
			}
		} else if err != nil {
			logger.Error("查詢Pending Pod數失敗", "error", err)
		}
	}()

	// 併發查詢失敗 Pod 數
	wg.Add(1)
	go func() {
		defer wg.Done()
		failedQuery := fmt.Sprintf("sum(kube_pod_status_phase{phase=\"Failed\",%s})", selector)
		if failedResp, err := s.QueryPrometheus(ctx, config, &models.MetricsQuery{
			Query: failedQuery,
			Start: now,
			End:   now,
			Step:  "1m",
		}); err == nil && len(failedResp.Data.Result) > 0 {
			if val, err := strconv.ParseFloat(fmt.Sprintf("%v", failedResp.Data.Result[0].Values[0][1]), 64); err == nil {
				mu.Lock()
				podMetrics.Failed = int(val)
				mu.Unlock()
			}
		} else if err != nil {
			logger.Error("查詢失敗Pod數失敗", "error", err)
		}
	}()

	wg.Wait()
	return podMetrics, nil
}

// queryPodNetworkMetrics 查詢 Pod 網路指標
func (s *PrometheusService) queryPodNetworkMetrics(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkMetrics, error) {
	// 查詢入站流量
	inQuery := fmt.Sprintf("rate(container_network_receive_bytes_total{%s}[5m])", selector)
	inSeries, err := s.queryMetricSeries(ctx, config, inQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod入站網路指標失敗", "error", err)
		inSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢出站流量
	outQuery := fmt.Sprintf("rate(container_network_transmit_bytes_total{%s}[5m])", selector)
	outSeries, err := s.queryMetricSeries(ctx, config, outQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod出站網路指標失敗", "error", err)
		outSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkMetrics{
		In:  inSeries,
		Out: outSeries,
	}, nil
}

// queryPodNetworkPPS 查詢 Pod 網路PPS指標
func (s *PrometheusService) queryPodNetworkPPS(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkPPS, error) {
	// 查詢入站PPS
	inQuery := fmt.Sprintf("sum(rate(container_network_receive_packets_total{%s}[1m]))", selector)
	inSeries, err := s.queryMetricSeries(ctx, config, inQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod入站PPS失敗", "error", err)
		inSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢出站PPS
	outQuery := fmt.Sprintf("sum(rate(container_network_transmit_packets_total{%s}[1m]))", selector)
	outSeries, err := s.queryMetricSeries(ctx, config, outQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod出站PPS失敗", "error", err)
		outSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkPPS{
		In:  inSeries,
		Out: outSeries,
	}, nil
}

// queryPodNetworkDrops 查詢 Pod 網絡卡丟包情況
func (s *PrometheusService) queryPodNetworkDrops(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkDrops, error) {
	// 查詢接收丟包
	receiveQuery := fmt.Sprintf("sum(rate(container_network_receive_packets_dropped_total{%s}[1m]))", selector)
	receiveSeries, err := s.queryMetricSeries(ctx, config, receiveQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod接收丟包失敗", "error", err)
		receiveSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢傳送丟包
	transmitQuery := fmt.Sprintf("sum(rate(container_network_transmit_packets_dropped_total{%s}[1m]))", selector)
	transmitSeries, err := s.queryMetricSeries(ctx, config, transmitQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod傳送丟包失敗", "error", err)
		transmitSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkDrops{
		Receive:  receiveSeries,
		Transmit: transmitSeries,
	}, nil
}

// queryPodDiskIOPS 查詢 Pod 磁碟IOPS
func (s *PrometheusService) queryPodDiskIOPS(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.DiskIOPS, error) {
	// 查詢讀IOPS
	readQuery := fmt.Sprintf("sum(rate(container_fs_reads_total{%s}[1m]))", selector)
	readSeries, err := s.queryMetricSeries(ctx, config, readQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod磁碟讀IOPS失敗", "error", err)
		readSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢寫IOPS
	writeQuery := fmt.Sprintf("sum(rate(container_fs_writes_total{%s}[1m]))", selector)
	writeSeries, err := s.queryMetricSeries(ctx, config, writeQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod磁碟寫IOPS失敗", "error", err)
		writeSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.DiskIOPS{
		Read:  readSeries,
		Write: writeSeries,
	}, nil
}

// queryPodDiskThroughput 查詢 Pod 磁碟吞吐量
func (s *PrometheusService) queryPodDiskThroughput(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.DiskThroughput, error) {
	// 查詢讀吞吐量
	readQuery := fmt.Sprintf("sum(rate(container_fs_reads_bytes_total{container!=\"\",container!=\"POD\",%s}[1m]))", selector)
	readSeries, err := s.queryMetricSeries(ctx, config, readQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod磁碟讀吞吐量失敗", "error", err)
		readSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢寫吞吐量
	writeQuery := fmt.Sprintf("sum(rate(container_fs_writes_bytes_total{container!=\"\",container!=\"POD\",%s}[1m]))", selector)
	writeSeries, err := s.queryMetricSeries(ctx, config, writeQuery, start, end, step)
	if err != nil {
		logger.Error("查詢Pod磁碟寫吞吐量失敗", "error", err)
		writeSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.DiskThroughput{
		Read:  readSeries,
		Write: writeSeries,
	}, nil
}
