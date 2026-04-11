import React from 'react';
import {
  Button, Space, Row, Col, Card, Table, Progress, Tag, Alert,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  ScatterChart, Scatter, XAxis, YAxis, ZAxis, CartesianGrid,
  Tooltip as RechartTooltip, Cell, ResponsiveContainer,
} from 'recharts';
import type { NamespaceEfficiency } from '../../../services/costService';
import { COLORS } from '../constants';

interface EfficiencyTabProps {
  nsEfficiency: NamespaceEfficiency[];
  nsEffLoading: boolean;
  onRefresh: () => void;
}

export const EfficiencyTab: React.FC<EfficiencyTabProps> = ({
  nsEfficiency,
  nsEffLoading,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  const scatterData = nsEfficiency.map(n => ({
    ...n,
    cpu_efficiency_pct: +(n.cpu_efficiency * 100).toFixed(1),
    mem_efficiency_pct: +(n.memory_efficiency * 100).toFixed(1),
  }));

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={nsEffLoading}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      {nsEfficiency.length > 0 && !nsEfficiency[0].has_metrics && (
        <Alert
          type="info"
          showIcon
          message={t('cost:occupancy.efficiencyNoMetrics')}
          style={{ marginBottom: 16 }}
        />
      )}

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} lg={14}>
          <Card title={t('cost:occupancy.nsEfficiency')} size="small">
            <ResponsiveContainer width="100%" height={320}>
              <ScatterChart margin={{ top: 20, right: 20, bottom: 20, left: 20 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="cpu_efficiency_pct"
                  name="CPU 效率"
                  type="number"
                  domain={[0, 100]}
                  unit="%"
                  label={{ value: 'CPU 效率 %', position: 'insideBottom', offset: -10 }}
                />
                <YAxis
                  dataKey="mem_efficiency_pct"
                  name="記憶體效率"
                  type="number"
                  domain={[0, 100]}
                  unit="%"
                  label={{ value: '記憶體效率 %', angle: -90, position: 'insideLeft' }}
                />
                <ZAxis dataKey="cpu_occupancy_percent" range={[40, 400]} name="CPU 佔用" unit="%" />
                <RechartTooltip
                  cursor={{ strokeDasharray: '3 3' }}
                  formatter={(value, name) => [`${Number(value).toFixed(1)}%`, name]}
                  content={({ active, payload }) => {
                    if (!active || !payload?.length) return null;
                    const d = payload[0].payload;
                    return (
                      <div style={{ background: '#fff', border: '1px solid #d9d9d9', padding: '8px 12px', borderRadius: 4 }}>
                        <p style={{ margin: 0, fontWeight: 600 }}>{d.namespace}</p>
                        <p style={{ margin: 0 }}>CPU 效率：{(d.cpu_efficiency_pct ?? 0).toFixed(1)}%</p>
                        <p style={{ margin: 0 }}>記憶體效率：{(d.mem_efficiency_pct ?? 0).toFixed(1)}%</p>
                        <p style={{ margin: 0 }}>CPU 佔用：{(d.cpu_occupancy_percent ?? 0).toFixed(1)}%</p>
                      </div>
                    );
                  }}
                />
                <Scatter name="Namespace" data={scatterData}>
                  {nsEfficiency.map((_, idx) => (
                    <Cell key={idx} fill={COLORS[idx % COLORS.length]} />
                  ))}
                </Scatter>
              </ScatterChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title={t('cost:occupancy.quadrantLegend')} size="small" style={{ height: '100%' }}>
            <div style={{ padding: 8, fontSize: 13, lineHeight: 2 }}>
              <div><Tag color="red">❌</Tag> {t('cost:occupancy.quadrantHighOccupancyLowEff')}</div>
              <div><Tag color="green">✅</Tag> {t('cost:occupancy.quadrantHighOccupancyHighEff')}</div>
              <div><Tag color="orange">⚠️</Tag> {t('cost:occupancy.quadrantLowOccupancyLowEff')}</div>
              <div><Tag color="blue">💡</Tag> {t('cost:occupancy.quadrantLowOccupancyHighEff')}</div>
              <div style={{ marginTop: 12, color: '#888' }}>
                {t('cost:occupancy.bubbleDesc')}
              </div>
            </div>
          </Card>
        </Col>
      </Row>

      <Table
        rowKey="namespace"
        loading={nsEffLoading}
        dataSource={nsEfficiency}
        size="small"
        scroll={{ x: 1100 }}
        pagination={{ pageSize: 20 }}
        columns={[
          { title: '命名空間', dataIndex: 'namespace', key: 'namespace', fixed: 'left', width: 140 },
          {
            title: 'CPU 申請 (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_request', width: 110,
            render: (v: number) => v.toFixed(0), sorter: (a, b) => a.cpu_request_millicores - b.cpu_request_millicores,
          },
          {
            title: 'CPU 使用 (m)', dataIndex: 'cpu_usage_millicores', key: 'cpu_usage', width: 110,
            render: (v: number, r: NamespaceEfficiency) => r.has_metrics ? v.toFixed(0) : <Tag>N/A</Tag>,
          },
          {
            title: 'CPU 效率', dataIndex: 'cpu_efficiency', key: 'cpu_eff', width: 170,
            render: (v: number, r: NamespaceEfficiency) => r.has_metrics ? (
              <Progress percent={+(v * 100).toFixed(1)} size="small"
                status={v < 0.2 ? 'exception' : v < 0.5 ? 'active' : 'normal'}
                format={p => `${p}%`} style={{ width: 130 }} />
            ) : <Tag>需要 Prometheus</Tag>,
            sorter: (a, b) => a.cpu_efficiency - b.cpu_efficiency,
          },
          {
            title: '記憶體申請 (MiB)', dataIndex: 'memory_request_mib', key: 'mem_request', width: 130,
            render: (v: number) => v.toFixed(0),
          },
          {
            title: '記憶體使用 (MiB)', dataIndex: 'memory_usage_mib', key: 'mem_usage', width: 130,
            render: (v: number, r: NamespaceEfficiency) => r.has_metrics ? v.toFixed(0) : <Tag>N/A</Tag>,
          },
          {
            title: '記憶體效率', dataIndex: 'memory_efficiency', key: 'mem_eff', width: 170,
            render: (v: number, r: NamespaceEfficiency) => r.has_metrics ? (
              <Progress percent={+(v * 100).toFixed(1)} size="small"
                status={v < 0.2 ? 'exception' : v < 0.5 ? 'active' : 'normal'}
                format={p => `${p}%`} style={{ width: 130 }} />
            ) : <Tag>需要 Prometheus</Tag>,
          },
          { title: 'Pod 數', dataIndex: 'pod_count', key: 'pod_count', width: 80 },
        ]}
      />
    </div>
  );
};
