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
  Alert,
  DatePicker,
  Tooltip,
  Breadcrumb,
  Typography,
} from 'antd';
import {
  DollarOutlined,
  ClusterOutlined,
  WarningOutlined,
  ReloadOutlined,
  BarChartOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import dayjs from 'dayjs';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartTooltip,
  Legend,
  LineChart,
  Line,
  ResponsiveContainer,
} from 'recharts';
import { clusterService } from '../../services/clusterService';
import { CostService, type CostOverview, type TrendPoint } from '../../services/costService';
import type { Cluster } from '../../types';

const { Title } = Typography;

const COLORS = [
  '#4e79a7', '#f28e2b', '#e15759', '#76b7b2',
  '#59a14f', '#edc948', '#b07aa1', '#ff9da7',
];

interface ClusterCostRow {
  cluster: Cluster;
  overview: CostOverview | null;
  loading: boolean;
}

interface ClusterTrendRow {
  clusterId: string;
  clusterName: string;
  trend: TrendPoint[];
}

const GlobalCostInsights: React.FC = () => {
  const { t } = useTranslation(['cost', 'common']);
  const navigate = useNavigate();
  const [month, setMonth] = useState(dayjs().format('YYYY-MM'));
  const [rows, setRows] = useState<ClusterCostRow[]>([]);
  const [trendData, setTrendData] = useState<ClusterTrendRow[]>([]);
  const [clustersLoading, setClustersLoading] = useState(false);

  const loadAll = useCallback(async () => {
    setClustersLoading(true);
    try {
      const res = await clusterService.getClusters();
      const clusters: Cluster[] = res.items ?? [];

      // Init rows with loading state
      setRows(clusters.map(c => ({ cluster: c, overview: null, loading: true })));

      // Fan-out: fetch overview for each cluster in parallel
      const overviewResults = await Promise.allSettled(
        clusters.map(c => CostService.getOverview(String(c.id), month))
      );
      setRows(clusters.map((c, i) => ({
        cluster: c,
        overview: overviewResults[i].status === 'fulfilled' ? overviewResults[i].value : null,
        loading: false,
      })));

      // Fan-out: fetch trend for each cluster in parallel
      const trendResults = await Promise.allSettled(
        clusters.map(c => CostService.getTrend(String(c.id), 6))
      );
      setTrendData(clusters.map((c, i) => ({
        clusterId: String(c.id),
        clusterName: c.name,
        trend: trendResults[i].status === 'fulfilled' ? (trendResults[i].value ?? []) : [],
      })));
    } finally {
      setClustersLoading(false);
    }
  }, [month]);

  useEffect(() => { loadAll(); }, [loadAll]);

  // ── Aggregated stats ──────────────────────────────────────────────────
  const configuredRows = rows.filter(r => r.overview && r.overview.snapshot_count > 0);
  const totalSpend = configuredRows.reduce((sum, r) => sum + (r.overview?.total_cost ?? 0), 0);
  const topCluster = configuredRows.reduce<ClusterCostRow | null>(
    (top, r) => (!top || (r.overview?.total_cost ?? 0) > (top.overview?.total_cost ?? 0)) ? r : top,
    null
  );

  // ── Bar chart: cluster cost comparison ───────────────────────────────
  const barData = configuredRows.map(r => ({
    name: r.cluster.name,
    cost: Number((r.overview?.total_cost ?? 0).toFixed(4)),
    currency: r.overview?.currency ?? '',
  }));

  // ── Line chart: cross-cluster trend ──────────────────────────────────
  // Collect all months across all clusters
  const allMonths = Array.from(
    new Set(trendData.flatMap(ct => ct.trend.map(p => p.month)))
  ).sort();

  const lineData = allMonths.map(m => {
    const point: Record<string, string | number> = { month: m };
    trendData.forEach(ct => {
      const found = ct.trend.find(p => p.month === m);
      point[ct.clusterName] = found ? Number(found.total.toFixed(4)) : 0;
    });
    return point;
  });

  // ── Table columns ─────────────────────────────────────────────────────
  const columns = [
    {
      title: t('cost:insights.table.cluster'),
      dataIndex: ['cluster', 'name'],
      key: 'cluster',
      render: (_: string, row: ClusterCostRow) => (
        <Button
          type="link"
          style={{ padding: 0 }}
          onClick={() => navigate(`/clusters/${row.cluster.id}/cost-insights`)}
        >
          {row.cluster.name} <RightOutlined style={{ fontSize: 10 }} />
        </Button>
      ),
    },
    {
      title: t('cost:insights.table.totalCost'),
      key: 'totalCost',
      sorter: (a: ClusterCostRow, b: ClusterCostRow) =>
        (a.overview?.total_cost ?? -1) - (b.overview?.total_cost ?? -1),
      render: (_: unknown, row: ClusterCostRow) => {
        if (row.loading) return <span style={{ color: '#bfbfbf' }}>…</span>;
        if (!row.overview || row.overview.snapshot_count === 0)
          return <Tag>{t('cost:insights.table.notConfigured')}</Tag>;
        return (
          <span style={{ fontWeight: 600 }}>
            {row.overview.currency} {row.overview.total_cost.toFixed(4)}
          </span>
        );
      },
    },
    {
      title: t('cost:insights.table.topNamespace'),
      key: 'topNamespace',
      render: (_: unknown, row: ClusterCostRow) =>
        row.overview?.top_namespace
          ? <Tag color="blue">{row.overview.top_namespace}</Tag>
          : '—',
    },
    {
      title: t('cost:insights.table.wastePercent'),
      key: 'wastePercent',
      render: (_: unknown, row: ClusterCostRow) => {
        const pct = row.overview?.waste_percent;
        if (pct == null) return '—';
        return (
          <Tag color={pct > 30 ? 'red' : pct > 10 ? 'orange' : 'green'}>
            {pct.toFixed(1)}%
          </Tag>
        );
      },
    },
    {
      title: t('cost:insights.table.snapshotDays'),
      key: 'snapshotDays',
      render: (_: unknown, row: ClusterCostRow) =>
        row.overview?.snapshot_count != null ? row.overview.snapshot_count : '—',
    },
    {
      title: t('cost:insights.table.currency'),
      key: 'currency',
      render: (_: unknown, row: ClusterCostRow) => row.overview?.currency ?? '—',
    },
    {
      title: '',
      key: 'action',
      render: (_: unknown, row: ClusterCostRow) => (
        <Tooltip title={t('cost:menu')}>
          <Button
            size="small"
            icon={<BarChartOutlined />}
            onClick={() => navigate(`/clusters/${row.cluster.id}/cost-insights`)}
          />
        </Tooltip>
      ),
    },
  ];

  const hasAnyData = configuredRows.length > 0;

  return (
    <div>
      <Breadcrumb
        items={[
          { title: <Link to="/">{t('common:breadcrumb.home', '首頁')}</Link> },
          { title: t('cost:insights.title') },
        ]}
        style={{ marginBottom: 16 }}
      />

      <div style={{ marginBottom: 24, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div>
          <Title level={3} style={{ margin: 0 }}>
            <DollarOutlined style={{ marginRight: 8 }} />
            {t('cost:insights.title')}
          </Title>
          <div style={{ color: '#8c8c8c', marginTop: 4 }}>{t('cost:insights.subtitle')}</div>
        </div>
        <Space>
          <DatePicker.MonthPicker
            value={dayjs(month, 'YYYY-MM')}
            onChange={v => v && setMonth(v.format('YYYY-MM'))}
            allowClear={false}
            style={{ width: 130 }}
          />
          <Button icon={<ReloadOutlined />} onClick={loadAll} loading={clustersLoading}>
            {t('common:actions.refresh', '重新整理')}
          </Button>
        </Space>
      </div>

      <Alert
        type="info"
        showIcon
        message={t('cost:banner.title')}
        description={t('cost:banner.description')}
        style={{ marginBottom: 24 }}
      />

      {/* ── Summary Stats ── */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:insights.totalSpend')}
              value={totalSpend}
              precision={4}
              prefix={<DollarOutlined />}
              loading={clustersLoading}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:insights.clusterCount')}
              value={rows.length}
              prefix={<ClusterOutlined />}
              loading={clustersLoading}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:insights.configuredCount')}
              value={configuredRows.length}
              suffix={`/ ${rows.length}`}
              loading={clustersLoading}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:insights.topCluster')}
              value={topCluster?.cluster.name ?? '—'}
              prefix={<WarningOutlined style={{ color: '#faad14' }} />}
              loading={clustersLoading}
            />
          </Card>
        </Col>
      </Row>

      {!hasAnyData && !clustersLoading ? (
        <Card>
          <div style={{ textAlign: 'center', padding: '40px 0', color: '#8c8c8c' }}>
            {t('cost:insights.noData')}
          </div>
        </Card>
      ) : (
        <>
          {/* ── Bar chart: cluster cost comparison ── */}
          <Card title={t('cost:insights.comparison')} style={{ marginBottom: 24 }}>
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={barData} margin={{ top: 5, right: 20, left: 10, bottom: 40 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" angle={-20} textAnchor="end" interval={0} tick={{ fontSize: 12 }} />
                <YAxis />
                <RechartTooltip
                  formatter={(v, _, props) => [
                    `${props.payload?.currency ?? ''} ${v}`,
                    t('cost:table.estCost'),
                  ]}
                />
                <Bar dataKey="cost" fill="#4e79a7" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </Card>

          {/* ── Line chart: cross-cluster trend ── */}
          {lineData.length > 0 && (
            <Card title={t('cost:insights.trend')} style={{ marginBottom: 24 }}>
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={lineData} margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="month" />
                  <YAxis />
                  <RechartTooltip />
                  <Legend />
                  {trendData.map((ct, i) => (
                    <Line
                      key={ct.clusterId}
                      type="monotone"
                      dataKey={ct.clusterName}
                      stroke={COLORS[i % COLORS.length]}
                      strokeWidth={2}
                      dot={false}
                    />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </Card>
          )}
        </>
      )}

      {/* ── Cluster cost table ── */}
      <Card>
        <Table
          rowKey={r => String(r.cluster.id)}
          columns={columns}
          dataSource={rows}
          loading={clustersLoading}
          size="middle"
          pagination={false}
          scroll={{ x: 800 }}
        />
      </Card>
    </div>
  );
};

export default GlobalCostInsights;
