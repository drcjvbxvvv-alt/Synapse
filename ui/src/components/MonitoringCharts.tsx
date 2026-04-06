import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Card, Row, Col, Statistic, Select, Button, Space, Alert, Switch, Skeleton } from 'antd';
import { Line, Area } from '@ant-design/plots';
import { ReloadOutlined } from '@ant-design/icons';
import api from '../utils/api';
import GrafanaPanel from './GrafanaPanel';
import { generateDataSourceUID } from '../config/grafana.config';
import { useGrafanaUrl } from '../hooks/useGrafanaUrl';

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

interface NetworkPPS {
  in: MetricSeries;
  out: MetricSeries;
}

interface NetworkDrops {
  receive: MetricSeries;
  transmit: MetricSeries;
}

interface DiskIOPS {
  read: MetricSeries;
  write: MetricSeries;
}

interface DiskThroughput {
  read: MetricSeries;
  write: MetricSeries;
}

interface MultiSeriesDataPoint {
  timestamp: number;
  values: { [podName: string]: number };
}

interface MultiSeriesMetric {
  series: MultiSeriesDataPoint[];
}

interface ClusterOverview {
  total_cpu_cores: number;
  total_memory: number;
  worker_nodes: number;
  cpu_usage_rate?: MetricSeries;
  memory_usage_rate?: MetricSeries;
  max_pods: number;
  created_pods: number;
  available_pods: number;
  pod_usage_rate: number;
  etcd_has_leader: boolean;
  apiserver_availability: number;
  cpu_request_ratio?: MetricSeries;
  cpu_limit_ratio?: MetricSeries;
  mem_request_ratio?: MetricSeries;
  mem_limit_ratio?: MetricSeries;
  apiserver_request_rate?: MetricSeries;
}

interface NodeMetricItem {
  node_name: string;
  cpu_usage_rate: number;
  memory_usage_rate: number;
  cpu_cores: number;
  total_memory: number;
  status: string;
}

interface ClusterMetricsData {
  cpu?: MetricSeries;
  memory?: MetricSeries;
  network?: NetworkMetrics;
  storage?: MetricSeries;
  pods?: PodMetrics;
  // Pod 級別的擴充套件指標
  cpu_request?: MetricSeries;
  cpu_limit?: MetricSeries;
  memory_request?: MetricSeries;
  memory_limit?: MetricSeries;
  probe_failures?: MetricSeries;
  container_restarts?: MetricSeries;
  network_pps?: NetworkPPS;
  threads?: MetricSeries;
  network_drops?: NetworkDrops;
  cpu_throttling?: MetricSeries;
  cpu_throttling_time?: MetricSeries;
  disk_iops?: DiskIOPS;
  disk_throughput?: DiskThroughput;
  cpu_usage_absolute?: MetricSeries;
  memory_usage_bytes?: MetricSeries;
  oom_kills?: MetricSeries;
  // 叢集級別監控指標
  cluster_overview?: ClusterOverview;
  node_list?: NodeMetricItem[];
  // 工作負載多Pod監控指標（顯示多條曲線）
  cpu_multi?: MultiSeriesMetric;
  memory_multi?: MultiSeriesMetric;
  container_restarts_multi?: MultiSeriesMetric;
  oom_kills_multi?: MultiSeriesMetric;
  probe_failures_multi?: MultiSeriesMetric;
  network_pps_multi?: MultiSeriesMetric;
  threads_multi?: MultiSeriesMetric;
  network_drops_multi?: MultiSeriesMetric;
  cpu_throttling_multi?: MultiSeriesMetric;
  cpu_throttling_time_multi?: MultiSeriesMetric;
  disk_iops_multi?: MultiSeriesMetric;
  disk_throughput_multi?: MultiSeriesMetric;
}

interface MonitoringChartsProps {
  clusterId: string;
  clusterName?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  workloadName?: string;
  type: 'cluster' | 'node' | 'pod' | 'workload';
  lazyLoad?: boolean; // 是否懶載入，預設 false
}

const MonitoringCharts: React.FC<MonitoringChartsProps> = ({
  clusterId,
  clusterName,
  nodeName,
  namespace,
  podName,
  workloadName,
  type,
  lazyLoad = false,
}) => {
  const { grafanaUrl } = useGrafanaUrl();
  const [metrics, setMetrics] = useState<ClusterMetricsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [timeRange, setTimeRange] = useState('1h');
  const [step, setStep] = useState('15s');
  const [autoRefresh, setAutoRefresh] = useState(false); // 預設關閉自動重新整理
  const [hasLoaded, setHasLoaded] = useState(false); // 是否已載入過資料
  const metricsCacheRef = useRef<{ key: string; data: ClusterMetricsData; timestamp: number } | null>(null);
  const CACHE_DURATION = 30000; // 快取30秒
  const intervalRef = useRef<NodeJS.Timeout | null>(null);

  // 生成快取鍵
  const cacheKey = useMemo(() => {
    return `${clusterId}-${type}-${timeRange}-${step}-${clusterName || ''}-${nodeName || ''}-${namespace || ''}-${podName || ''}-${workloadName || ''}`;
  }, [clusterId, type, timeRange, step, clusterName, nodeName, namespace, podName, workloadName]);

  // 檢查快取
  const getCachedData = useCallback(() => {
    if (metricsCacheRef.current && metricsCacheRef.current.key === cacheKey) {
      const now = Date.now();
      if (now - metricsCacheRef.current.timestamp < CACHE_DURATION) {
        return metricsCacheRef.current.data;
      }
    }
    return null;
  }, [cacheKey]);

  const fetchMetrics = useCallback(async (forceRefresh = false) => {
    // 檢查快取
    if (!forceRefresh) {
      const cachedData = getCachedData();
      if (cachedData) {
        setMetrics(cachedData);
        setLoading(false);
        return;
      }
    }

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
        case 'workload':
          url = `/clusters/${clusterId}/workloads/${namespace}/${workloadName}/metrics`;
          break;
      }

      const response = await api.get(`${url}?${params.toString()}`);
      const data = response.data;
      setMetrics(data);
      
      // 更新快取
      metricsCacheRef.current = {
        key: cacheKey,
        data: data,
        timestamp: Date.now(),
      };
      
      setHasLoaded(true);
    } catch (error) {
      console.error('獲取監控資料失敗:', error);
    } finally {
      setLoading(false);
    }
  }, [clusterId, timeRange, step, clusterName, nodeName, namespace, podName, workloadName, type, cacheKey, getCachedData]);

  useEffect(() => {
    // 如果是懶載入模式且未載入過，延遲自動載入
    if (lazyLoad && !hasLoaded) {
      // 延遲自動載入，給使用者更好的體驗
      const timer = setTimeout(() => {
        // 檢查快取
        const cachedData = getCachedData();
        if (cachedData) {
          setMetrics(cachedData);
          setHasLoaded(true);
          return;
        }
        fetchMetrics();
      }, 100); // 100ms 後自動載入
      return () => clearTimeout(timer);
    }
    
    // 非懶載入模式或已載入過，正常載入
    if (!lazyLoad || hasLoaded) {
      // 檢查快取
      const cachedData = getCachedData();
      if (cachedData) {
        setMetrics(cachedData);
        setHasLoaded(true);
        return;
      }
      
      fetchMetrics();
    }
    
    // 只在開啟自動重新整理時設定定時器
    if (autoRefresh) {
      intervalRef.current = setInterval(() => fetchMetrics(true), 30000); // 30秒重新整理一次，強制重新整理
    }

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [clusterId, timeRange, step, clusterName, nodeName, namespace, podName, fetchMetrics, autoRefresh, lazyLoad, hasLoaded, getCachedData]);

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
            name: '數值',
            value: formatValue(datum.value, unit),
          };
        },
        title: (datum: { time: string }) => `時間: ${datum.time}`,
      },
      yAxis: {
        label: {
          formatter: (value: number) => formatValue(value, unit),
        },
      },
    };

    return <Line {...config} />;
  };

  // 渲染多時間序列圖表（多個Pod的曲線）
  const renderMultiSeriesChart = (data: MultiSeriesDataPoint[], unit: string = '') => {
    if (!data || data.length === 0) {
      return <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>暫無資料</div>;
    }

    // 轉換資料格式：將 {timestamp, values: {pod1: val1, pod2: val2}} 轉為 [{time, pod, value}, ...]
    const chartData: Array<{ time: string; pod: string; value: number }> = [];
    data.forEach(point => {
      const time = formatTimestamp(point.timestamp);
      Object.entries(point.values).forEach(([podName, value]) => {
        // 只新增有效的數值資料點
        if (value != null && typeof value === 'number' && !isNaN(value) && isFinite(value)) {
          chartData.push({
            time,
            pod: podName,
            value,
          });
        }
      });
    });

    const config = {
      data: chartData,
      xField: 'time',
      yField: 'value',
      colorField: 'pod',
      height: 300,
      smooth: true,
      point: {
        size: 0,
      },
      legend: {
        position: 'top' as const,
        maxRow: 3,
        layout: 'horizontal' as const,
      },
      yAxis: {
        label: {
          formatter: (value: string) => formatValue(parseFloat(value), unit),
        },
      },
    };

    return <Line {...config} />;
  };

  // Helper function to convert bytes to appropriate unit
  const convertBytesToUnit = (bytes: number): { value: number; unit: string } => {
    if (bytes >= 1024 * 1024 * 1024) {
      return { value: bytes / (1024 * 1024 * 1024), unit: 'GB' };
    } else if (bytes >= 1024 * 1024) {
      return { value: bytes / (1024 * 1024), unit: 'MB' };
    } else if (bytes >= 1024) {
      return { value: bytes / 1024, unit: 'KB' };
    }
    return { value: bytes, unit: 'B' };
  };

  const renderNetworkChart = (inData: DataPoint[], outData: DataPoint[], unit: string = '', inLabel: string = '入站', outLabel: string = '出站') => {
    let chartData;
    let yAxisSuffix = '';

    if (unit === 'bytes') {
      // Find max value to determine the best unit
      const maxValue = Math.max(
        ...inData.map(p => p.value),
        ...outData.map(p => p.value)
      );
      const { unit: bestUnit } = convertBytesToUnit(maxValue);
      yAxisSuffix = bestUnit;

      // Convert all data to the best unit
      const divisor = 
        bestUnit === 'GB' ? (1024 * 1024 * 1024) :
        bestUnit === 'MB' ? (1024 * 1024) :
        bestUnit === 'KB' ? 1024 : 1;

      chartData = inData.map((point, index) => ({
        time: formatTimestamp(point.timestamp),
        in: point.value / divisor,
        out: (outData[index]?.value || 0) / divisor,
        inRaw: point.value,
        outRaw: outData[index]?.value || 0,
        timestamp: point.timestamp,
      }));
    } else {
      chartData = inData.map((point, index) => ({
        time: formatTimestamp(point.timestamp),
        in: point.value,
        out: outData[index]?.value || 0,
        timestamp: point.timestamp,
      }));
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const config: any = {
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
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        formatter: (datum: any) => {
          if (unit === 'bytes') {
            return [
              {
                name: inLabel,
                value: formatValue(datum.inRaw, 'bytes'),
              },
              {
                name: outLabel,
                value: formatValue(datum.outRaw, 'bytes'),
              },
            ];
          } else {
            return [
              {
                name: inLabel,
                value: datum.in.toFixed(2),
              },
              {
                name: outLabel,
                value: datum.out.toFixed(2),
              },
            ];
          }
        },
        title: (datum: { time: string }) => `時間: ${datum.time}`,
      },
      yAxis: {
        label: {
          formatter: (value: string) => {
            const numValue = parseFloat(value);
            return yAxisSuffix ? `${numValue.toFixed(2)} ${yAxisSuffix}` : numValue.toFixed(2);
          },
        },
      },
    };

    return <Area {...config} />;
  };

  // 懶載入處理 - 顯示骨架屏，自動載入（透過 useEffect 觸發）
  if (lazyLoad && !hasLoaded && !loading) {
    return (
      <div style={{ padding: '24px' }}>
        <Card title="監控圖表">
          <Skeleton active paragraph={{ rows: 8 }} />
        </Card>
      </div>
    );
  }

  if (loading && !metrics) {
    return (
      <div style={{ padding: '24px' }}>
        <Card title="監控圖表">
          <Skeleton active paragraph={{ rows: 8 }} />
        </Card>
      </div>
    );
  }

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
          {/* 叢集概覽（僅在叢集型別時顯示） */}
          {type === 'cluster' && metrics.cluster_overview && (
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
                        value={formatValue(metrics.cluster_overview.total_memory, 'bytes')}
                        valueStyle={{ color: '#fa8c16' }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic
                        title="Pod 最大可建立數"
                        value={metrics.cluster_overview.max_pods}
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
                        value={metrics.cluster_overview.created_pods}
                        valueStyle={{ color: '#1890ff' }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic
                        title="Pod 可建立數"
                        value={metrics.cluster_overview.available_pods}
                        valueStyle={{ color: '#52c41a' }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic
                        title="Pod 使用率"
                        value={metrics.cluster_overview.pod_usage_rate.toFixed(2)}
                        suffix="%"
                        valueStyle={{ 
                          color: metrics.cluster_overview.pod_usage_rate > 80 ? '#cf1322' : '#3f8600' 
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
                        value={metrics.cluster_overview.etcd_has_leader ? 'YES' : 'NO'}
                        valueStyle={{ 
                          color: metrics.cluster_overview.etcd_has_leader ? '#52c41a' : '#cf1322' 
                        }}
                      />
                    </Col>
                    <Col span={8}>
                      <Statistic
                        title="ApiServer 近30天可用率"
                        value={metrics.cluster_overview.apiserver_availability.toFixed(4)}
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
                    {metrics.cluster_overview.cpu_request_ratio && (
                      <Col span={12}>
                        <Card size="small" title="CPU Request 比率">
                          <Statistic
                            value={metrics.cluster_overview.cpu_request_ratio.current}
                            suffix="%"
                            precision={2}
                            valueStyle={{ color: '#1890ff' }}
                          />
                          {renderChart(metrics.cluster_overview.cpu_request_ratio.series, '#1890ff', '%')}
                        </Card>
                      </Col>
                    )}
                    {metrics.cluster_overview.cpu_limit_ratio && (
                      <Col span={12}>
                        <Card size="small" title="CPU Limit 比率">
                          <Statistic
                            value={metrics.cluster_overview.cpu_limit_ratio.current}
                            suffix="%"
                            precision={2}
                            valueStyle={{ 
                              color: metrics.cluster_overview.cpu_limit_ratio.current > 100 ? '#cf1322' : '#52c41a' 
                            }}
                          />
                          {renderChart(metrics.cluster_overview.cpu_limit_ratio.series, '#52c41a', '%')}
                        </Card>
                      </Col>
                    )}
                    {metrics.cluster_overview.mem_request_ratio && (
                      <Col span={12}>
                        <Card size="small" title="記憶體 Request 比率">
                          <Statistic
                            value={metrics.cluster_overview.mem_request_ratio.current}
                            suffix="%"
                            precision={2}
                            valueStyle={{ color: '#fa8c16' }}
                          />
                          {renderChart(metrics.cluster_overview.mem_request_ratio.series, '#fa8c16', '%')}
                        </Card>
                      </Col>
                    )}
                    {metrics.cluster_overview.mem_limit_ratio && (
                      <Col span={12}>
                        <Card size="small" title="記憶體 Limit 比率">
                          <Statistic
                            value={metrics.cluster_overview.mem_limit_ratio.current}
                            suffix="%"
                            precision={2}
                            valueStyle={{ 
                              color: metrics.cluster_overview.mem_limit_ratio.current > 100 ? '#cf1322' : '#52c41a' 
                            }}
                          />
                          {renderChart(metrics.cluster_overview.mem_limit_ratio.series, '#722ed1', '%')}
                        </Card>
                      </Col>
                    )}
                  </Row>
                </Card>
              </Col>

              {/* ApiServer 請求量 */}
              {metrics.cluster_overview.apiserver_request_rate && (
                <Col span={24}>
                  <Card size="small" title="ApiServer 總請求量">
                    <Statistic
                      value={metrics.cluster_overview.apiserver_request_rate.current.toFixed(2)}
                      suffix="req/s"
                      valueStyle={{ color: '#1890ff' }}
                    />
                    {renderChart(metrics.cluster_overview.apiserver_request_rate.series, '#1890ff', '')}
                  </Card>
                </Col>
              )}

              {/* 叢集 CPU/記憶體使用率趨勢圖 */}
              {metrics.cluster_overview.cpu_usage_rate && (
                <Col span={12}>
                  <Card size="small" title="叢集 CPU 使用率">
                    <Statistic
                      value={metrics.cluster_overview.cpu_usage_rate.current}
                      suffix="%"
                      precision={2}
                      valueStyle={{ 
                        color: metrics.cluster_overview.cpu_usage_rate.current > 80 ? '#cf1322' : '#3f8600' 
                      }}
                    />
                    {renderChart(metrics.cluster_overview.cpu_usage_rate.series, '#1890ff', '%')}
                  </Card>
                </Col>
              )}
              
              {metrics.cluster_overview.memory_usage_rate && (
                <Col span={12}>
                  <Card size="small" title="叢集記憶體使用率">
                    <Statistic
                      value={metrics.cluster_overview.memory_usage_rate.current}
                      suffix="%"
                      precision={2}
                      valueStyle={{ 
                        color: metrics.cluster_overview.memory_usage_rate.current > 80 ? '#cf1322' : '#3f8600' 
                      }}
                    />
                    {renderChart(metrics.cluster_overview.memory_usage_rate.series, '#52c41a', '%')}
                  </Card>
                </Col>
              )}
            </>
          )}

          {/* Node 列表監控（僅在叢集型別時顯示） */}
          {type === 'cluster' && metrics.node_list && metrics.node_list.length > 0 && (
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

          {/* Pod/工作負載 資源規格 */}
          {(type === 'pod' || type === 'workload') && (metrics.cpu_request || metrics.cpu_limit || metrics.memory_request || metrics.memory_limit) && (
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
          {(type === 'pod' || type === 'workload') && metrics.cpu && (
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
                  {(type === 'pod' || type === 'workload') && metrics.cpu_usage_absolute && (
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
                {/* 工作負載型別顯示多Pod曲線，Pod型別顯示單條曲線 */}
                {type === 'workload' && metrics.cpu_multi ? (
                  renderMultiSeriesChart(metrics.cpu_multi.series, '%')
                ) : (
                  renderChart(metrics.cpu.series, '#1890ff', '%')
                )}
              </Card>
            </Col>
          )}

          {/* 記憶體使用率 */}
          {(type === 'pod' || type === 'workload') && metrics.memory && (
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
                  {(type === 'pod' || type === 'workload') && metrics.memory_usage_bytes && (
                    <Col span={12}>
                      <Statistic
                        title="實際使用"
                        value={formatValue(metrics.memory_usage_bytes.current, 'bytes')}
                        valueStyle={{ color: '#52c41a' }}
                      />
                    </Col>
                  )}
                </Row>
                {/* 工作負載型別顯示多Pod曲線，Pod型別顯示單條曲線 */}
                {type === 'workload' && metrics.memory_multi ? (
                  renderMultiSeriesChart(metrics.memory_multi.series, '%')
                ) : (
                  renderChart(metrics.memory.series, '#52c41a', '%')
                )}
              </Card>
            </Col>
          )}

          {/* 容器重啟次數 */}
          {(type === 'pod' || type === 'workload') && metrics.container_restarts && (
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
          {(type === 'pod' || type === 'workload') && metrics.oom_kills && (
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
          {(type === 'pod' || type === 'workload') && metrics.probe_failures && (
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

          {/* 網路 PPS */}
          {(type === 'pod' || type === 'workload') && metrics.network_pps && (
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
          {(type === 'pod' || type === 'workload') && metrics.disk_iops && (
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
          {(type === 'pod' || type === 'workload') && metrics.disk_throughput && (
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
          {(type === 'pod' || type === 'workload') && metrics.threads && (
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
          {(type === 'pod' || type === 'workload') && (metrics.cpu_throttling || metrics.cpu_throttling_time) && (
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
          {(type === 'pod' || type === 'workload') && metrics.network_drops && (
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
        </Row>
      </Card>
    </div>
  );
};

export default MonitoringCharts;