package services

import (
	"context"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// queryWorkloadNetworkMetrics 查詢工作負載網路指標（聚合所有Pod）
func (s *PrometheusService) queryWorkloadNetworkMetrics(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkMetrics, error) {
	// 查詢入站流量（聚合）
	inQuery := fmt.Sprintf("sum(rate(container_network_receive_bytes_total{%s}[5m]))", selector)
	inSeries, err := s.queryMetricSeries(ctx, config, inQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載入站網路指標失敗", "error", err)
		inSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢出站流量（聚合）
	outQuery := fmt.Sprintf("sum(rate(container_network_transmit_bytes_total{%s}[5m]))", selector)
	outSeries, err := s.queryMetricSeries(ctx, config, outQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載出站網路指標失敗", "error", err)
		outSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkMetrics{
		In:  inSeries,
		Out: outSeries,
	}, nil
}

// queryWorkloadNetworkPPS 查詢工作負載網路PPS（聚合所有Pod）
func (s *PrometheusService) queryWorkloadNetworkPPS(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkPPS, error) {
	// 查詢入站PPS（聚合）
	inQuery := fmt.Sprintf("sum(rate(container_network_receive_packets_total{%s}[1m]))", selector)
	inSeries, err := s.queryMetricSeries(ctx, config, inQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載入站PPS失敗", "error", err)
		inSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢出站PPS（聚合）
	outQuery := fmt.Sprintf("sum(rate(container_network_transmit_packets_total{%s}[1m]))", selector)
	outSeries, err := s.queryMetricSeries(ctx, config, outQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載出站PPS失敗", "error", err)
		outSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkPPS{
		In:  inSeries,
		Out: outSeries,
	}, nil
}

// queryWorkloadNetworkDrops 查詢工作負載網路丟包（聚合所有Pod）
func (s *PrometheusService) queryWorkloadNetworkDrops(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.NetworkDrops, error) {
	// 查詢接收丟包（聚合）
	receiveQuery := fmt.Sprintf("sum(rate(container_network_receive_packets_dropped_total{%s}[1m]))", selector)
	receiveSeries, err := s.queryMetricSeries(ctx, config, receiveQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載接收丟包失敗", "error", err)
		receiveSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢傳送丟包（聚合）
	transmitQuery := fmt.Sprintf("sum(rate(container_network_transmit_packets_dropped_total{%s}[1m]))", selector)
	transmitSeries, err := s.queryMetricSeries(ctx, config, transmitQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載傳送丟包失敗", "error", err)
		transmitSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.NetworkDrops{
		Receive:  receiveSeries,
		Transmit: transmitSeries,
	}, nil
}

// queryWorkloadDiskIOPS 查詢工作負載磁碟IOPS（聚合所有Pod）
func (s *PrometheusService) queryWorkloadDiskIOPS(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.DiskIOPS, error) {
	// 查詢讀IOPS（聚合）
	readQuery := fmt.Sprintf("sum(rate(container_fs_reads_total{%s}[1m]))", selector)
	readSeries, err := s.queryMetricSeries(ctx, config, readQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載磁碟讀IOPS失敗", "error", err)
		readSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢寫IOPS（聚合）
	writeQuery := fmt.Sprintf("sum(rate(container_fs_writes_total{%s}[1m]))", selector)
	writeSeries, err := s.queryMetricSeries(ctx, config, writeQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載磁碟寫IOPS失敗", "error", err)
		writeSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.DiskIOPS{
		Read:  readSeries,
		Write: writeSeries,
	}, nil
}

// queryWorkloadDiskThroughput 查詢工作負載磁碟吞吐量（聚合所有Pod）
func (s *PrometheusService) queryWorkloadDiskThroughput(ctx context.Context, config *models.MonitoringConfig, selector string, start, end int64, step string) (*models.DiskThroughput, error) {
	// 查詢讀吞吐量（聚合）
	readQuery := fmt.Sprintf("sum(rate(container_fs_reads_bytes_total{container!=\"\",container!=\"POD\",%s}[1m]))", selector)
	readSeries, err := s.queryMetricSeries(ctx, config, readQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載磁碟讀吞吐量失敗", "error", err)
		readSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	// 查詢寫吞吐量（聚合）
	writeQuery := fmt.Sprintf("sum(rate(container_fs_writes_bytes_total{container!=\"\",container!=\"POD\",%s}[1m]))", selector)
	writeSeries, err := s.queryMetricSeries(ctx, config, writeQuery, start, end, step)
	if err != nil {
		logger.Error("查詢工作負載磁碟寫吞吐量失敗", "error", err)
		writeSeries = &models.MetricSeries{Current: 0, Series: []models.DataPoint{}}
	}

	return &models.DiskThroughput{
		Read:  readSeries,
		Write: writeSeries,
	}, nil
}
