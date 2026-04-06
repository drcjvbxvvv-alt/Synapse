import React, { useState } from 'react';
import { Row, Col, Card, Space, Select, Switch, Spin, Alert } from 'antd';
import GrafanaPanel from './GrafanaPanel';
import { GRAFANA_CONFIG, TIME_RANGE_MAP } from '../config/grafana.config';
import { useGrafanaUrl } from '../hooks/useGrafanaUrl';

const { Option } = Select;

interface GrafanaMonitoringChartsProps {
  clusterId: string;
  clusterName?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  workloadName?: string;
  type: 'cluster' | 'node' | 'pod' | 'workload';
  lazyLoad?: boolean;
}

const GrafanaMonitoringCharts: React.FC<GrafanaMonitoringChartsProps> = ({
  clusterId,
  clusterName,
  nodeName,
  namespace,
  podName,
  workloadName,
  type,
}) => {
  const { grafanaUrl, loading: grafanaUrlLoading } = useGrafanaUrl();
  const [timeRange, setTimeRange] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(false);

  // 構建變數對映
  const variables: Record<string, string> = {
    cluster: clusterName || clusterId,
  };
  
  if (nodeName) variables.node = nodeName;
  if (namespace) variables.namespace = namespace;
  if (podName) variables.pod = podName;
  if (workloadName) variables.workload = workloadName;

  // 獲取配置
  const config = GRAFANA_CONFIG[type] as { dashboardUid: string; panels: Record<string, number> };
  const { dashboardUid, panels } = config;

  // 時間範圍
  const { from, to } = TIME_RANGE_MAP[timeRange];
  const refresh = autoRefresh ? '30s' : undefined;

  // 渲染 Pod 型別的圖表
  const renderPodCharts = () => (
    <Row gutter={[16, 16]}>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.cpuUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="CPU 使用率"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.memoryUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="記憶體使用率"
          height={250}
        />
      </Col>
      <Col span={24}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.networkTraffic}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="網路流量"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.containerRestarts}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="容器重啟次數"
          height={200}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.cpuThrottling}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="CPU 限流"
          height={200}
        />
      </Col>
    </Row>
  );

  // 渲染叢集型別的圖表
  const renderClusterCharts = () => (
    <Row gutter={[16, 16]}>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.cpuUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="叢集 CPU 使用率"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.memoryUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="叢集記憶體使用率"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.podStatus}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="Pod 狀態概覽"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.networkTraffic}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="網路流量"
          height={250}
        />
      </Col>
      <Col span={24}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.nodeOverview}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="節點資源使用概覽"
          height={300}
        />
      </Col>
    </Row>
  );

  // 渲染節點型別的圖表
  const renderNodeCharts = () => (
    <Row gutter={[16, 16]}>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.cpuUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="CPU 使用率"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.memoryUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="記憶體使用率"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.diskUsage}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="磁碟使用率"
          height={250}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.networkIO}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="網路 I/O"
          height={250}
        />
      </Col>
    </Row>
  );

  // 渲染工作負載型別的圖表
  const renderWorkloadCharts = () => (
    <Row gutter={[16, 16]}>
      <Col span={24}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.cpuUsageMulti}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="CPU 使用率（多 Pod 對比）"
          height={300}
        />
      </Col>
      <Col span={24}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.memoryUsageMulti}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="記憶體使用率（多 Pod 對比）"
          height={300}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.podStatus}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="Pod 狀態"
          height={200}
        />
      </Col>
      <Col span={12}>
        <GrafanaPanel
          grafanaUrl={grafanaUrl}
          dashboardUid={dashboardUid}
          panelId={panels.restartCount}
          variables={variables}
          from={from}
          to={to}
          refresh={refresh}
          title="重啟次數"
          height={200}
        />
      </Col>
    </Row>
  );

  // 根據型別渲染對應圖表
  const renderCharts = () => {
    switch (type) {
      case 'cluster':
        return renderClusterCharts();
      case 'node':
        return renderNodeCharts();
      case 'pod':
        return renderPodCharts();
      case 'workload':
        return renderWorkloadCharts();
      default:
        return null;
    }
  };

  return (
    <Card
      title="監控圖表 (Grafana)"
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
          <Space>
            <span>自動重新整理</span>
            <Switch
              checked={autoRefresh}
              onChange={setAutoRefresh}
              checkedChildren="開"
              unCheckedChildren="關"
            />
          </Space>
        </Space>
      }
    >
      {grafanaUrlLoading ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin size="large" /></div>
      ) : !grafanaUrl ? (
        <Alert
          message="Grafana 未配置"
          description="請在「系統設定 → Grafana 設定」中配置 Grafana 地址。"
          type="warning"
          showIcon
        />
      ) : (
        renderCharts()
      )}
    </Card>
  );
};

export default GrafanaMonitoringCharts;
