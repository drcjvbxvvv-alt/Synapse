import React from 'react';
import { Row, Col, Card, Statistic } from 'antd';
import { renderChart, renderMultiSeriesChart, renderNetworkChart, formatValue } from '../chartHelpers';
import type { ClusterMetricsData } from '../types';

interface PodWorkloadMonitoringViewProps {
  metrics: ClusterMetricsData;
  type: 'pod' | 'workload';
}

export const PodWorkloadMonitoringView: React.FC<PodWorkloadMonitoringViewProps> = ({
  metrics,
  type,
}) => {
  return (
    <>
      {/* 資源規格 */}
      {(metrics.cpu_request || metrics.cpu_limit || metrics.memory_request || metrics.memory_limit) && (
        <Col span={24}>
          <Card size="small" title="資源規格">
            <Row gutter={16}>
              {metrics.cpu_request && (
                <Col span={6}>
                  <Statistic
                    title="CPU Request"
                    value={metrics.cpu_request.current.toFixed(2)}
                    suffix="cores"
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Col>
              )}
              {metrics.cpu_limit && (
                <Col span={6}>
                  <Statistic
                    title="CPU Limit"
                    value={metrics.cpu_limit.current.toFixed(2)}
                    suffix="cores"
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Col>
              )}
              {metrics.memory_request && (
                <Col span={6}>
                  <Statistic
                    title="Memory Request"
                    value={formatValue(metrics.memory_request.current, 'bytes')}
                    valueStyle={{ color: '#52c41a' }}
                  />
                </Col>
              )}
              {metrics.memory_limit && (
                <Col span={6}>
                  <Statistic
                    title="Memory Limit"
                    value={formatValue(metrics.memory_limit.current, 'bytes')}
                    valueStyle={{ color: '#52c41a' }}
                  />
                </Col>
              )}
            </Row>
          </Card>
        </Col>
      )}

      {/* CPU 使用率 */}
      {metrics.cpu && (
        <Col span={12}>
          <Card size="small" title="CPU 使用">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="使用率"
                  value={metrics.cpu.current}
                  suffix="%"
                  precision={2}
                  valueStyle={{ color: metrics.cpu.current > 80 ? '#cf1322' : '#3f8600' }}
                />
              </Col>
              {metrics.cpu_usage_absolute && (
                <Col span={12}>
                  <Statistic
                    title="實際使用"
                    value={metrics.cpu_usage_absolute.current.toFixed(3)}
                    suffix="cores"
                    precision={3}
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Col>
              )}
            </Row>
            {type === 'workload' && metrics.cpu_multi ? (
              renderMultiSeriesChart(metrics.cpu_multi.series, '%')
            ) : (
              renderChart(metrics.cpu.series, '#1890ff', '%')
            )}
          </Card>
        </Col>
      )}

      {/* 記憶體使用率 */}
      {metrics.memory && (
        <Col span={12}>
          <Card size="small" title="記憶體使用">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="使用率"
                  value={metrics.memory.current}
                  suffix="%"
                  precision={2}
                  valueStyle={{ color: metrics.memory.current > 80 ? '#cf1322' : '#3f8600' }}
                />
              </Col>
              {metrics.memory_usage_bytes && (
                <Col span={12}>
                  <Statistic
                    title="實際使用"
                    value={formatValue(metrics.memory_usage_bytes.current, 'bytes')}
                    valueStyle={{ color: '#52c41a' }}
                  />
                </Col>
              )}
            </Row>
            {type === 'workload' && metrics.memory_multi ? (
              renderMultiSeriesChart(metrics.memory_multi.series, '%')
            ) : (
              renderChart(metrics.memory.series, '#52c41a', '%')
            )}
          </Card>
        </Col>
      )}

      {/* 容器重啟次數 */}
      {metrics.container_restarts && (
        <Col span={12}>
          <Card size="small" title="容器重啟次數">
            <Statistic
              value={metrics.container_restarts.current}
              precision={0}
              suffix="次"
              valueStyle={{ color: metrics.container_restarts.current > 0 ? '#cf1322' : '#3f8600' }}
            />
            {type === 'workload' && metrics.container_restarts_multi ? (
              renderMultiSeriesChart(metrics.container_restarts_multi.series, '次')
            ) : (
              renderChart(metrics.container_restarts.series, '#ff4d4f', '次 ')
            )}
          </Card>
        </Col>
      )}

      {/* OOM Kill 次數 */}
      {metrics.oom_kills && (
        <Col span={12}>
          <Card size="small" title="OOM Kill 次數">
            <Statistic
              value={metrics.oom_kills.current}
              precision={0}
              suffix="次"
              valueStyle={{ color: metrics.oom_kills.current > 0 ? '#cf1322' : '#3f8600' }}
            />
            {type === 'workload' && metrics.oom_kills_multi ? (
              renderMultiSeriesChart(metrics.oom_kills_multi.series, '次')
            ) : (
              renderChart(metrics.oom_kills.series, '#ff4d4f', '次 ')
            )}
          </Card>
        </Col>
      )}

      {/* 健康檢查失敗次數 */}
      {metrics.probe_failures && (
        <Col span={12}>
          <Card size="small" title="健康檢查失敗次數">
            <Statistic
              value={metrics.probe_failures.current}
              precision={2}
              suffix="次/分鐘"
              valueStyle={{ color: metrics.probe_failures.current > 0 ? '#cf1322' : '#3f8600' }}
            />
            {type === 'workload' && metrics.probe_failures_multi ? (
              renderMultiSeriesChart(metrics.probe_failures_multi.series, '次')
            ) : (
              renderChart(metrics.probe_failures.series, '#faad14', '次 ')
            )}
          </Card>
        </Col>
      )}

      {/* 網路 PPS */}
      {metrics.network_pps && (
        <Col span={24}>
          <Card size="small" title="網路 PPS（包/秒）">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="入站 PPS"
                  value={metrics.network_pps.in.current.toFixed(2)}
                  suffix="pps"
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="出站 PPS"
                  value={metrics.network_pps.out.current.toFixed(2)}
                  suffix="pps"
                />
              </Col>
            </Row>
            {renderNetworkChart(metrics.network_pps.in.series, metrics.network_pps.out.series, '', '入站', '出站')}
          </Card>
        </Col>
      )}

      {/* 磁碟 IOPS */}
      {metrics.disk_iops && (
        <Col span={24}>
          <Card size="small" title="磁碟 IOPS">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="讀 IOPS"
                  value={metrics.disk_iops.read.current.toFixed(2)}
                  suffix="ops/s"
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="寫 IOPS"
                  value={metrics.disk_iops.write.current.toFixed(2)}
                  suffix="ops/s"
                />
              </Col>
            </Row>
            {renderNetworkChart(metrics.disk_iops.read.series, metrics.disk_iops.write.series, '', '讀', '寫')}
          </Card>
        </Col>
      )}

      {/* 磁碟吞吐量 */}
      {metrics.disk_throughput && (
        <Col span={24}>
          <Card size="small" title="磁碟吞吐量">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="讀吞吐量"
                  value={formatValue(metrics.disk_throughput.read.current, 'bytes')}
                  suffix="/s"
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="寫吞吐量"
                  value={formatValue(metrics.disk_throughput.write.current, 'bytes')}
                  suffix="/s"
                />
              </Col>
            </Row>
            {renderNetworkChart(metrics.disk_throughput.read.series, metrics.disk_throughput.write.series, 'bytes', '讀', '寫')}
          </Card>
        </Col>
      )}

      {/* 執行緒數 */}
      {metrics.threads && (
        <Col span={12}>
          <Card size="small" title="執行緒數">
            <Statistic
              value={metrics.threads.current}
              precision={0}
              valueStyle={{ color: '#722ed1' }}
            />
            {type === 'workload' && metrics.threads_multi ? (
              renderMultiSeriesChart(metrics.threads_multi.series, '次')
            ) : (
              renderChart(metrics.threads.series, '#722ed1', '次 ')
            )}
          </Card>
        </Col>
      )}

      {/* CPU 限流情況 */}
      {(metrics.cpu_throttling || metrics.cpu_throttling_time) && (
        <Col span={24}>
          <Card size="small" title="CPU 限流情況">
            <Row gutter={16}>
              {metrics.cpu_throttling && (
                <Col span={12}>
                  <Card size="small" title="CPU 限流比例">
                    <Statistic
                      value={metrics.cpu_throttling.current}
                      suffix="%"
                      precision={2}
                      valueStyle={{ color: metrics.cpu_throttling.current > 10 ? '#cf1322' : '#3f8600' }}
                    />
                    {type === 'workload' && metrics.cpu_throttling_multi ? (
                      renderMultiSeriesChart(metrics.cpu_throttling_multi.series, '%')
                    ) : (
                      renderChart(metrics.cpu_throttling.series, '#ff7a45', '%')
                    )}
                  </Card>
                </Col>
              )}
              {metrics.cpu_throttling_time && (
                <Col span={12}>
                  <Card size="small" title="CPU 限流時間">
                    <Statistic
                      value={metrics.cpu_throttling_time.current}
                      suffix="秒"
                      precision={2}
                      valueStyle={{ color: metrics.cpu_throttling_time.current > 1 ? '#cf1322' : '#3f8600' }}
                    />
                    {type === 'workload' && metrics.cpu_throttling_time_multi ? (
                      renderMultiSeriesChart(metrics.cpu_throttling_time_multi.series, '秒')
                    ) : (
                      renderChart(metrics.cpu_throttling_time.series, '#ff4d4f', '秒 ')
                    )}
                  </Card>
                </Col>
              )}
            </Row>
          </Card>
        </Col>
      )}

      {/* 網絡卡丟包情況 */}
      {metrics.network_drops && (
        <Col span={24}>
          <Card size="small" title="網絡卡丟包情況">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="接收丟包"
                  value={metrics.network_drops.receive.current.toFixed(2)}
                  suffix="包/秒"
                  valueStyle={{ color: metrics.network_drops.receive.current > 0 ? '#cf1322' : '#3f8600' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="傳送丟包"
                  value={metrics.network_drops.transmit.current.toFixed(2)}
                  suffix="包/秒"
                  valueStyle={{ color: metrics.network_drops.transmit.current > 0 ? '#cf1322' : '#3f8600' }}
                />
              </Col>
            </Row>
            {type === 'workload' && metrics.network_drops_multi ? (
              renderMultiSeriesChart(metrics.network_drops_multi.series, '包/秒')
            ) : (
              renderNetworkChart(metrics.network_drops.receive.series, metrics.network_drops.transmit.series, '', '接收', '傳送')
            )}
          </Card>
        </Col>
      )}
    </>
  );
};
