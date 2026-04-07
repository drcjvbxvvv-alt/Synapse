import React, { useState, useCallback, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card,
  Tabs,
  Statistic,
  Table,
  Progress,
  Button,
  Space,
  DatePicker,
  Tag,
  Tooltip,
  Empty,
  Row,
  Col,
  App,
  Typography,
  Alert,
} from 'antd';
import {
  DownloadOutlined,
  ReloadOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
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
import EmptyState from '../../components/EmptyState';
import {
  CostService,
  ResourceService,
  type CostConfig,
  type CostItem,
  type CostOverview,
  type TrendPoint,
  type WasteItem,
  type ClusterResourceSnapshot,
  type NamespaceOccupancy,
} from '../../services/costService';

const { Text } = Typography;

const COLORS = [
  '#4e79a7', '#f28e2b', '#e15759', '#76b7b2',
  '#59a14f', '#edc948', '#b07aa1', '#ff9da7',
];

const CostDashboard: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { t } = useTranslation(['cost', 'common']);
  const { message } = App.useApp();

  const [activeTab, setActiveTab] = useState('overview');
  const [month, setMonth] = useState(dayjs().format('YYYY-MM'));

  // Overview
  const [overview, setOverview] = useState<CostOverview | null>(null);
  const [overviewLoading, setOverviewLoading] = useState(false);

  // Namespace costs
  const [nsCosts, setNsCosts] = useState<CostItem[]>([]);
  const [nsLoading, setNsLoading] = useState(false);

  // Workload costs
  const [wlCosts, setWlCosts] = useState<CostItem[]>([]);
  const [wlTotal, setWlTotal] = useState(0);
  const [wlPage, setWlPage] = useState(1);
  const [wlLoading, setWlLoading] = useState(false);

  // Trend
  const [trend, setTrend] = useState<TrendPoint[]>([]);
  const [trendLoading, setTrendLoading] = useState(false);

  // Waste
  const [waste, setWaste] = useState<WasteItem[]>([]);
  const [wasteLoading, setWasteLoading] = useState(false);

  // Resource occupancy (Phase 1)
  const [snapshot, setSnapshot] = useState<ClusterResourceSnapshot | null>(null);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [nsOccupancy, setNsOccupancy] = useState<NamespaceOccupancy[]>([]);
  const [nsOccLoading, setNsOccLoading] = useState(false);


  const loadOverview = useCallback(async () => {
    if (!clusterId) return;
    setOverviewLoading(true);
    try {
      const data = await CostService.getOverview(clusterId, month);
      setOverview(data);
    } catch { /* ignore */ }
    finally { setOverviewLoading(false); }
  }, [clusterId, month]);

  const loadNsCosts = useCallback(async () => {
    if (!clusterId) return;
    setNsLoading(true);
    try {
      const data = await CostService.getNamespaceCosts(clusterId, month);
      setNsCosts(data ?? []);
    } catch { /* ignore */ }
    finally { setNsLoading(false); }
  }, [clusterId, month]);

  const loadWorkloads = useCallback(async (page = 1) => {
    if (!clusterId) return;
    setWlLoading(true);
    try {
      const res = await CostService.getWorkloadCosts(clusterId, month, undefined, page);
      setWlCosts(res.items ?? []);
      setWlTotal(res.total ?? 0);
    } catch { /* ignore */ }
    finally { setWlLoading(false); }
  }, [clusterId, month]);

  const loadTrend = useCallback(async () => {
    if (!clusterId) return;
    setTrendLoading(true);
    try {
      const data = await CostService.getTrend(clusterId, 6);
      setTrend(data ?? []);
    } catch { /* ignore */ }
    finally { setTrendLoading(false); }
  }, [clusterId]);

  const loadWaste = useCallback(async () => {
    if (!clusterId) return;
    setWasteLoading(true);
    try {
      const data = await CostService.getWaste(clusterId);
      setWaste(data ?? []);
    } catch { /* ignore */ }
    finally { setWasteLoading(false); }
  }, [clusterId]);

  const loadSnapshot = useCallback(async () => {
    if (!clusterId) return;
    setSnapshotLoading(true);
    try {
      const data = await ResourceService.getSnapshot(clusterId);
      setSnapshot(data);
    } catch { /* ignore */ }
    finally { setSnapshotLoading(false); }
  }, [clusterId]);

  const loadNsOccupancy = useCallback(async () => {
    if (!clusterId) return;
    setNsOccLoading(true);
    try {
      const data = await ResourceService.getNamespaceOccupancy(clusterId);
      setNsOccupancy(data ?? []);
    } catch { /* ignore */ }
    finally { setNsOccLoading(false); }
  }, [clusterId]);

  useEffect(() => { loadOverview(); }, [loadOverview]);
  useEffect(() => { loadSnapshot(); }, [loadSnapshot]);
  useEffect(() => { if (activeTab === 'occupancy') { loadNsOccupancy(); } }, [activeTab, loadNsOccupancy]);
  useEffect(() => { if (activeTab === 'namespaces') loadNsCosts(); }, [activeTab, loadNsCosts]);
  useEffect(() => { if (activeTab === 'workloads') loadWorkloads(1); }, [activeTab, loadWorkloads]);
  useEffect(() => { if (activeTab === 'trend') loadTrend(); }, [activeTab, loadTrend]);
  useEffect(() => { if (activeTab === 'waste') loadWaste(); }, [activeTab, loadWaste]);

  const currency = overview?.config?.currency ?? 'USD';

  // ---- Bar chart data (top 10 namespaces) ----
  const barData = nsCosts.slice(0, 10).map(item => ({
    name: item.name,
    cost: Number(item.est_cost.toFixed(4)),
  }));

  // ---- Line chart data (trend) ----
  const allNs = Array.from(new Set(trend.flatMap(p => p.breakdown?.map(b => b.namespace) ?? [])));
  const lineData = trend.map(p => {
    const point: Record<string, string | number> = { month: p.month };
    allNs.forEach(ns => {
      point[ns] = p.breakdown?.find(b => b.namespace === ns)?.cost ?? 0;
    });
    point.total = Number(p.total.toFixed(4));
    return point;
  });

  // ---- Columns ----
  const utilCell = (val: number) => (
    <Progress
      percent={Number(val.toFixed(1))}
      size="small"
      status={val < 10 ? 'exception' : val < 50 ? 'active' : 'normal'}
      format={p => `${p}%`}
    />
  );

  const nsColumns = [
    { title: t('cost:table.namespace'), dataIndex: 'name', key: 'name' },
    { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request', key: 'cpu_request', render: (v: number) => v.toFixed(1) },
    { title: t('cost:table.cpuUtil'), dataIndex: 'cpu_util', key: 'cpu_util', render: utilCell },
    { title: t('cost:table.memRequest'), dataIndex: 'mem_request', key: 'mem_request', render: (v: number) => v.toFixed(1) },
    { title: t('cost:table.memUtil'), dataIndex: 'mem_util', key: 'mem_util', render: utilCell },
    { title: t('cost:table.podCount'), dataIndex: 'pod_count', key: 'pod_count' },
    {
      title: t('cost:table.estCost'), dataIndex: 'est_cost', key: 'est_cost',
      render: (v: number, row: CostItem) => <Text strong>{`${row.currency} ${v.toFixed(4)}`}</Text>,
      sorter: (a: CostItem, b: CostItem) => b.est_cost - a.est_cost,
    },
  ];

  const wlColumns = [
    { title: t('cost:table.workload'), dataIndex: 'name', key: 'name', ellipsis: true },
    { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request', key: 'cpu_request', render: (v: number) => v.toFixed(1) },
    { title: t('cost:table.cpuUtil'), dataIndex: 'cpu_util', key: 'cpu_util', render: utilCell },
    { title: t('cost:table.memRequest'), dataIndex: 'mem_request', key: 'mem_request', render: (v: number) => v.toFixed(1) },
    { title: t('cost:table.memUtil'), dataIndex: 'mem_util', key: 'mem_util', render: utilCell },
    {
      title: t('cost:table.estCost'), dataIndex: 'est_cost', key: 'est_cost',
      render: (v: number, row: CostItem) => `${row.currency} ${v.toFixed(4)}`,
    },
  ];

  const wasteColumns = [
    { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace' },
    { title: t('cost:table.workload'), dataIndex: 'workload', key: 'workload', ellipsis: true },
    { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request', key: 'cpu_request', render: (v: number) => v.toFixed(1) },
    {
      title: t('cost:table.cpuUtil'), dataIndex: 'cpu_util', key: 'cpu_util',
      render: (v: number) => <Tag color="red">{v.toFixed(1)}%</Tag>,
    },
    { title: t('cost:table.memRequest'), dataIndex: 'mem_request', key: 'mem_request', render: (v: number) => v.toFixed(1) },
    { title: t('cost:table.days'), dataIndex: 'days', key: 'days' },
    {
      title: t('cost:table.wastedCost'), dataIndex: 'wasted_cost', key: 'wasted_cost',
      render: (v: number, row: WasteItem) => (
        <Text type="danger" strong>{`${row.currency} ${v.toFixed(4)}`}</Text>
      ),
    },
  ];

  const monthPicker = (
    <DatePicker.MonthPicker
      value={dayjs(month, 'YYYY-MM')}
      onChange={v => v && setMonth(v.format('YYYY-MM'))}
      allowClear={false}
      style={{ width: 130 }}
    />
  );

  const tabItems = [
    {
      key: 'occupancy',
      label: t('cost:tabs.occupancy', '資源佔用'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={() => { loadSnapshot(); loadNsOccupancy(); }}>
              {t('common:actions.refresh', '重新整理')}
            </Button>
          </Space>
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.cpuOccupancy', 'CPU 佔用率')}
                  value={snapshot?.occupancy.cpu ?? 0}
                  precision={1}
                  suffix="%"
                  loading={snapshotLoading}
                  valueStyle={{ color: (snapshot?.occupancy.cpu ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
                />
                <Progress percent={+(snapshot?.occupancy.cpu ?? 0).toFixed(1)} showInfo={false} status={(snapshot?.occupancy.cpu ?? 0) > 80 ? 'exception' : 'normal'} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.memOccupancy', '記憶體佔用率')}
                  value={snapshot?.occupancy.memory ?? 0}
                  precision={1}
                  suffix="%"
                  loading={snapshotLoading}
                  valueStyle={{ color: (snapshot?.occupancy.memory ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
                />
                <Progress percent={+(snapshot?.occupancy.memory ?? 0).toFixed(1)} showInfo={false} status={(snapshot?.occupancy.memory ?? 0) > 80 ? 'exception' : 'normal'} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.nodeCount', '節點數')}
                  value={snapshot?.node_count ?? 0}
                  loading={snapshotLoading}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.podCount', 'Pod 數')}
                  value={snapshot?.pod_count ?? 0}
                  loading={snapshotLoading}
                />
              </Card>
            </Col>
          </Row>
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col xs={24} lg={14}>
              <Card title={t('cost:occupancy.nsBreakdown', '命名空間 CPU 佔用')} size="small">
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart
                    data={nsOccupancy.slice(0, 15).map(n => ({ name: n.namespace, cpu: +n.cpu_occupancy_percent.toFixed(2) }))}
                    margin={{ top: 5, right: 20, left: 10, bottom: 60 }}
                  >
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="name" angle={-30} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                    <YAxis unit="%" />
                    <RechartTooltip formatter={(v) => [`${v}%`, 'CPU 佔用率']} />
                    <Bar dataKey="cpu" fill="#4e79a7" />
                  </BarChart>
                </ResponsiveContainer>
              </Card>
            </Col>
            <Col xs={24} lg={10}>
              <Card title={t('cost:occupancy.headroom', '可用空間')} size="small">
                <Statistic
                  title={t('cost:occupancy.cpuHeadroom', 'CPU 剩餘 (millicores)')}
                  value={+(snapshot?.headroom.cpu_millicores ?? 0).toFixed(0)}
                  loading={snapshotLoading}
                />
                <Statistic
                  title={t('cost:occupancy.memHeadroom', '記憶體剩餘 (MiB)')}
                  value={+(snapshot?.headroom.memory_mib ?? 0).toFixed(0)}
                  loading={snapshotLoading}
                  style={{ marginTop: 16 }}
                />
              </Card>
            </Col>
          </Row>
          <Table
            rowKey="namespace"
            columns={[
              { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace' },
              { title: 'CPU 申請 (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_request_millicores', render: (v: number) => v.toFixed(0) },
              { title: 'CPU 佔用 %', dataIndex: 'cpu_occupancy_percent', key: 'cpu_occupancy_percent', render: (v: number) => `${v.toFixed(2)}%` },
              { title: '記憶體申請 (MiB)', dataIndex: 'memory_request_mib', key: 'memory_request_mib', render: (v: number) => v.toFixed(0) },
              { title: '記憶體佔用 %', dataIndex: 'memory_occupancy_percent', key: 'memory_occupancy_percent', render: (v: number) => `${v.toFixed(2)}%` },
              { title: 'Pod 數', dataIndex: 'pod_count', key: 'pod_count' },
            ]}
            dataSource={nsOccupancy}
            loading={nsOccLoading}
            size="small"
            scroll={{ x: 700 }}
            pagination={{ pageSize: 20 }}
          />
        </div>
      ),
    },
    {
      key: 'overview',
      label: t('cost:tabs.overview'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={loadSnapshot} loading={snapshotLoading}>
              {t('common:actions.refresh', '重新整理')}
            </Button>
          </Space>
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.cpuOccupancy', 'CPU 佔用率')}
                  value={snapshot?.occupancy.cpu ?? 0}
                  precision={1}
                  suffix="%"
                  loading={snapshotLoading}
                  valueStyle={{ color: (snapshot?.occupancy.cpu ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
                />
                <Progress
                  percent={+(snapshot?.occupancy.cpu ?? 0).toFixed(1)}
                  showInfo={false}
                  status={(snapshot?.occupancy.cpu ?? 0) > 80 ? 'exception' : 'normal'}
                  style={{ marginTop: 8 }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.memOccupancy', '記憶體佔用率')}
                  value={snapshot?.occupancy.memory ?? 0}
                  precision={1}
                  suffix="%"
                  loading={snapshotLoading}
                  valueStyle={{ color: (snapshot?.occupancy.memory ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
                />
                <Progress
                  percent={+(snapshot?.occupancy.memory ?? 0).toFixed(1)}
                  showInfo={false}
                  status={(snapshot?.occupancy.memory ?? 0) > 80 ? 'exception' : 'normal'}
                  style={{ marginTop: 8 }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card>
                <Statistic
                  title={t('cost:occupancy.nodeCount', '節點數')}
                  value={snapshot?.node_count ?? 0}
                  loading={snapshotLoading}
                />
                <Statistic
                  title={t('cost:occupancy.podCount', 'Pod 數')}
                  value={snapshot?.pod_count ?? 0}
                  loading={snapshotLoading}
                  style={{ marginTop: 12 }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card title={t('cost:occupancy.headroom', '可用空間')} size="small">
                <Statistic
                  title={t('cost:occupancy.cpuHeadroom', 'CPU 剩餘 (m)')}
                  value={+(snapshot?.headroom.cpu_millicores ?? 0).toFixed(0)}
                  loading={snapshotLoading}
                />
                <Statistic
                  title={t('cost:occupancy.memHeadroom', '記憶體剩餘 (MiB)')}
                  value={+(snapshot?.headroom.memory_mib ?? 0).toFixed(0)}
                  loading={snapshotLoading}
                  style={{ marginTop: 12 }}
                />
              </Card>
            </Col>
          </Row>
        </div>
      ),
    },
    {
      key: 'namespaces',
      label: t('cost:tabs.namespaces'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            {monthPicker}
            <Button icon={<ReloadOutlined />} onClick={loadNsCosts}>{t('common:actions.refresh', '重新整理')}</Button>
          </Space>
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col xs={24} lg={14}>
              <Card title={t('cost:tabs.namespaces')} size="small">
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart data={barData} margin={{ top: 5, right: 20, left: 10, bottom: 60 }}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="name" angle={-30} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                    <YAxis />
                    <RechartTooltip formatter={(v) => [`${currency} ${v}`, t('cost:table.estCost')]} />
                    <Bar dataKey="cost" fill="#4e79a7" />
                  </BarChart>
                </ResponsiveContainer>
              </Card>
            </Col>
          </Row>
          <Table
            rowKey="name"
            columns={nsColumns}
            dataSource={nsCosts}
            loading={nsLoading}
            size="small"
            scroll={{ x: 800 }}
            pagination={false}
          />
        </div>
      ),
    },
    {
      key: 'workloads',
      label: t('cost:tabs.workloads'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            {monthPicker}
            <Button icon={<ReloadOutlined />} onClick={() => loadWorkloads(1)}>{t('common:actions.refresh', '重新整理')}</Button>
            <Tooltip title={t('cost:export.button')}>
              <Button
                icon={<DownloadOutlined />}
                href={CostService.getExportURL(clusterId!, month)}
                target="_blank"
              >
                {t('cost:export.button')}
              </Button>
            </Tooltip>
          </Space>
          <Table
            rowKey="name"
            columns={wlColumns}
            dataSource={wlCosts}
            loading={wlLoading}
            size="small"
            scroll={{ x: 800 }}
            pagination={{
              current: wlPage,
              total: wlTotal,
              pageSize: 20,
              onChange: (p) => { setWlPage(p); loadWorkloads(p); },
            }}
          />
        </div>
      ),
    },
    {
      key: 'trend',
      label: t('cost:tabs.trend'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={loadTrend}>{t('common:actions.refresh', '重新整理')}</Button>
          </Space>
          <Card title={t('cost:trend.title')} loading={trendLoading}>
            <ResponsiveContainer width="100%" height={320}>
              <LineChart data={lineData} margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="month" />
                <YAxis />
                <RechartTooltip />
                <Legend />
                {allNs.map((ns, i) => (
                  <Line
                    key={ns}
                    type="monotone"
                    dataKey={ns}
                    stroke={COLORS[i % COLORS.length]}
                    dot={false}
                  />
                ))}
                <Line type="monotone" dataKey="total" stroke="#000" strokeWidth={2} dot={false} name={t('cost:trend.totalLabel')} />
              </LineChart>
            </ResponsiveContainer>
          </Card>
        </div>
      ),
    },
    {
      key: 'waste',
      label: (
        <span>
          <WarningOutlined style={{ color: '#faad14' }} /> {t('cost:tabs.waste')}
          {waste.length > 0 && <Tag color="red" style={{ marginLeft: 4 }}>{waste.length}</Tag>}
        </span>
      ),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={loadWaste}>{t('common:actions.refresh', '重新整理')}</Button>
          </Space>
          {waste.length === 0 && !wasteLoading ? (
            <Empty description={t('cost:waste.empty')} />
          ) : (
            <>
              <Alert
                type="warning"
                showIcon
                message={t('cost:waste.suggestion')}
                style={{ marginBottom: 12 }}
              />
              <Table
                rowKey={(r: WasteItem) => `${r.namespace}/${r.workload}`}
                columns={wasteColumns}
                dataSource={waste}
                loading={wasteLoading}
                size="small"
                scroll={{ x: 900 }}
                pagination={false}
              />
            </>
          )}
        </div>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card bordered={false} title={t('cost:title')}>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />
      </Card>
    </div>
  );
};

export default CostDashboard;
