import React, { useState, useEffect, useCallback } from 'react';
import { Card, Row, Col, Statistic, Segmented, Button, Alert, Spin } from 'antd';
import { Line } from '@ant-design/plots';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import EmptyState from './EmptyState';
import api from '../utils/api';

interface DataPoint {
  timestamp: number;
  value: number;
}

interface MetricSeries {
  current: number;
  series: DataPoint[];
}

interface WorkloadMetricsData {
  cpu?: MetricSeries;
  memory?: MetricSeries;
  cpu_usage_absolute?: MetricSeries;
  memory_usage_bytes?: MetricSeries;
}

interface Props {
  clusterId: string;
  namespace: string;
  name: string;
  /** API path segment: "deployments" | "statefulsets" | "daemonsets" | "rollouts" */
  workloadKind: string;
}

const TIME_RANGES = [
  { label: '15m', value: '15m', step: '15s' },
  { label: '1h',  value: '1h',  step: '1m'  },
  { label: '3h',  value: '3h',  step: '2m'  },
  { label: '24h', value: '24h', step: '15m' },
];

const formatTimestamp = (ts: number) =>
  new Date(ts * 1000).toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit' });

const formatBytes = (bytes: number) => {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes.toFixed(0)} B`;
};

const buildChartData = (series: DataPoint[] | undefined) => {
  if (!series || series.length === 0) return [];
  return series.map(p => ({ time: formatTimestamp(p.timestamp), value: p.value }));
};

const lineConfig = (data: ReturnType<typeof buildChartData>, color: string, yFormatter: (v: number) => string) => ({
  data,
  xField: 'time',
  yField: 'value',
  height: 200,
  smooth: true,
  color,
  point: { size: 0 },
  tooltip: {
    formatter: (d: { value: number; time: string }) => ({ name: '數值', value: yFormatter(d.value) }),
  },
  yAxis: { label: { formatter: yFormatter } },
  animation: false,
});

const WorkloadMetricsChart: React.FC<Props> = ({ clusterId, namespace, name, workloadKind }) => {
  const { t } = useTranslation(['common']);
  const [metrics, setMetrics] = useState<WorkloadMetricsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState('1h');
  const [noPrometheus, setNoPrometheus] = useState(false);

  const currentRange = TIME_RANGES.find(r => r.value === range) ?? TIME_RANGES[1];

  const fetchMetrics = useCallback(async () => {
    setLoading(true);
    setNoPrometheus(false);
    try {
      const url = `/clusters/${clusterId}/${workloadKind}/${namespace}/${name}/metrics`;
      const res = await api.get<WorkloadMetricsData>(url, {
        params: { range: currentRange.value, step: currentRange.step },
      });
      const data = res.data;
      // Detect unconfigured Prometheus: both cpu and memory are absent or empty series
      if (!data.cpu?.series?.length && !data.memory?.series?.length) {
        setNoPrometheus(true);
      }
      setMetrics(data);
    } catch {
      setNoPrometheus(true);
    } finally {
      setLoading(false);
    }
  }, [clusterId, workloadKind, namespace, name, currentRange.value, currentRange.step]);

  useEffect(() => {
    fetchMetrics();
  }, [fetchMetrics]);

  const cpuData    = buildChartData(metrics?.cpu?.series);
  const memData    = buildChartData(metrics?.memory?.series);
  const cpuCurrent = metrics?.cpu?.current ?? 0;
  const memCurrent = metrics?.memory?.current ?? 0;
  const cpuCores   = metrics?.cpu_usage_absolute?.current ?? 0;
  const memBytes   = metrics?.memory_usage_bytes?.current ?? 0;

  return (
    <Card
      title="效能指標"
      extra={
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <Segmented
            size="small"
            options={TIME_RANGES.map(r => ({ label: r.label, value: r.value }))}
            value={range}
            onChange={v => setRange(v as string)}
          />
          <Button size="small" icon={<ReloadOutlined />} onClick={fetchMetrics} loading={loading} />
        </div>
      }
      styles={{ body: { padding: '16px 24px' } }}
    >
      {noPrometheus && !loading && (
        <Alert
          type="warning"
          showIcon
          message="未設定 Prometheus 資料來源"
          description="請至「叢集設定 → 監控配置」完成 Prometheus 接入後，效能指標將自動顯示。"
          style={{ marginBottom: 16 }}
        />
      )}

      <Spin spinning={loading}>
        {/* 統計卡片 */}
        <Row gutter={16} style={{ marginBottom: 24 }}>
          <Col span={6}>
            <Statistic title="CPU 使用率" value={cpuCurrent.toFixed(1)} suffix="%" />
          </Col>
          <Col span={6}>
            <Statistic title="CPU 使用量" value={cpuCores.toFixed(3)} suffix="cores" />
          </Col>
          <Col span={6}>
            <Statistic title="記憶體使用率" value={memCurrent.toFixed(1)} suffix="%" />
          </Col>
          <Col span={6}>
            <Statistic title="記憶體使用量" value={formatBytes(memBytes)} />
          </Col>
        </Row>

        {/* CPU 圖表 */}
        <div style={{ marginBottom: 24 }}>
          <div style={{ fontWeight: 600, marginBottom: 8, color: '#262626' }}>CPU 使用率 (%)</div>
          {cpuData.length > 0 ? (
            <Line {...lineConfig(cpuData, '#1677ff', v => `${v.toFixed(2)}%`)} />
          ) : (
            <EmptyState style={{ height: 200, display: 'flex', flexDirection: 'column', justifyContent: 'center' }} />
          )}
        </div>

        {/* Memory 圖表 */}
        <div>
          <div style={{ fontWeight: 600, marginBottom: 8, color: '#262626' }}>記憶體使用率 (%)</div>
          {memData.length > 0 ? (
            <Line {...lineConfig(memData, '#52c41a', v => `${v.toFixed(2)}%`)} />
          ) : (
            <EmptyState style={{ height: 200, display: 'flex', flexDirection: 'column', justifyContent: 'center' }} />
          )}
        </div>
      </Spin>
    </Card>
  );
};

export default WorkloadMetricsChart;
