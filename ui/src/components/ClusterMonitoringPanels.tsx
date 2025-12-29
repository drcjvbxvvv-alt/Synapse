import React, { useState } from 'react';
import { Card, Row, Col, Space, Switch, Button, DatePicker, Popover, Divider, Typography } from 'antd';
import { ReloadOutlined, ClockCircleOutlined } from '@ant-design/icons';
import type { Dayjs } from 'dayjs';
import GrafanaPanel from './GrafanaPanel';
import { generateDataSourceUID } from '../config/grafana.config';

const { Text } = Typography;

// Grafana 风格的时间范围选项
const TIME_RANGE_OPTIONS = [
  {
    label: '快速选择',
    options: [
      { value: '5m', label: 'Last 5 minutes' },
      { value: '15m', label: 'Last 15 minutes' },
      { value: '30m', label: 'Last 30 minutes' },
      { value: '1h', label: 'Last 1 hour' },
      { value: '3h', label: 'Last 3 hours' },
      { value: '6h', label: 'Last 6 hours' },
      { value: '12h', label: 'Last 12 hours' },
      { value: '24h', label: 'Last 24 hours' },
    ],
  },
  {
    label: '更长时间',
    options: [
      { value: '2d', label: 'Last 2 days' },
      { value: '7d', label: 'Last 7 days' },
      { value: '30d', label: 'Last 30 days' },
      { value: '90d', label: 'Last 90 days' },
    ],
  },
];

interface ClusterMonitoringPanelsProps {
  clusterId: string;
  clusterName?: string;
}

// Grafana Dashboard 配置
const DASHBOARD_UID = 'kubepolaris-cluster-overview';

// Panel ID 映射（对应 Grafana Dashboard 中的 Panel）
const PANEL_IDS = {
  // 资源池
  workerNodes: 85,        // 工作节点数
  cpuCores: 87,           // CPU 总核数
  totalMemory: 89,        // 内存总数
  maxPods: 90,            // Pod 最大可创建数
  createdPods: 91,        // Pod 已创建数
  podUsage: 92,           // Pod 使用率
  availablePods: 93,      // Pod 可创建数
  
  // 集群状态
  etcdLeader: 69,         // Etcd has a leader?
  apiserverAvailability: 4, // apiserver 近30天可用率
  cpuRequest: 25,         // CPU request
  cpuLimit: 29,           // CPU limit
  memRequest: 27,         // 内存 request
  memLimit: 31,           // 内存 limit
  apiserverRequests: 2,   // apiserver总请求量
  clusterCpuUsage: 63,    // 集群 CPU 使用率
  clusterMemoryUsage: 65, // 集群内存使用率
  
  // Node 资源使用
  nodeCpuUsage: 104,      // CPU 使用率
  nodeMemoryUsage: 107,   // 内存使用率
  loadSaturation: 108,    // Load 饱和度
  networkTraffic: 106,    // Network Traffic
  
  // Pod 情况
  podAbnormalList: 73,    // Pod 异常列表
  podAbnormalDistribution: 47, // 异常 Pod 分布情况
};

const ClusterMonitoringPanels: React.FC<ClusterMonitoringPanelsProps> = ({
  clusterId,
  clusterName,
}) => {
  const [timeRange, setTimeRange] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  const [timePickerOpen, setTimePickerOpen] = useState(false);
  
  // 自定义时间范围状态
  const [isCustomRange, setIsCustomRange] = useState(false);
  const [customFromTime, setCustomFromTime] = useState<Dayjs | null>(null);
  const [customToTime, setCustomToTime] = useState<Dayjs | null>(null);

  // 根据集群名生成数据源 UID
  const dataSourceUid = clusterName ? generateDataSourceUID(clusterName) : '';

  // 获取时间范围（支持快速选择和自定义范围）
  const getFromTime = () => {
    if (isCustomRange && customFromTime) {
      return customFromTime.valueOf().toString(); // 毫秒时间戳
    }
    return `now-${timeRange}`;
  };

  const getToTime = () => {
    if (isCustomRange && customToTime) {
      return customToTime.valueOf().toString(); // 毫秒时间戳
    }
    return 'now';
  };

  // 获取显示的时间范围文本
  const getTimeRangeDisplay = () => {
    if (isCustomRange && customFromTime && customToTime) {
      return `${customFromTime.format('MM-DD HH:mm')} to ${customToTime.format('MM-DD HH:mm')}`;
    }
    const option = TIME_RANGE_OPTIONS.flatMap(g => g.options).find(o => o.value === timeRange);
    return option?.label || 'Last 1 hour';
  };

  // 应用自定义时间范围
  const applyCustomRange = () => {
    if (customFromTime && customToTime) {
      setIsCustomRange(true);
      setTimePickerOpen(false);
      setRefreshKey(prev => prev + 1);
    }
  };

  // 选择快速时间范围
  const handleQuickRangeSelect = (value: string) => {
    setTimeRange(value);
    setIsCustomRange(false);
    setTimePickerOpen(false);
    setRefreshKey(prev => prev + 1);
  };

  // 刷新间隔
  const getRefreshInterval = () => {
    return autoRefresh ? '30s' : undefined;
  };

  // 公共 Panel 配置
  // priority: 'high' = 立即加载, 'normal' = 延迟加载, 'low' = 最后加载
  const getPanelProps = (
    panelId: number, 
    height: number = 200, 
    priority: 'high' | 'normal' | 'low' = 'normal',
    batchIndex: number = 0  // 分批加载的批次索引
  ) => ({
    dashboardUid: DASHBOARD_UID,
    panelId,
    // 传递数据源 UID，切换集群时会自动切换到对应的 Prometheus 数据源
    variables: { DS_PROMETHEUS: dataSourceUid } as Record<string, string>,
    from: getFromTime(),
    to: getToTime(),
    refresh: getRefreshInterval(),
    height,
    showToolbar: false,
    theme: 'light' as const,
    // key 中包含 clusterName，切换集群时强制刷新所有 Panel
    key: `${panelId}-${refreshKey}-${clusterId}`,
    priority,
    loadDelay: batchIndex * 300, // 每批次延迟 300ms，控制并发
  });

  const handleRefresh = () => {
    setRefreshKey(prev => prev + 1);
  };

  // 时间选择器 Popover 内容
  const timePickerContent = (
    <div style={{ display: 'flex', gap: 16, padding: 8 }}>
      {/* 左侧：自定义时间范围 */}
      <div style={{ width: 240 }}>
        <Text strong style={{ marginBottom: 8, display: 'block' }}>Absolute time range</Text>
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>From</Text>
          <DatePicker
            showTime
            value={customFromTime}
            onChange={setCustomFromTime}
            style={{ width: '100%', marginTop: 4 }}
            placeholder="开始时间"
            format="YYYY-MM-DD HH:mm:ss"
          />
        </div>
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>To</Text>
          <DatePicker
            showTime
            value={customToTime}
            onChange={setCustomToTime}
            style={{ width: '100%', marginTop: 4 }}
            placeholder="结束时间"
            format="YYYY-MM-DD HH:mm:ss"
          />
        </div>
        <Button 
          type="primary" 
          block 
          onClick={applyCustomRange}
          disabled={!customFromTime || !customToTime}
        >
          Apply time range
        </Button>
      </div>
      
      <Divider type="vertical" style={{ height: 'auto' }} />
      
      {/* 右侧：快速选择 */}
      <div style={{ width: 160 }}>
        {TIME_RANGE_OPTIONS.map(group => (
          <div key={group.label} style={{ marginBottom: 12 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>{group.label}</Text>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginTop: 4 }}>
              {group.options.map(opt => (
                <Button
                  key={opt.value}
                  type={!isCustomRange && timeRange === opt.value ? 'primary' : 'text'}
                  size="small"
                  style={{ textAlign: 'left', justifyContent: 'flex-start' }}
                  onClick={() => handleQuickRangeSelect(opt.value)}
                >
                  {opt.label}
                </Button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );

  return (
    <div>
      <Card
        title="监控图表"
        extra={
          <Space>
            <Popover
              content={timePickerContent}
              trigger="click"
              open={timePickerOpen}
              onOpenChange={setTimePickerOpen}
              placement="bottomRight"
            >
              <Button icon={<ClockCircleOutlined />} style={{ minWidth: 180 }}>
                {getTimeRangeDisplay()}
              </Button>
            </Popover>
            <Space>
              <span>自动刷新</span>
              <Switch
                checked={autoRefresh}
                onChange={setAutoRefresh}
                checkedChildren="开"
                unCheckedChildren="关"
              />
            </Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={handleRefresh}
            >
              刷新
            </Button>
          </Space>
        }
      >
        {/* 集群资源总量 - 第一批加载 (高优先级) */}
        <Card size="small" title="集群资源总量" style={{ marginBottom: 16 }}>
          <Row gutter={[16, 16]}>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.workerNodes, 120, 'high', 0)} />
            </Col>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuCores, 120, 'high', 0)} />
            </Col>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.totalMemory, 120, 'high', 0)} />
            </Col>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.maxPods, 120, 'normal', 1)} />
            </Col>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.createdPods, 120, 'normal', 1)} />
            </Col>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.podUsage, 120, 'normal', 1)} />
            </Col>
            <Col span={3}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.availablePods, 120, 'normal', 1)} />
            </Col>
          </Row>
        </Card>

        {/* 集群状态 - 第二批加载 */}
        <Card size="small" title="集群状态" style={{ marginBottom: 16 }}>
          <Row gutter={[12, 12]} align="top">
            {/* 左侧：CPU/内存使用率 - 高优先级 */}
            <Col span={6}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.clusterCpuUsage, 280, 'high', 0)} />
            </Col>
            <Col span={6}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.clusterMemoryUsage, 280, 'high', 0)} />
            </Col>
            
            {/* 右侧：Stat 指标（第二批加载） */}
            <Col span={12}>
              <Row gutter={[8, 8]}>
                <Col span={4}>
                  <GrafanaPanel {...getPanelProps(PANEL_IDS.etcdLeader, 120, 'normal', 2)} />
                </Col>
                <Col span={4}>
                  <GrafanaPanel {...getPanelProps(PANEL_IDS.apiserverAvailability, 120, 'normal', 2)} />
                </Col>
                <Col span={4}>
                  <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuRequest, 120, 'normal', 2)} />
                </Col>
                <Col span={4}>
                  <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuLimit, 120, 'normal', 3)} />
                </Col>
                <Col span={4}>
                  <GrafanaPanel {...getPanelProps(PANEL_IDS.memRequest, 120, 'normal', 3)} />
                </Col>
                <Col span={4}>
                  <GrafanaPanel {...getPanelProps(PANEL_IDS.memLimit, 120, 'normal', 3)} />
                </Col>
              </Row>
              {/* apiserver 请求量 - 第四批 */}
              <div style={{ marginTop: 8 }}>
                <GrafanaPanel {...getPanelProps(PANEL_IDS.apiserverRequests, 150, 'normal', 4)} />
              </div>
            </Col>
          </Row>
        </Card>

        {/* Node 资源使用 - 第五批加载 (低优先级，需要滚动才可见) */}
        <Card size="small" title="Node 资源使用" style={{ marginBottom: 16 }}>
          <Row gutter={[16, 16]}>
            <Col span={12}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.nodeCpuUsage, 280, 'low', 5)} />
            </Col>
            <Col span={12}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.nodeMemoryUsage, 280, 'low', 5)} />
            </Col>
            <Col span={12}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.loadSaturation, 280, 'low', 6)} />
            </Col>
            <Col span={12}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.networkTraffic, 280, 'low', 6)} />
            </Col>
          </Row>
        </Card>

        {/* Pod 情况 - 最后加载 */}
        <Card size="small" title="Pod 情况">
          <Row gutter={[16, 16]}>
            <Col span={12}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.podAbnormalList, 320, 'low', 7)} />
            </Col>
            <Col span={12}>
              <GrafanaPanel {...getPanelProps(PANEL_IDS.podAbnormalDistribution, 320, 'low', 7)} />
            </Col>
          </Row>
        </Card>
      </Card>
    </div>
  );
};

export default ClusterMonitoringPanels;

