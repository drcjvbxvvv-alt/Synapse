import React from 'react';
import { Row, Col, Card, Statistic } from 'antd';
import { renderChart, renderNetworkChart, formatValue } from '../chartHelpers';
import type { ClusterMetricsData } from '../types';

interface SharedMetricsViewProps {
  metrics: ClusterMetricsData;
}

export const SharedMetricsView: React.FC<SharedMetricsViewProps> = ({ metrics }) => {
  return (
    <>
      {/* 網路流量 */}
      {metrics.network && (
        <Col span={24}>
          <Card size="small" title="網路流量">
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title="入站流量"
                  value={formatValue(metrics.network.in.current, 'bytes')}
                  suffix="/s"
                  precision={2}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="出站流量"
                  value={formatValue(metrics.network.out.current, 'bytes')}
                  suffix="/s"
                  precision={2}
                />
              </Col>
            </Row>
            {renderNetworkChart(metrics.network.in.series, metrics.network.out.series, 'bytes', '入站', '出站')}
          </Card>
        </Col>
      )}

      {/* 儲存使用率 */}
      {metrics.storage && (
        <Col span={12}>
          <Card size="small" title="儲存使用率">
            <Statistic
              value={metrics.storage.current}
              suffix="%"
              precision={2}
              valueStyle={{ color: metrics.storage.current > 80 ? '#cf1322' : '#3f8600' }}
            />
            {renderChart(metrics.storage.series, '#fa8c16', '%')}
          </Card>
        </Col>
      )}

      {/* Pod 統計 */}
      {metrics.pods && (
        <Col span={12}>
          <Card size="small" title="Pod 狀態">
            <Row gutter={16}>
              <Col span={6}>
                <Statistic
                  title="總數"
                  value={metrics.pods.total}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="執行中"
                  value={metrics.pods.running}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="等待中"
                  value={metrics.pods.pending}
                  valueStyle={{ color: '#faad14' }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="失敗"
                  value={metrics.pods.failed}
                  valueStyle={{ color: '#cf1322' }}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      )}
    </>
  );
};
