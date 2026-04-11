import React from 'react';
import { Card, Row, Select, Button, Space, Alert, Switch, Skeleton } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useGrafanaUrl } from '../hooks/useGrafanaUrl';
import { useMonitoringData } from './monitoring/useMonitoringData';
import {
  ClusterMonitoringView,
  PodWorkloadMonitoringView,
  SharedMetricsView,
} from './monitoring/views';
import type { MonitoringChartsProps } from './monitoring/types';

const { Option } = Select;

const MonitoringCharts: React.FC<MonitoringChartsProps> = (props) => {
  const {
    clusterId,
    clusterName,
    nodeName,
    namespace,
    podName,
    workloadName,
    type,
    lazyLoad = false,
  } = props;

  const { grafanaUrl } = useGrafanaUrl();
  const {
    metrics,
    loading,
    timeRange,
    setTimeRange,
    step,
    setStep,
    autoRefresh,
    setAutoRefresh,
    fetchMetrics,
  } = useMonitoringData({
    clusterId,
    clusterName,
    nodeName,
    namespace,
    podName,
    workloadName,
    type,
    lazyLoad,
  });

  // Lazy loading skeleton
  if (lazyLoad && !metrics && !loading) {
    return (
      <div style={{ padding: '24px' }}>
        <Card title="監控圖表">
          <Skeleton active paragraph={{ rows: 8 }} />
        </Card>
      </div>
    );
  }

  // Loading state
  if (loading && !metrics) {
    return (
      <div style={{ padding: '24px' }}>
        <Card title="監控圖表">
          <Skeleton active paragraph={{ rows: 8 }} />
        </Card>
      </div>
    );
  }

  // No data state
  if (!metrics) {
    return (
      <Alert
        message="監控資料不可用"
        description="請檢查監控配置是否正確，或監控資料來源是否可用。"
        type="warning"
        showIcon
      />
    );
  }

  return (
    <div>
      <Card
        title="監控圖表"
        extra={
          <Space>
            <Select
              value={timeRange}
              onChange={setTimeRange}
              style={{ width: 100 }}
            >
              <Option value="1h">1小時</Option>
              <Option value="6h">6小時</Option>
              <Option value="24h">24小時</Option>
              <Option value="7d">7天</Option>
            </Select>
            <Select
              value={step}
              onChange={setStep}
              style={{ width: 100 }}
            >
              <Option value="15s">15秒</Option>
              <Option value="1m">1分鐘</Option>
              <Option value="5m">5分鐘</Option>
              <Option value="15m">15分鐘</Option>
              <Option value="1h">1小時</Option>
            </Select>
            <Space>
              <span>自動重新整理</span>
              <Switch
                checked={autoRefresh}
                onChange={setAutoRefresh}
                checkedChildren="開"
                unCheckedChildren="關"
              />
            </Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => fetchMetrics()}
              loading={loading}
            >
              重新整理
            </Button>
          </Space>
        }
      >
        <Row gutter={[16, 16]}>
          {/* Cluster-specific monitoring */}
          {type === 'cluster' && (
            <ClusterMonitoringView
              metrics={metrics}
              clusterName={clusterName}
              grafanaUrl={grafanaUrl}
            />
          )}

          {/* Pod/Workload-specific monitoring */}
          {(type === 'pod' || type === 'workload') && (
            <PodWorkloadMonitoringView
              metrics={metrics}
              type={type}
            />
          )}

          {/* Shared metrics (network, storage, pods) */}
          <SharedMetricsView metrics={metrics} />
        </Row>
      </Card>
    </div>
  );
};

export default MonitoringCharts;
