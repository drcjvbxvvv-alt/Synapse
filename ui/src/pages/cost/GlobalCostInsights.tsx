import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Tag,
  Button,
  Space,
  Progress,
  Breadcrumb,
  Typography,
  theme,
} from 'antd';
import {
  ClusterOutlined,
  ReloadOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartTooltip,
  ResponsiveContainer,
} from 'recharts';
import {
  ResourceService,
  type GlobalResourceOverview,
  type ClusterResourceSummary,
} from '../../services/costService';

const { Title } = Typography;

// ── Unified chart style (shared with CostDashboard) ──────────────────────────
const BAR_PROPS = {
  radius: [5, 5, 0, 0] as [number, number, number, number],
  maxBarSize: 44,
  isAnimationActive: true,
  animationBegin: 0,
  animationDuration: 800,
  animationEasing: 'ease-out' as const,
};

const TOOLTIP_STYLE = {
  contentStyle: {
    borderRadius: 10,
    border: 'none',
    boxShadow: '0 6px 24px rgba(0,0,0,0.10)',
    fontSize: 13,
  },
};

const GRID_STYLE = { stroke: '#f0f0f0', strokeDasharray: '4 4' };

const GlobalCostInsights: React.FC = () => {
  const { t } = useTranslation(['cost', 'common']);
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const [overview, setOverview] = useState<GlobalResourceOverview | null>(null);
  const [loading, setLoading] = useState(false);

  const loadOverview = useCallback(async () => {
    setLoading(true);
    try {
      const data = await ResourceService.getGlobalOverview();
      setOverview(data);
    } catch { /* ignore */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { loadOverview(); }, [loadOverview]);

  // ── Bar chart: cluster CPU occupancy ────────────────────────────────
  const barData = (overview?.clusters ?? [])
    .filter(c => c.informer_ready)
    .map(c => ({
      name: c.cluster_name,
      cpu: +c.cpu_occupancy_percent.toFixed(1),
      memory: +c.memory_occupancy_percent.toFixed(1),
    }));

  // ── Table columns ───────────────────────────────────────────────────
  const columns = [
    {
      title: t('cost:global.clusterName'),
      dataIndex: 'cluster_name',
      key: 'cluster_name',
      render: (name: string, row: ClusterResourceSummary) => (
        <Link to={`/clusters/${row.cluster_id}/cost-insights`}>{name}</Link>
      ),
    },
    {
      title: t('cost:occupancy.cpuOccupancy'),
      key: 'cpu_occupancy_percent',
      render: (_: unknown, row: ClusterResourceSummary) => row.informer_ready ? (
        <Progress
          percent={+row.cpu_occupancy_percent.toFixed(1)}
          size="small"
          style={{ width: 140 }}
          status={row.cpu_occupancy_percent > 80 ? 'exception' : 'normal'}
        />
      ) : <Tag color="default">N/A</Tag>,
    },
    {
      title: t('cost:occupancy.memOccupancy'),
      key: 'memory_occupancy_percent',
      render: (_: unknown, row: ClusterResourceSummary) => row.informer_ready ? (
        <Progress
          percent={+row.memory_occupancy_percent.toFixed(1)}
          size="small"
          style={{ width: 140 }}
          status={row.memory_occupancy_percent > 80 ? 'exception' : 'normal'}
        />
      ) : <Tag color="default">N/A</Tag>,
    },
    {
      title: t('cost:occupancy.nodeCount'),
      dataIndex: 'node_count',
      key: 'node_count',
    },
    {
      title: t('cost:occupancy.podCount'),
      dataIndex: 'pod_count',
      key: 'pod_count',
    },
    {
      title: t('common:actions.detail'),
      key: 'action',
      render: (_: unknown, row: ClusterResourceSummary) => (
        <Button
          type="link"
          size="small"
          icon={<RightOutlined />}
          onClick={() => navigate(`/clusters/${row.cluster_id}/cost-insights`)}
        >
          {t('cost:global.viewDetail')}
        </Button>
      ),
    },
  ];

  return (
    <div style={{ padding: token.paddingLG }}>
      <Breadcrumb
        items={[
          { title: t('common:menu.home') },
          { title: t('common:menu.costInsights') },
        ]}
        style={{ marginBottom: token.marginMD }}
      />
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: token.marginLG }}>
        <Title level={4} style={{ margin: 0 }}>
          <ClusterOutlined style={{ marginRight: token.marginXS }} />
          {t('common:menu.costInsights')}
        </Title>
        <Button icon={<ReloadOutlined />} onClick={loadOverview} loading={loading}>
          {t('common:actions.refresh')}
        </Button>
      </div>

      {/* Summary stats */}
      <Row gutter={token.marginMD} style={{ marginBottom: token.marginLG }}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.clusterCount')}
              value={overview?.cluster_count ?? 0}
              loading={loading}
              prefix={<ClusterOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.readyCount')}
              value={overview?.ready_count ?? 0}
              loading={loading}
              valueStyle={{ color: token.colorSuccess }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.avgCpuOccupancy')}
              value={overview?.avg_cpu_occupancy_percent ?? 0}
              precision={1}
              suffix="%"
              loading={loading}
              valueStyle={{ color: (overview?.avg_cpu_occupancy_percent ?? 0) > 80 ? token.colorError : token.colorSuccess }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.avgMemOccupancy')}
              value={overview?.avg_memory_occupancy_percent ?? 0}
              precision={1}
              suffix="%"
              loading={loading}
              valueStyle={{ color: (overview?.avg_memory_occupancy_percent ?? 0) > 80 ? token.colorError : token.colorSuccess }}
            />
          </Card>
        </Col>
      </Row>

      {/* Bar chart: cluster occupancy comparison */}
      <Row gutter={token.marginMD} style={{ marginBottom: token.marginLG }}>
        <Col xs={24} lg={14}>
          <Card title={t('cost:global.occupancyChart')} size="small">
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={barData} margin={{ top: 5, right: 20, left: 10, bottom: 40 }}>
                <CartesianGrid {...GRID_STYLE} />
                <XAxis dataKey="name" angle={-20} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                <YAxis unit="%" />
                <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${v}%`]} />
                <Bar dataKey="cpu" name={t('cost:occupancy.cpuOccupancy')} fill="#5B8FF9" {...BAR_PROPS} />
                <Bar dataKey="memory" name={t('cost:occupancy.memOccupancy')} fill="#5AD8A6" {...BAR_PROPS} animationBegin={100} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title={t('cost:global.clusterList')} size="small" style={{ height: '100%' }}>
            <div style={{ maxHeight: 260, overflowY: 'auto' }}>
              {(overview?.clusters ?? []).map(c => (
                <div key={c.cluster_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: `${token.paddingXS}px 0`, borderBottom: `1px solid ${token.colorBorder}` }}>
                  <Link to={`/clusters/${c.cluster_id}/cost-insights`} style={{ fontWeight: 500 }}>
                    {c.cluster_name}
                  </Link>
                  {c.informer_ready ? (
                    <Tag color={c.cpu_occupancy_percent > 80 ? 'red' : 'green'}>
                      CPU {c.cpu_occupancy_percent.toFixed(1)}%
                    </Tag>
                  ) : (
                    <Tag color="default">{t('cost:global.notReady')}</Tag>
                  )}
                </div>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      {/* Detail table */}
      <Card title={t('cost:global.clusterDetail')}>
        <Table
          rowKey="cluster_id"
          columns={columns}
          dataSource={overview?.clusters ?? []}
          loading={loading}
          size="small"
          scroll={{ x: 800 }}
          pagination={false}
        />
      </Card>
    </div>
  );
};

export default GlobalCostInsights;
