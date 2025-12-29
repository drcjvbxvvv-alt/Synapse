import React, { useState } from 'react';
import { Card, Row, Col, Space, Switch, Button, DatePicker, Popover, Divider, Typography } from 'antd';
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
    ],
  },
];

interface PodMonitoringTabProps {
  clusterId: string;
  clusterName?: string;
  namespace: string;
  podName: string;
}

// Grafana Dashboard 配置
const DASHBOARD_UID = 'kubepolaris-pod-detail';

// Panel ID 映射（对应 Grafana Dashboard 中的 Panel）
const PANEL_IDS = {
  // 资源使用（按容器）
  cpuUsage: 2,          // CPU 使用率 (按容器)
  memoryUsage: 6,       // Memory 使用率 (按容器)
  ioRead: 18,           // IO Read (按容器)
  ioWrite: 19,          // IO Write (按容器)
  
  // 容器状态
  cpuLimit: 28,           // CPU 核限制
  memoryLimit: 30,        // 内存限制
  containerRestarts: 38,  // 容器重启次数 (stat)
  healthCheckFailed: 36,  // 健康检查失败次数
  containerRestartsChart: 39, // 容器重启情况 (按容器)
  
  // 网络流量
  networkIncoming: 4,     // Network Incoming
  networkOutgoing: 14,    // Network Outgoing
  networkInputPps: 15,    // Network Input PPS
  networkOutputPps: 16,   // Network Output PPS
  
  // 系统资源
  fileDescriptors: 22,      // 文件句柄打开数 (按容器)
  runningThreads: 23,       // Running Threads (按容器)
  networkInputDropped: 12,  // Network Input Dropped
  networkOutputDropped: 20, // Network Output Dropped
  
  // CPU 限流
  cpuThrottleRate: 46,    // CPU限流比例 (按容器)
  cpuThrottleTime: 32,    // CPU节流时间 (按容器)
};

const PodMonitoringTab: React.FC<PodMonitoringTabProps> = ({
  clusterId,
  clusterName,
  namespace,
  podName,
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
      namespace: namespace,
      podname: podName,
      Interface: 'eth0',
      Intervals: '2m',
    } as Record<string, string>,
    from: getFromTime(),
    to: getToTime(),
    refresh: getRefreshInterval(),
    height,
    showToolbar: false,
    theme: 'light' as const,
    key: `${panelId}-${refreshKey}-${clusterId}-${podName}`,
    priority,
    loadDelay: batchIndex * 300,
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
      {/* 时间控制栏 */}
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
          <Button
            icon={<ReloadOutlined />}
            onClick={handleRefresh}
          >
            刷新
          </Button>
        </Space>
      </div>

      {/* 资源使用 - 第一批加载 (高优先级) */}
      <Card size="small" title="资源使用 (按容器)" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuUsage, 220, 'high', 0)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.memoryUsage, 220, 'high', 0)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.ioRead, 220, 'high', 0)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.ioWrite, 220, 'high', 0)} />
          </Col>
        </Row>
      </Card>

      {/* 容器状态 - 第二批加载 */}
      <Card size="small" title="容器状态" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={4}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuLimit, 150, 'normal', 1)} />
          </Col>
          <Col span={4}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.memoryLimit, 150, 'normal', 1)} />
          </Col>
          <Col span={4}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.containerRestarts, 150, 'normal', 1)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.healthCheckFailed, 150, 'normal', 2)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.containerRestartsChart, 150, 'normal', 2)} />
          </Col>
        </Row>
      </Card>

      {/* 网络流量 - 第三批加载 */}
      <Card size="small" title="网络流量" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkIncoming, 220, 'normal', 3)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkOutgoing, 220, 'normal', 3)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkInputPps, 220, 'normal', 4)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkOutputPps, 220, 'normal', 4)} />
          </Col>
        </Row>
      </Card>

      {/* 系统资源 - 第四批加载 */}
      <Card size="small" title="系统资源" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.fileDescriptors, 220, 'low', 5)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.runningThreads, 220, 'low', 5)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkInputDropped, 220, 'low', 5)} />
          </Col>
          <Col span={6}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.networkOutputDropped, 220, 'low', 5)} />
          </Col>
        </Row>
      </Card>

      {/* CPU 限流 - 第五批加载 */}
      <Card size="small" title="CPU 限流">
        <Row gutter={[16, 16]}>
          <Col span={12}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuThrottleRate, 220, 'low', 6)} />
          </Col>
          <Col span={12}>
            <GrafanaPanel {...getPanelProps(PANEL_IDS.cpuThrottleTime, 220, 'low', 6)} />
          </Col>
        </Row>
      </Card>
    </div>
  );
};

export default PodMonitoringTab;

