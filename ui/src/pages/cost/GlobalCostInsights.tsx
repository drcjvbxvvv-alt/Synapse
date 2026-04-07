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

const GlobalCostInsights: React.FC = () => {
  const { t } = useTranslation(['cost', 'common']);
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
      title: t('cost:global.clusterName', '叢集'),
      dataIndex: 'cluster_name',
      key: 'cluster_name',
      render: (name: string, row: ClusterResourceSummary) => (
        <Link to={`/clusters/${row.cluster_id}/cost-insights`}>{name}</Link>
      ),
    },
    {
      title: t('cost:occupancy.cpuOccupancy', 'CPU 佔用率'),
      key: 'cpu_occupancy_percent',
      render: (_: unknown, row: ClusterResourceSummary) => row.informer_ready ? (
        <Space>
          <Progress
            percent={+row.cpu_occupancy_percent.toFixed(1)}
            size="small"
            style={{ width: 100 }}
            status={row.cpu_occupancy_percent > 80 ? 'exception' : 'normal'}
          />
          <span>{row.cpu_occupancy_percent.toFixed(1)}%</span>
        </Space>
      ) : <Tag color="default">N/A</Tag>,
    },
    {
      title: t('cost:occupancy.memOccupancy', '記憶體佔用率'),
      key: 'memory_occupancy_percent',
      render: (_: unknown, row: ClusterResourceSummary) => row.informer_ready ? (
        <Space>
          <Progress
            percent={+row.memory_occupancy_percent.toFixed(1)}
            size="small"
            style={{ width: 100 }}
            status={row.memory_occupancy_percent > 80 ? 'exception' : 'normal'}
          />
          <span>{row.memory_occupancy_percent.toFixed(1)}%</span>
        </Space>
      ) : <Tag color="default">N/A</Tag>,
    },
    {
      title: t('cost:occupancy.nodeCount', '節點數'),
      dataIndex: 'node_count',
      key: 'node_count',
    },
    {
      title: t('cost:occupancy.podCount', 'Pod 數'),
      dataIndex: 'pod_count',
      key: 'pod_count',
    },
    {
      title: t('common:actions.detail', '詳情'),
      key: 'action',
      render: (_: unknown, row: ClusterResourceSummary) => (
        <Button
          type="link"
          size="small"
          icon={<RightOutlined />}
          onClick={() => navigate(`/clusters/${row.cluster_id}/cost-insights`)}
        >
          {t('cost:global.viewDetail', '查看')}
        </Button>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Breadcrumb
        items={[
          { title: t('menu.home', '首頁') },
          { title: t('menu.costInsights', '成本洞察') },
        ]}
        style={{ marginBottom: 16 }}
      />
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>
          <ClusterOutlined style={{ marginRight: 8 }} />
          {t('menu.costInsights', '成本洞察')}
        </Title>
        <Button icon={<ReloadOutlined />} onClick={loadOverview} loading={loading}>
          {t('common:actions.refresh', '重新整理')}
        </Button>
      </div>

      {/* Summary stats */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.clusterCount', '叢集總數')}
              value={overview?.cluster_count ?? 0}
              loading={loading}
              prefix={<ClusterOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.readyCount', 'Informer 就緒')}
              value={overview?.ready_count ?? 0}
              loading={loading}
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.avgCpuOccupancy', '平均 CPU 佔用率')}
              value={overview?.avg_cpu_occupancy_percent ?? 0}
              precision={1}
              suffix="%"
              loading={loading}
              valueStyle={{ color: (overview?.avg_cpu_occupancy_percent ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:global.avgMemOccupancy', '平均記憶體佔用率')}
              value={overview?.avg_memory_occupancy_percent ?? 0}
              precision={1}
              suffix="%"
              loading={loading}
              valueStyle={{ color: (overview?.avg_memory_occupancy_percent ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
            />
          </Card>
        </Col>
      </Row>

      {/* Bar chart: cluster occupancy comparison */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} lg={14}>
          <Card title={t('cost:global.occupancyChart', '各叢集資源佔用率')} size="small">
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={barData} margin={{ top: 5, right: 20, left: 10, bottom: 40 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" angle={-20} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                <YAxis unit="%" />
                <RechartTooltip formatter={(v) => [`${v}%`]} />
                <Bar dataKey="cpu" name="CPU 佔用率" fill="#4e79a7" />
                <Bar dataKey="memory" name="記憶體佔用率" fill="#f28e2b" />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title={t('cost:global.clusterList', '叢集列表')} size="small" style={{ height: '100%' }}>
            <div style={{ maxHeight: 260, overflowY: 'auto' }}>
              {(overview?.clusters ?? []).map(c => (
                <div key={c.cluster_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Link to={`/clusters/${c.cluster_id}/cost-insights`} style={{ fontWeight: 500 }}>
                    {c.cluster_name}
                  </Link>
                  {c.informer_ready ? (
                    <Tag color={c.cpu_occupancy_percent > 80 ? 'red' : 'green'}>
                      CPU {c.cpu_occupancy_percent.toFixed(1)}%
                    </Tag>
                  ) : (
                    <Tag color="default">未就緒</Tag>
                  )}
                </div>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      {/* Detail table */}
      <Card title={t('cost:global.clusterDetail', '叢集資源明細')}>
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
