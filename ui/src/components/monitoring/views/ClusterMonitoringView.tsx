import React from 'react';
import { Row, Col, Card, Statistic } from 'antd';
import GrafanaPanel from '../../GrafanaPanel';
import { generateDataSourceUID } from '../../../config/grafana.config';
import { renderChart, formatValue } from '../chartHelpers';
import type { ClusterMetricsData } from '../types';

interface ClusterMonitoringViewProps {
  metrics: ClusterMetricsData;
  clusterName?: string;
  grafanaUrl: string;
}

export const ClusterMonitoringView: React.FC<ClusterMonitoringViewProps> = ({
  metrics,
  clusterName,
  grafanaUrl,
}) => {
  if (!metrics.cluster_overview) {
    return null;
  }

  const overview = metrics.cluster_overview;

  return (
    <>
      {/* 資源總量 */}
      <Col span={24}>
        <Card size="small" title="叢集資源總量">
          <Row gutter={16}>
            <Col span={6}>
              <GrafanaPanel
                grafanaUrl={grafanaUrl}
                dashboardUid="k8s-cluster"
                panelId={80}
                variables={{
                  cluster: clusterName || '',
                  datasource: clusterName ? generateDataSourceUID(clusterName) : ''
                }}
                height={120}
                showToolbar={false}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="記憶體總數"
                value={formatValue(overview.total_memory, 'bytes')}
                valueStyle={{ color: '#fa8c16' }}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="Pod 最大可建立數"
                value={overview.max_pods}
                valueStyle={{ color: '#722ed1' }}
              />
            </Col>
          </Row>
        </Card>
      </Col>

      {/* Pod 狀態 */}
      <Col span={24}>
        <Card size="small" title="Pod 狀態">
          <Row gutter={16}>
            <Col span={6}>
              <Statistic
                title="Pod 已建立數"
                value={overview.created_pods}
                valueStyle={{ color: '#1890ff' }}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="Pod 可建立數"
                value={overview.available_pods}
                valueStyle={{ color: '#52c41a' }}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="Pod 使用率"
                value={overview.pod_usage_rate.toFixed(2)}
                suffix="%"
                valueStyle={{
                  color: overview.pod_usage_rate > 80 ? '#cf1322' : '#3f8600'
                }}
              />
            </Col>
          </Row>
        </Card>
      </Col>

      {/* 叢集狀態 */}
      <Col span={24}>
        <Card size="small" title="叢集狀態">
          <Row gutter={16}>
            <Col span={8}>
              <Statistic
                title="Etcd Leader 狀態"
                value={overview.etcd_has_leader ? 'YES' : 'NO'}
                valueStyle={{
                  color: overview.etcd_has_leader ? '#52c41a' : '#cf1322'
                }}
              />
            </Col>
            <Col span={8}>
              <Statistic
                title="ApiServer 近30天可用率"
                value={overview.apiserver_availability.toFixed(4)}
                suffix="%"
                precision={4}
                valueStyle={{ color: '#1890ff' }}
              />
            </Col>
          </Row>
        </Card>
      </Col>

      {/* 資源配額比率 */}
      <Col span={24}>
        <Card size="small" title="資源配額比率">
          <Row gutter={16}>
            {overview.cpu_request_ratio && (
              <Col span={12}>
                <Card size="small" title="CPU Request 比率">
                  <Statistic
                    value={overview.cpu_request_ratio.current}
                    suffix="%"
                    precision={2}
                    valueStyle={{ color: '#1890ff' }}
                  />
                  {renderChart(overview.cpu_request_ratio.series, '#1890ff', '%')}
                </Card>
              </Col>
            )}
            {overview.cpu_limit_ratio && (
              <Col span={12}>
                <Card size="small" title="CPU Limit 比率">
                  <Statistic
                    value={overview.cpu_limit_ratio.current}
                    suffix="%"
                    precision={2}
                    valueStyle={{
                      color: overview.cpu_limit_ratio.current > 100 ? '#cf1322' : '#52c41a'
                    }}
                  />
                  {renderChart(overview.cpu_limit_ratio.series, '#52c41a', '%')}
                </Card>
              </Col>
            )}
            {overview.mem_request_ratio && (
              <Col span={12}>
                <Card size="small" title="記憶體 Request 比率">
                  <Statistic
                    value={overview.mem_request_ratio.current}
                    suffix="%"
                    precision={2}
                    valueStyle={{ color: '#fa8c16' }}
                  />
                  {renderChart(overview.mem_request_ratio.series, '#fa8c16', '%')}
                </Card>
              </Col>
            )}
            {overview.mem_limit_ratio && (
              <Col span={12}>
                <Card size="small" title="記憶體 Limit 比率">
                  <Statistic
                    value={overview.mem_limit_ratio.current}
                    suffix="%"
                    precision={2}
                    valueStyle={{
                      color: overview.mem_limit_ratio.current > 100 ? '#cf1322' : '#52c41a'
                    }}
                  />
                  {renderChart(overview.mem_limit_ratio.series, '#722ed1', '%')}
                </Card>
              </Col>
            )}
          </Row>
        </Card>
      </Col>

      {/* ApiServer 請求量 */}
      {overview.apiserver_request_rate && (
        <Col span={24}>
          <Card size="small" title="ApiServer 總請求量">
            <Statistic
              value={overview.apiserver_request_rate.current.toFixed(2)}
              suffix="req/s"
              valueStyle={{ color: '#1890ff' }}
            />
            {renderChart(overview.apiserver_request_rate.series, '#1890ff', '')}
          </Card>
        </Col>
      )}

      {/* 叢集 CPU/記憶體使用率趨勢圖 */}
      {overview.cpu_usage_rate && (
        <Col span={12}>
          <Card size="small" title="叢集 CPU 使用率">
            <Statistic
              value={overview.cpu_usage_rate.current}
              suffix="%"
              precision={2}
              valueStyle={{
                color: overview.cpu_usage_rate.current > 80 ? '#cf1322' : '#3f8600'
              }}
            />
            {renderChart(overview.cpu_usage_rate.series, '#1890ff', '%')}
          </Card>
        </Col>
      )}

      {overview.memory_usage_rate && (
        <Col span={12}>
          <Card size="small" title="叢集記憶體使用率">
            <Statistic
              value={overview.memory_usage_rate.current}
              suffix="%"
              precision={2}
              valueStyle={{
                color: overview.memory_usage_rate.current > 80 ? '#cf1322' : '#3f8600'
              }}
            />
            {renderChart(overview.memory_usage_rate.series, '#52c41a', '%')}
          </Card>
        </Col>
      )}

      {/* Node 列表監控 */}
      {metrics.node_list && metrics.node_list.length > 0 && (
        <Col span={24}>
          <Card size="small" title="Node 資源使用情況">
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ backgroundColor: '#fafafa', borderBottom: '1px solid #f0f0f0' }}>
                    <th style={{ padding: '12px', textAlign: 'left' }}>節點名稱</th>
                    <th style={{ padding: '12px', textAlign: 'left' }}>CPU 核數</th>
                    <th style={{ padding: '12px', textAlign: 'left' }}>CPU 使用率</th>
                    <th style={{ padding: '12px', textAlign: 'left' }}>總記憶體</th>
                    <th style={{ padding: '12px', textAlign: 'left' }}>記憶體使用率</th>
                    <th style={{ padding: '12px', textAlign: 'left' }}>狀態</th>
                  </tr>
                </thead>
                <tbody>
                  {metrics.node_list.map((node, index) => (
                    <tr key={index} style={{ borderBottom: '1px solid #f0f0f0' }}>
                      <td style={{ padding: '12px' }}>{node.node_name}</td>
                      <td style={{ padding: '12px' }}>{node.cpu_cores} cores</td>
                      <td style={{ padding: '12px' }}>
                        <span style={{
                          color: node.cpu_usage_rate > 80 ? '#cf1322' : '#3f8600',
                          fontWeight: 'bold'
                        }}>
                          {node.cpu_usage_rate.toFixed(2)}%
                        </span>
                      </td>
                      <td style={{ padding: '12px' }}>{formatValue(node.total_memory, 'bytes')}</td>
                      <td style={{ padding: '12px' }}>
                        <span style={{
                          color: node.memory_usage_rate > 80 ? '#cf1322' : '#3f8600',
                          fontWeight: 'bold'
                        }}>
                          {node.memory_usage_rate.toFixed(2)}%
                        </span>
                      </td>
                      <td style={{ padding: '12px' }}>
                        <span style={{
                          color: node.status === 'Ready' ? '#52c41a' : '#cf1322'
                        }}>
                          {node.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </Col>
      )}
    </>
  );
};
