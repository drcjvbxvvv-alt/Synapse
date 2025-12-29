import React, { useState } from 'react';
import { Card, Row, Col, Space, Switch, Button, DatePicker, Popover, Divider, Typography, Empty } from 'antd';
import { ReloadOutlined, ClockCircleOutlined } from '@ant-design/icons';
import type { Dayjs } from 'dayjs';
import GrafanaPanel from '../../../components/GrafanaPanel';
import { generateDataSourceUID } from '../../../config/grafana.config';

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
    ],
  },
  {
    label: '更长时间',
    options: [
      { value: '12h', label: 'Last 12 hours' },
      { value: '24h', label: 'Last 24 hours' },
      { value: '2d', label: 'Last 2 days' },
      { value: '7d', label: 'Last 7 days' },
    ],
  },
];

interface MonitoringTabProps {
  clusterId: string;
  clusterName?: string;
  namespace: string;
  workloadName: string;
  workloadType?: 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout';
}

// Grafana Dashboard UID（需要在 Grafana 中导入对应的 Dashboard）
const DASHBOARD_UID = 'kubepolaris-workload-detail';

// Panel ID 映射（对应 Grafana Dashboard 中的 Panel）
const PANEL_IDS = {
  // 第一行：CPU、内存、IO
  cpuUsage: 2,              // CPU 使用率
  memoryUsage: 6,           // Memory 使用率
  ioReadQps: 18,            // IO Read QPS
  ioWriteQps: 19,           // IO Write QPS
  
  // 第二行：限制指标和健康状态
  cpuLimit: 28,             // CPU 核限制
  memoryLimit: 30,          // 内存限制
  availability: 34,         // 容器整体可用率
  healthCheckFailed: 36,    // 健康检查失败次数
  containerRestarts: 38,    // 容器重启情况
  
  // 第三行：网络流量
  networkIncoming: 4,       // Network Incoming
  networkOutgoing: 14,      // Network Outgoing
  networkInputPps: 15,      // Network Input PPS
  networkOutputPps: 16,     // Network Output PPS
  
  // 第四行：文件句柄和线程
  fileDescriptors: 22,      // 文件句柄打开数
  runningThreads: 23,       // Running Threads
  networkInputDropped: 12,  // Network Input Dropped
  networkOutputDropped: 20, // Network Output Dropped
  
  // 第五行：CPU限流
  cpuThrottleRate: 46,      // CPU限流比例
  cpuThrottleTime: 32,      // CPU节流时间
  
};

const MonitoringTab: React.FC<MonitoringTabProps> = ({
  clusterId,
  clusterName,
  namespace,
  workloadName,
}) => {
  const [timeRange, setTimeRange] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  const [timePickerOpen, setTimePickerOpen] = useState(false);
  const [isCustomRange, setIsCustomRange] = useState(false);
  const [customFromTime, setCustomFromTime] = useState<Dayjs | null>(null);
  const [customToTime, setCustomToTime] = useState<Dayjs | null>(null);

  // 根据集群名生成数据源 UID
  const dataSourceUid = clusterName ? generateDataSourceUID(clusterName) : '';

  // 获取时间范围
  const getFromTime = () => {
    if (isCustomRange && customFromTime) {
      return customFromTime.valueOf().toString();
    }
    return `now-${timeRange}`;
  };

  const getToTime = () => {
    if (isCustomRange && customToTime) {
      return customToTime.valueOf().toString();
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
  const getPanelProps = (
    panelId: number,
    height: number = 200,
    priority: 'high' | 'normal' | 'low' = 'normal',
    batchIndex: number = 0
  ) => ({
    dashboardUid: DASHBOARD_UID,
    panelId,
    variables: {
      DS_PROMETHEUS: dataSourceUid,
      deployment_namespace: namespace,
      podname: workloadName,
      Interface: 'eth0',
      Intervals: '1m',
    } as Record<string, string>,
    from: getFromTime(),
    to: getToTime(),
    refresh: getRefreshInterval(),
    height,
    showToolbar: false,
    theme: 'light' as const,
    key: `${panelId}-${refreshKey}-${clusterId}-${namespace}-${workloadName}`,
    priority,
    loadDelay: batchIndex * 300,
  });

  const handleRefresh = () => {
    setRefreshKey(prev => prev + 1);
  };

  // 检查必要的参数
  if (!clusterName) {
    return (
      <Empty
        description="无法获取集群信息，请刷新页面重试"
        style={{ padding: '60px 0' }}
      />
    );
  }

  // 时间选择器 Popover 内容
  const timePickerContent = (
    <div style={{ display: 'flex', gap: 16, padding: 8 }}>
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
      {/* 工具栏 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
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
          <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
            刷新
          </Button>
        </Space>
      </div>

      {/* CPU/内存/IO 使用率 */}
      <Card size="small" title="资源使用" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuUsage, 220, 'high', 0)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.memoryUsage, 220, 'high', 0)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.ioReadQps, 220, 'high', 0)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.ioWriteQps, 220, 'high', 0)} />
          </Col>
        </Row>
      </Card>

      {/* 资源限制和容器状态 */}
      <Card size="small" title="容器状态" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={4}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuLimit, 180, 'normal', 1)} />
          </Col>
          <Col span={4}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.memoryLimit, 180, 'normal', 1)} />
          </Col>
          <Col span={4}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.availability, 180, 'normal', 1)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.healthCheckFailed, 180, 'normal', 2)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.containerRestarts, 180, 'normal', 2)} />
          </Col>
        </Row>
      </Card>

      {/* 网络流量 */}
      <Card size="small" title="网络流量" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkIncoming, 220, 'normal', 3)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkOutgoing, 220, 'normal', 3)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkInputPps, 220, 'normal', 3)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkOutputPps, 220, 'normal', 3)} />
          </Col>
        </Row>
      </Card>

      {/* 系统资源 */}
      <Card size="small" title="系统资源" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.fileDescriptors, 220, 'low', 4)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.runningThreads, 220, 'low', 4)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkInputDropped, 220, 'low', 4)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkOutputDropped, 220, 'low', 4)} />
          </Col>
        </Row>
      </Card>

      {/* CPU 限流 */}
      <Card size="small" title="CPU 限流" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={24}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuThrottleRate, 220, 'low', 5)} />
          </Col>
          <Col span={24}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuThrottleTime, 220, 'low', 5)} />
          </Col>
        </Row>
      </Card>

    </div>
  );
};

export default MonitoringTab;

