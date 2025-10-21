import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Card, Row, Col, Statistic, Select, Button, Space, Spin, Alert } from 'antd';
import { Line, Area } from '@ant-design/plots';
import { ReloadOutlined } from '@ant-design/icons';
import api from '../utils/api';

const { Option } = Select;

interface DataPoint {
  timestamp: number;
  value: number;
}

interface MetricSeries {
  current: number;
  series: DataPoint[];
}

interface NetworkMetrics {
  in: MetricSeries;
  out: MetricSeries;
}

interface PodMetrics {
  total: number;
  running: number;
  pending: number;
  failed: number;
}

interface ClusterMetricsData {
  cpu?: MetricSeries;
  memory?: MetricSeries;
  network?: NetworkMetrics;
  storage?: MetricSeries;
  pods?: PodMetrics;
}

interface MonitoringChartsProps {
  clusterId: string;
  clusterName?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  type: 'cluster' | 'node' | 'pod';
}

const MonitoringCharts: React.FC<MonitoringChartsProps> = ({
  clusterId,
  clusterName,
  nodeName,
  namespace,
  podName,
  type,
}) => {
  const [metrics, setMetrics] = useState<ClusterMetricsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [timeRange, setTimeRange] = useState('1h');
  const [step, setStep] = useState('1m');
  const intervalRef = useRef<NodeJS.Timeout | null>(null);

  const fetchMetrics = useCallback(async () => {
    try {
      setLoading(true);
      let url = '';
      const params = new URLSearchParams({
        range: timeRange,
        step: step,
      });

      if (clusterName) {
        params.append('clusterName', clusterName);
      }

      switch (type) {
        case 'cluster':
          url = `/clusters/${clusterId}/monitoring/metrics`;
          break;
        case 'node':
          url = `/clusters/${clusterId}/nodes/${nodeName}/metrics`;
          break;
        case 'pod':
          url = `/clusters/${clusterId}/pods/${namespace}/${podName}/metrics`;
          break;
      }

      const response = await api.get(`${url}?${params.toString()}`);
      setMetrics(response.data.data);
    } catch (error) {
      console.error('获取监控数据失败:', error);
    } finally {
      setLoading(false);
    }
  }, [clusterId, timeRange, step, clusterName, nodeName, namespace, podName, type]);

  useEffect(() => {
    fetchMetrics();
    
    // 设置定时刷新
    intervalRef.current = setInterval(fetchMetrics, 30000); // 30秒刷新一次

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [clusterId, timeRange, step, clusterName, nodeName, namespace, podName, fetchMetrics]);

  const formatTimestamp = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleTimeString();
  };

  const formatValue = (value: number, unit: string = '') => {
    if (unit === '%') {
      return `${value.toFixed(2)}%`;
    }
    if (unit === 'bytes') {
      if (value >= 1024 * 1024 * 1024) {
        return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
      } else if (value >= 1024 * 1024) {
        return `${(value / (1024 * 1024)).toFixed(2)} MB`;
      } else if (value >= 1024) {
        return `${(value / 1024).toFixed(2)} KB`;
      }
      return `${value.toFixed(2)} B`;
    }
    return value.toFixed(2);
  };

  const renderChart = (data: DataPoint[], color: string, unit: string = '') => {
    const chartData = data.map(point => ({
      time: formatTimestamp(point.timestamp),
      value: point.value,
      timestamp: point.timestamp,
    }));

    const config = {
      data: chartData,
      xField: 'time',
      yField: 'value',
      height: 200,
      smooth: true,
      color: color,
      point: {
        size: 0,
      },
      tooltip: {
        formatter: (datum: { value: number; time: string }) => {
          return {
            name: '数值',
            value: formatValue(datum.value, unit),
          };
        },
        title: (datum: { time: string }) => `时间: ${datum.time}`,
      },
      yAxis: {
        label: {
          formatter: (value: number) => formatValue(value, unit),
        },
      },
    };

    return <Line {...config} />;
  };

  const renderNetworkChart = (inData: DataPoint[], outData: DataPoint[]) => {
    const chartData = inData.map((point, index) => ({
      time: formatTimestamp(point.timestamp),
      in: point.value,
      out: outData[index]?.value || 0,
      timestamp: point.timestamp,
    }));

    const config = {
      data: chartData,
      xField: 'time',
      yField: ['in', 'out'],
      height: 200,
      smooth: true,
      color: ['#1890ff', '#52c41a'],
      areaStyle: {
        fillOpacity: 0.6,
      },
      tooltip: {
        formatter: (datum: { in: number; out: number; time: string }) => {
          return [
            {
              name: '入站',
              value: formatValue(datum.in, 'bytes'),
            },
            {
              name: '出站',
              value: formatValue(datum.out, 'bytes'),
            },
          ];
        },
        title: (datum: { time: string }) => `时间: ${datum.time}`,
      },
      yAxis: {
        label: {
          formatter: (value: number) => formatValue(value, 'bytes'),
        },
      },
    };

    return <Area {...config} />;
  };

  if (loading && !metrics) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spin size="large" />
        <div style={{ marginTop: 16 }}>加载监控数据中...</div>
      </div>
    );
  }

  if (!metrics) {
    return (
      <Alert
        message="监控数据不可用"
        description="请检查监控配置是否正确，或监控数据源是否可用。"
        type="warning"
        showIcon
      />
    );
  }

  return (
    <div>
      <Card
        title="监控图表"
        extra={
          <Space>
            <Select
              value={timeRange}
              onChange={setTimeRange}
              style={{ width: 100 }}
            >
              <Option value="1h">1小时</Option>
              <Option value="6h">6小时</Option>
              <Option value="24h">24小时</Option>
              <Option value="7d">7天</Option>
            </Select>
            <Select
              value={step}
              onChange={setStep}
              style={{ width: 100 }}
            >
              <Option value="1m">1分钟</Option>
              <Option value="5m">5分钟</Option>
              <Option value="15m">15分钟</Option>
              <Option value="1h">1小时</Option>
            </Select>
            <Button
              icon={<ReloadOutlined />}
              onClick={fetchMetrics}
              loading={loading}
            >
              刷新
            </Button>
          </Space>
        }
      >
        <Row gutter={[16, 16]}>
          {/* CPU 使用率 */}
          {metrics.cpu && (
            <Col span={12}>
              <Card size="small" title="CPU 使用率">
                <Statistic
                  value={metrics.cpu.current}
                  suffix="%"
                  precision={2}
                  valueStyle={{ color: metrics.cpu.current > 80 ? '#cf1322' : '#3f8600' }}
                />
                {renderChart(metrics.cpu.series, '#1890ff', '%')}
              </Card>
            </Col>
          )}

          {/* 内存使用率 */}
          {metrics.memory && (
            <Col span={12}>
              <Card size="small" title="内存使用率">
                <Statistic
                  value={metrics.memory.current}
                  suffix="%"
                  precision={2}
                  valueStyle={{ color: metrics.memory.current > 80 ? '#cf1322' : '#3f8600' }}
                />
                {renderChart(metrics.memory.series, '#52c41a', '%')}
              </Card>
            </Col>
          )}

          {/* 网络流量 */}
          {metrics.network && (
            <Col span={24}>
              <Card size="small" title="网络流量">
                <Row gutter={16}>
                  <Col span={12}>
                    {/* genAI_main_start */}
                    <Statistic
                      title="入站流量"
                      value={formatValue(metrics.network.in.current, 'bytes')}
                      suffix="/s"
                      precision={2}
                    />
                    {/* genAI_main_end */}
                  </Col>
                  <Col span={12}>
                    {/* genAI_main_start */}
                    <Statistic
                      title="出站流量"
                      value={formatValue(metrics.network.out.current, 'bytes')}
                      suffix="/s"
                      precision={2}
                    />
                    {/* genAI_main_end */}
                  </Col>
                </Row>
                {renderNetworkChart(metrics.network.in.series, metrics.network.out.series)}
              </Card>
            </Col>
          )}

          {/* 存储使用率 */}
          {metrics.storage && (
            <Col span={12}>
              <Card size="small" title="存储使用率">
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

          {/* Pod 统计 */}
          {metrics.pods && (
            <Col span={12}>
              <Card size="small" title="Pod 状态">
                <Row gutter={16}>
                  <Col span={6}>
                    <Statistic
                      title="总数"
                      value={metrics.pods.total}
                      valueStyle={{ color: '#1890ff' }}
                    />
                  </Col>
                  <Col span={6}>
                    <Statistic
                      title="运行中"
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
                      title="失败"
                      value={metrics.pods.failed}
                      valueStyle={{ color: '#cf1322' }}
                    />
                  </Col>
                </Row>
              </Card>
            </Col>
          )}
        </Row>
      </Card>
    </div>
  );
};

export default MonitoringCharts;