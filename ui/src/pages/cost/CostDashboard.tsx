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
  Form,
  Radio,
  Input,
  Divider,
  Spin,
} from 'antd';
import {
  DownloadOutlined,
  ReloadOutlined,
  WarningOutlined,
  CloudOutlined,
  SyncOutlined,
  SaveOutlined,
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
  ScatterChart,
  Scatter,
  ZAxis,
  Cell,
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
  type NamespaceEfficiency,
  type WorkloadEfficiency,
  type CapacityTrendPoint,
  type ForecastResult,
} from '../../services/costService';
import {
  CloudBillingService,
  type CloudBillingConfig,
  type CloudBillingOverview,
  type UpdateBillingConfigReq,
} from '../../services/cloudBillingService';

const { Text } = Typography;

// ── Unified chart color palette (AntV G2 Colorful) ──────────────────────────
// chart-only: data visualization colors are exempt from design token rule (analogous to TERMINAL_COLORS)
const COLORS = [
  '#5B8FF9', '#5AD8A6', '#F6BD16', '#E8684A',
  '#6DC8EC', '#9867BC', '#FF9D4D', '#269A99',
];

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

  // Phase 2: efficiency analysis
  const [nsEfficiency, setNsEfficiency] = useState<NamespaceEfficiency[]>([]);
  const [nsEffLoading, setNsEffLoading] = useState(false);
  const [wlEfficiency, setWlEfficiency] = useState<WorkloadEfficiency[]>([]);
  const [wlEffTotal, setWlEffTotal] = useState(0);
  const [wlEffPage, setWlEffPage] = useState(1);
  const [wlEffLoading, setWlEffLoading] = useState(false);
  const [wlEffNs, setWlEffNs] = useState('');
  const [wasteItems, setWasteItems] = useState<WorkloadEfficiency[]>([]);
  const [wasteItemsLoading, setWasteItemsLoading] = useState(false);

  // Phase 3: capacity trend + forecast
  const [capacityTrend, setCapacityTrend] = useState<CapacityTrendPoint[]>([]);
  const [capacityTrendLoading, setCapacityTrendLoading] = useState(false);
  const [forecast, setForecast] = useState<ForecastResult | null>(null);
  const [forecastLoading, setForecastLoading] = useState(false);

  // Phase 4: cloud billing
  const [billingConfig, setBillingConfig] = useState<CloudBillingConfig | null>(null);
  const [billingConfigLoading, setBillingConfigLoading] = useState(false);
  const [billingOverview, setBillingOverview] = useState<CloudBillingOverview | null>(null);
  const [billingOverviewLoading, setBillingOverviewLoading] = useState(false);
  const [billingProvider, setBillingProvider] = useState<'disabled' | 'aws' | 'gcp'>('disabled');
  const [billingSaving, setBillingSaving] = useState(false);
  const [billingSyncing, setBillingSyncing] = useState(false);
  const [billingMonth, setBillingMonth] = useState(dayjs().format('YYYY-MM'));
  const [billingForm] = Form.useForm<UpdateBillingConfigReq>();


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

  const loadNsEfficiency = useCallback(async () => {
    if (!clusterId) return;
    setNsEffLoading(true);
    try {
      const data = await ResourceService.getNamespaceEfficiency(clusterId);
      setNsEfficiency(data ?? []);
    } catch { /* ignore */ }
    finally { setNsEffLoading(false); }
  }, [clusterId]);

  const loadWlEfficiency = useCallback(async (page = 1, ns = wlEffNs) => {
    if (!clusterId) return;
    setWlEffLoading(true);
    try {
      const res = await ResourceService.getWorkloadEfficiency(clusterId, ns, page, 20);
      setWlEfficiency(res.items ?? []);
      setWlEffTotal(res.total ?? 0);
    } catch { /* ignore */ }
    finally { setWlEffLoading(false); }
  }, [clusterId, wlEffNs]);

  const loadWasteItems = useCallback(async () => {
    if (!clusterId) return;
    setWasteItemsLoading(true);
    try {
      const data = await ResourceService.getWasteWorkloads(clusterId, 0.2);
      setWasteItems(data ?? []);
    } catch { /* ignore */ }
    finally { setWasteItemsLoading(false); }
  }, [clusterId]);

  const loadCapacityTrend = useCallback(async () => {
    if (!clusterId) return;
    setCapacityTrendLoading(true);
    try {
      const data = await ResourceService.getTrend(clusterId, 6);
      setCapacityTrend(data ?? []);
    } catch { /* ignore */ }
    finally { setCapacityTrendLoading(false); }
  }, [clusterId]);

  const loadForecast = useCallback(async () => {
    if (!clusterId) return;
    setForecastLoading(true);
    try {
      const data = await ResourceService.getForecast(clusterId, 180);
      setForecast(data);
    } catch { /* ignore */ }
    finally { setForecastLoading(false); }
  }, [clusterId]);

  const loadBillingConfig = useCallback(async () => {
    if (!clusterId) return;
    setBillingConfigLoading(true);
    try {
      const cfg = await CloudBillingService.getConfig(clusterId);
      setBillingConfig(cfg);
      setBillingProvider(cfg.provider as 'disabled' | 'aws' | 'gcp');
      billingForm.setFieldsValue({
        provider: cfg.provider,
        aws_access_key_id: cfg.aws_access_key_id,
        aws_region: cfg.aws_region,
        aws_linked_account_id: cfg.aws_linked_account_id,
        gcp_project_id: cfg.gcp_project_id,
        gcp_billing_account_id: cfg.gcp_billing_account_id,
      });
    } catch { /* ignore */ }
    finally { setBillingConfigLoading(false); }
  }, [clusterId, billingForm]);

  const loadBillingOverview = useCallback(async (m?: string) => {
    if (!clusterId) return;
    setBillingOverviewLoading(true);
    try {
      const data = await CloudBillingService.getOverview(clusterId, m ?? billingMonth);
      setBillingOverview(data);
    } catch { setBillingOverview(null); }
    finally { setBillingOverviewLoading(false); }
  }, [clusterId, billingMonth]);

  const saveBillingConfig = async () => {
    if (!clusterId) return;
    const values = await billingForm.validateFields();
    setBillingSaving(true);
    try {
      await CloudBillingService.updateConfig(clusterId, values);
      message.success(t('cost:billing.saveSuccess'));
      loadBillingConfig();
    } catch (e: unknown) {
      message.error((e as Error).message ?? t('common:messages.failed'));
    } finally { setBillingSaving(false); }
  };

  const syncBilling = async () => {
    if (!clusterId) return;
    setBillingSyncing(true);
    try {
      await CloudBillingService.sync(clusterId, billingMonth);
      message.success(t('cost:billing.syncSuccess'));
      loadBillingOverview(billingMonth);
    } catch (e: unknown) {
      message.error((e as Error).message ?? t('cost:billing.syncError'));
    } finally { setBillingSyncing(false); }
  };

  useEffect(() => { loadOverview(); }, [loadOverview]);
  useEffect(() => { loadSnapshot(); }, [loadSnapshot]);
  useEffect(() => { if (activeTab === 'occupancy') { loadNsOccupancy(); } }, [activeTab, loadNsOccupancy]);
  useEffect(() => { if (activeTab === 'efficiency') { loadNsEfficiency(); } }, [activeTab, loadNsEfficiency]);
  useEffect(() => { if (activeTab === 'wl-efficiency') { loadWlEfficiency(1); } }, [activeTab, loadWlEfficiency]);
  useEffect(() => { if (activeTab === 'waste-resources') { loadWasteItems(); } }, [activeTab, loadWasteItems]);
  useEffect(() => { if (activeTab === 'namespaces') loadNsCosts(); }, [activeTab, loadNsCosts]);
  useEffect(() => { if (activeTab === 'workloads') loadWorkloads(1); }, [activeTab, loadWorkloads]);
  useEffect(() => { if (activeTab === 'trend') loadTrend(); }, [activeTab, loadTrend]);
  useEffect(() => { if (activeTab === 'waste') loadWaste(); }, [activeTab, loadWaste]);
  useEffect(() => {
    if (activeTab === 'capacity-trend') { loadCapacityTrend(); loadForecast(); }
  }, [activeTab, loadCapacityTrend, loadForecast]);
  useEffect(() => {
    if (activeTab === 'cloud-billing') { loadBillingConfig(); loadBillingOverview(); }
  }, [activeTab, loadBillingConfig, loadBillingOverview]);

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
                    <CartesianGrid {...GRID_STYLE} />
                    <XAxis dataKey="name" angle={-30} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                    <YAxis unit="%" />
                    <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${v}%`, t('cost:occupancy.cpuRateLabel')]} />
                    <Bar dataKey="cpu" fill="#5B8FF9" {...BAR_PROPS} />
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
              { title: t('cost:occupancy.cpuRequestCol'), dataIndex: 'cpu_request_millicores', key: 'cpu_request_millicores', render: (v: number) => v.toFixed(0) },
              { title: t('cost:occupancy.cpuOccupancyCol'), dataIndex: 'cpu_occupancy_percent', key: 'cpu_occupancy_percent', render: (v: number) => `${v.toFixed(2)}%` },
              { title: t('cost:occupancy.memRequestCol'), dataIndex: 'memory_request_mib', key: 'memory_request_mib', render: (v: number) => v.toFixed(0) },
              { title: t('cost:occupancy.memOccupancyCol'), dataIndex: 'memory_occupancy_percent', key: 'memory_occupancy_percent', render: (v: number) => `${v.toFixed(2)}%` },
              { title: t('cost:occupancy.podCountCol'), dataIndex: 'pod_count', key: 'pod_count' },
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
      key: 'efficiency',
      label: t('cost:tabs.efficiency', '效率分析'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={loadNsEfficiency} loading={nsEffLoading}>
              {t('common:actions.refresh', '重新整理')}
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
                    <Scatter
                      name="Namespace"
                      data={nsEfficiency.map(n => ({
                        ...n,
                        cpu_efficiency_pct: +(n.cpu_efficiency * 100).toFixed(1),
                        mem_efficiency_pct: +(n.memory_efficiency * 100).toFixed(1),
                      }))}
                    >
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
      ),
    },
    {
      key: 'wl-efficiency',
      label: t('cost:tabs.wlEfficiency', '工作負載效率'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={() => loadWlEfficiency(1, wlEffNs)} loading={wlEffLoading}>
              {t('common:actions.refresh', '重新整理')}
            </Button>
          </Space>
          <Table
            rowKey={(r: WorkloadEfficiency) => `${r.namespace}/${r.kind}/${r.name}`}
            loading={wlEffLoading}
            dataSource={wlEfficiency}
            size="small"
            scroll={{ x: 1400 }}
            pagination={{
              current: wlEffPage,
              total: wlEffTotal,
              pageSize: 20,
              onChange: (p) => { setWlEffPage(p); loadWlEfficiency(p); },
            }}
            columns={[
              { title: '命名空間', dataIndex: 'namespace', key: 'namespace', width: 130, fixed: 'left' },
              { title: '工作負載', dataIndex: 'name', key: 'name', width: 160, ellipsis: true },
              { title: '類型', dataIndex: 'kind', key: 'kind', width: 100,
                render: (k: string) => <Tag color={{ Deployment: 'blue', StatefulSet: 'purple', DaemonSet: 'cyan' }[k] ?? 'default'}>{k}</Tag> },
              { title: '副本', dataIndex: 'replicas', key: 'replicas', width: 60 },
              { title: 'CPU 申請 (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_req', width: 110, render: (v: number) => v.toFixed(0) },
              {
                title: 'CPU 效率', dataIndex: 'cpu_efficiency', key: 'cpu_eff', width: 160,
                render: (v: number, r: WorkloadEfficiency) => r.has_metrics ? (
                  <Progress percent={+(v * 100).toFixed(1)} size="small"
                    status={v < 0.2 ? 'exception' : v < 0.5 ? 'active' : 'normal'}
                    format={p => `${p}%`} style={{ width: 120 }} />
                ) : <Tag>需要 Prometheus</Tag>,
                sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.cpu_efficiency - b.cpu_efficiency,
              },
              { title: '記憶體申請 (MiB)', dataIndex: 'memory_request_mib', key: 'mem_req', width: 130, render: (v: number) => v.toFixed(0) },
              {
                title: '記憶體效率', dataIndex: 'memory_efficiency', key: 'mem_eff', width: 160,
                render: (v: number, r: WorkloadEfficiency) => r.has_metrics ? (
                  <Progress percent={+(v * 100).toFixed(1)} size="small"
                    status={v < 0.2 ? 'exception' : v < 0.5 ? 'active' : 'normal'}
                    format={p => `${p}%`} style={{ width: 120 }} />
                ) : <Tag>需要 Prometheus</Tag>,
              },
              {
                title: '廢棄分數',
                dataIndex: 'waste_score',
                key: 'waste',
                width: 90,
                render: (v: number, r: WorkloadEfficiency) => r.has_metrics ? (
                  <Tag color={v > 0.7 ? 'red' : v > 0.4 ? 'orange' : 'green'}>
                    {(v * 100).toFixed(0)}
                  </Tag>
                ) : '—',
                sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => b.waste_score - a.waste_score,
                defaultSortOrder: 'ascend' as const,
              },
              {
                title: '建議 CPU (m)',
                key: 'rec_cpu',
                width: 110,
                render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                  ? <Tag color="geekblue">{r.rightsizing.cpu_recommended_millicores.toFixed(0)}</Tag>
                  : '—',
              },
              {
                title: '建議記憶體 (MiB)',
                key: 'rec_mem',
                width: 130,
                render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                  ? <Tag color="geekblue">{r.rightsizing.memory_recommended_mib.toFixed(0)}</Tag>
                  : '—',
              },
            ]}
          />
        </div>
      ),
    },
    {
      key: 'capacity-trend',
      label: t('cost:tabs.capacityTrend', '容量趨勢'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={() => { loadCapacityTrend(); loadForecast(); }} loading={capacityTrendLoading || forecastLoading}>
              {t('common:actions.refresh', '重新整理')}
            </Button>
          </Space>
          {capacityTrend.length === 0 && !capacityTrendLoading ? (
            <Empty description="尚無歷史快照資料，CostWorker 每日 00:05 UTC 拍攝，連線後隔日可見趨勢。" />
          ) : (
            <>
              <Card title="CPU & 記憶體佔用率月度趨勢" size="small" style={{ marginBottom: 16 }} loading={capacityTrendLoading}>
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={capacityTrend} margin={{ top: 10, right: 30, left: 10, bottom: 5 }}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="month" />
                    <YAxis unit="%" domain={[0, 100]} />
                    <RechartTooltip formatter={(v) => [`${Number(v ?? 0).toFixed(1)}%`]} />
                    <Legend />
                    <Line type="monotone" dataKey="cpu_occupancy_percent" name="CPU 佔用率" stroke="#7eb8d4" strokeWidth={2} dot={{ r: 4 }} />
                    <Line type="monotone" dataKey="memory_occupancy_percent" name="記憶體佔用率" stroke="#a8c9a5" strokeWidth={2} dot={{ r: 4 }} />
                  </LineChart>
                </ResponsiveContainer>
              </Card>
              <Card title="容量耗盡預測（線性外推）" size="small" loading={forecastLoading}>
                {forecast && (
                  <Row gutter={16}>
                    <Col xs={24} sm={12}>
                      <Card size="small" title="CPU 佔用率預測" style={{ background: '#fafafa' }}>
                        <Statistic title="當前佔用率" value={forecast.current_cpu_percent} precision={1} suffix="%" />
                        <div style={{ marginTop: 12 }}>
                          <Space direction="vertical" size={4}>
                            <span>到達 80%：
                              {forecast.cpu_80_percent_date
                                ? <Tag color="orange">{forecast.cpu_80_percent_date}</Tag>
                                : <Tag color="green">預測期內不到達</Tag>}
                            </span>
                            <span>到達 100%：
                              {forecast.cpu_100_percent_date
                                ? <Tag color="red">{forecast.cpu_100_percent_date}</Tag>
                                : <Tag color="green">預測期內不到達</Tag>}
                            </span>
                          </Space>
                        </div>
                      </Card>
                    </Col>
                    <Col xs={24} sm={12}>
                      <Card size="small" title="記憶體佔用率預測" style={{ background: '#fafafa' }}>
                        <Statistic title="當前佔用率" value={forecast.current_memory_percent} precision={1} suffix="%" />
                        <div style={{ marginTop: 12 }}>
                          <Space direction="vertical" size={4}>
                            <span>到達 80%：
                              {forecast.memory_80_percent_date
                                ? <Tag color="orange">{forecast.memory_80_percent_date}</Tag>
                                : <Tag color="green">預測期內不到達</Tag>}
                            </span>
                            <span>到達 100%：
                              {forecast.memory_100_percent_date
                                ? <Tag color="red">{forecast.memory_100_percent_date}</Tag>
                                : <Tag color="green">預測期內不到達</Tag>}
                            </span>
                          </Space>
                        </div>
                      </Card>
                    </Col>
                    <Col xs={24} style={{ marginTop: 8 }}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        基於近 {forecast.based_on_months} 個月歷史資料進行線性外推，預測期 180 天。數據點不足或趨勢下降時顯示「預測期內不到達」。
                      </Typography.Text>
                    </Col>
                  </Row>
                )}
              </Card>
            </>
          )}
        </div>
      ),
    },
    {
      key: 'waste-resources',
      label: (
        <span>
          <WarningOutlined style={{ color: wasteItems.length > 0 ? '#faad14' : undefined }} />
          {t('cost:tabs.wasteResources', '低效識別')}
          {wasteItems.length > 0 && <Tag color="orange" style={{ marginLeft: 4 }}>{wasteItems.length}</Tag>}
        </span>
      ),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={loadWasteItems} loading={wasteItemsLoading}>
              {t('common:actions.refresh', '重新整理')}
            </Button>
            <Button
              icon={<DownloadOutlined />}
              href={ResourceService.getWasteExportURL(clusterId!)}
              target="_blank"
              disabled={wasteItems.length === 0}
            >
              匯出 CSV
            </Button>
          </Space>
          <Alert
            type="warning"
            showIcon
            message="以下工作負載的 CPU 效率低於 20%，代表申請的資源遠超實際使用量，建議降低 CPU requests。"
            style={{ marginBottom: 16 }}
          />
          {wasteItems.length === 0 && !wasteItemsLoading ? (
            <Empty description={nsEfficiency.length > 0 && !nsEfficiency[0].has_metrics ? '需要 Prometheus 監控資料才能識別低效工作負載' : '目前無低效工作負載'} />
          ) : (
            <Table
              rowKey={(r: WorkloadEfficiency) => `${r.namespace}/${r.kind}/${r.name}`}
              loading={wasteItemsLoading}
              dataSource={wasteItems}
              size="small"
              scroll={{ x: 900 }}
              pagination={{ pageSize: 20 }}
              columns={[
                { title: '命名空間', dataIndex: 'namespace', key: 'namespace', width: 140 },
                { title: '工作負載', dataIndex: 'name', key: 'name', ellipsis: true },
                { title: '類型', dataIndex: 'kind', key: 'kind', width: 110,
                  render: (k: string) => <Tag color={{ Deployment: 'blue', StatefulSet: 'purple', DaemonSet: 'cyan' }[k] ?? 'default'}>{k}</Tag> },
                { title: '副本', dataIndex: 'replicas', key: 'replicas', width: 70 },
                {
                  title: 'CPU 效率',
                  render: (_: unknown, r: WorkloadEfficiency) => (
                    <Progress percent={+(r.cpu_efficiency * 100).toFixed(1)} size="small"
                      status="exception" format={p => `${p}%`} style={{ width: 110 }} />
                  ),
                  sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.cpu_efficiency - b.cpu_efficiency,
                },
                { title: 'CPU 申請 (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_req', render: (v: number) => v.toFixed(0) },
                { title: 'CPU 使用 (m)', dataIndex: 'cpu_usage_millicores', key: 'cpu_usage', render: (v: number) => v.toFixed(1) },
                { title: '記憶體效率', render: (_: unknown, r: WorkloadEfficiency) => (
                  <Progress percent={+(r.memory_efficiency * 100).toFixed(1)} size="small"
                    status={r.memory_efficiency < 0.2 ? 'exception' : 'active'} format={p => `${p}%`} style={{ width: 110 }} />
                )},
                {
                  title: '廢棄分數',
                  render: (_: unknown, r: WorkloadEfficiency) => (
                    <Tag color={r.waste_score > 0.7 ? 'red' : 'orange'}>{(r.waste_score * 100).toFixed(0)}</Tag>
                  ),
                  sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => b.waste_score - a.waste_score,
                  defaultSortOrder: 'ascend' as const,
                },
                {
                  title: '建議 CPU (m)',
                  key: 'rec_cpu',
                  render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                    ? <Tag color="geekblue">{r.rightsizing.cpu_recommended_millicores.toFixed(0)}</Tag>
                    : '—',
                },
                {
                  title: '建議記憶體 (MiB)',
                  key: 'rec_mem',
                  render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                    ? <Tag color="geekblue">{r.rightsizing.memory_recommended_mib.toFixed(0)}</Tag>
                    : '—',
                },
              ]}
            />
          )}
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
                    <CartesianGrid {...GRID_STYLE} />
                    <XAxis dataKey="name" angle={-30} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                    <YAxis />
                    <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${currency} ${v}`, t('cost:table.estCost')]} />
                    <Bar dataKey="cost" fill="#5B8FF9" {...BAR_PROPS} />
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
      key: 'cloud-billing',
      label: (
        <span>
          <CloudOutlined /> {t('cost:tabs.cloudBilling')}
        </span>
      ),
      children: (
        <div>
          {/* Config card */}
          <Card
            title={t('cost:billing.configTitle')}
            size="small"
            style={{ marginBottom: 16 }}
            loading={billingConfigLoading}
            extra={
              <Space>
                <Button
                  icon={<SaveOutlined />}
                  type="primary"
                  loading={billingSaving}
                  onClick={saveBillingConfig}
                >
                  {t('cost:billing.saveConfig')}
                </Button>
              </Space>
            }
          >
            <Form form={billingForm} layout="vertical" style={{ maxWidth: 600 }}>
              <Form.Item name="provider" label={t('cost:billing.provider')} initialValue="disabled">
                <Radio.Group onChange={e => setBillingProvider(e.target.value)}>
                  <Radio.Button value="disabled">{t('cost:billing.providerDisabled')}</Radio.Button>
                  <Radio.Button value="aws">AWS</Radio.Button>
                  <Radio.Button value="gcp">GCP</Radio.Button>
                </Radio.Group>
              </Form.Item>

              {billingProvider === 'aws' && (
                <>
                  <Divider orientation="left" plain style={{ fontSize: 13 }}>AWS Cost Explorer</Divider>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="aws_access_key_id" label={t('cost:billing.accessKeyId')}>
                        <Input placeholder="AKIA..." />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="aws_secret_access_key" label={`${t('cost:billing.secretAccessKey')}${billingConfig?.aws_secret_set ? ` (${t('cost:billing.secretSet')})` : ''}`}>
                        <Input.Password placeholder={billingConfig?.aws_secret_set ? `******** (${t('cost:billing.keepOriginal')})` : t('cost:billing.inputSecret')} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="aws_region" label={t('cost:billing.region')} initialValue="us-east-1">
                        <Input placeholder="us-east-1" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="aws_linked_account_id" label={t('cost:billing.linkedAccountId')}>
                        <Input placeholder="123456789012" />
                      </Form.Item>
                    </Col>
                  </Row>
                </>
              )}

              {billingProvider === 'gcp' && (
                <>
                  <Divider orientation="left" plain style={{ fontSize: 13 }}>GCP Cloud Billing</Divider>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="gcp_project_id" label="Project ID">
                        <Input placeholder="my-project-id" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="gcp_billing_account_id" label="Billing Account ID">
                        <Input placeholder="XXXXXX-XXXXXX-XXXXXX" />
                      </Form.Item>
                    </Col>
                    <Col span={24}>
                      <Form.Item
                        name="gcp_service_account_json"
                        label={`${t('cost:billing.serviceAccountJson')}${billingConfig?.gcp_service_account_set ? ` (${t('cost:billing.secretSet')})` : ''}`}
                      >
                        <Input.TextArea
                          rows={5}
                          placeholder={billingConfig?.gcp_service_account_set ? t('cost:billing.savedKeepOriginal') : t('cost:billing.pasteServiceAccount')}
                        />
                      </Form.Item>
                    </Col>
                  </Row>
                </>
              )}
            </Form>

            {billingConfig && billingConfig.provider !== 'disabled' && (
              <div style={{ marginTop: 8 }}>
                {billingConfig.last_synced_at
                  ? <Typography.Text type="secondary">{t('cost:billing.lastSync')}{billingConfig.last_synced_at}</Typography.Text>
                  : <Typography.Text type="secondary">{t('cost:billing.neverSynced')}</Typography.Text>}
                {billingConfig.last_error && (
                  <Alert type="error" message={billingConfig.last_error} showIcon style={{ marginTop: 8 }} />
                )}
              </div>
            )}
          </Card>

          {/* Sync controls + overview */}
          {billingConfig?.provider !== 'disabled' && (
            <Card
              title={t('cost:billing.overview')}
              size="small"
              extra={
                <Space>
                  <DatePicker.MonthPicker
                    value={dayjs(billingMonth, 'YYYY-MM')}
                    onChange={v => v && setBillingMonth(v.format('YYYY-MM'))}
                    allowClear={false}
                    style={{ width: 120 }}
                  />
                  <Button
                    icon={<SyncOutlined />}
                    loading={billingSyncing}
                    onClick={syncBilling}
                  >
                    {t('cost:billing.syncBtn')}
                  </Button>
                  <Button
                    icon={<ReloadOutlined />}
                    onClick={() => loadBillingOverview(billingMonth)}
                    loading={billingOverviewLoading}
                  >
                    {t('common:actions.refresh')}
                  </Button>
                </Space>
              }
            >
              <Spin spinning={billingOverviewLoading}>
                {!billingOverview ? (
                  <Empty description={t('cost:billing.emptyData')} />
                ) : (
                  <>
                    <Row gutter={16} style={{ marginBottom: 20 }}>
                      <Col xs={24} sm={8}>
                        <Statistic
                          title={`${billingOverview.month} ${t('cost:billing.totalCost')}`}
                          value={billingOverview.total_amount}
                          precision={2}
                          suffix={billingOverview.currency}
                          valueStyle={{ color: '#1677ff' }}
                        />
                      </Col>
                      <Col xs={24} sm={8}>
                        <Statistic
                          title="CPU 單位成本"
                          value={billingOverview.cpu_unit_cost}
                          precision={4}
                          suffix="USD/core-hr"
                        />
                      </Col>
                      <Col xs={24} sm={8}>
                        <Statistic
                          title="記憶體單位成本"
                          value={billingOverview.memory_unit_cost}
                          precision={4}
                          suffix="USD/GiB-hr"
                        />
                      </Col>
                    </Row>
                    {billingOverview.services?.length > 0 && (
                      <Row gutter={16}>
                        <Col xs={24} lg={14}>
                          <ResponsiveContainer width="100%" height={260}>
                            <BarChart
                              data={billingOverview.services.slice(0, 12).map(s => ({
                                name: s.service.replace('Amazon ', '').replace('Google ', ''),
                                amount: +s.amount.toFixed(2),
                              }))}
                              layout="vertical"
                              margin={{ top: 5, right: 30, left: 10, bottom: 5 }}
                            >
                              <CartesianGrid {...GRID_STYLE} />
                              <XAxis type="number" unit={` ${billingOverview.currency}`} />
                              <YAxis type="category" dataKey="name" width={130} tick={{ fontSize: 11 }} />
                              <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${billingOverview.currency} ${v}`, '費用']} />
                              <Bar dataKey="amount" fill="#5B8FF9"
                                radius={[0, 5, 5, 0]}
                                maxBarSize={44}
                                isAnimationActive={true}
                                animationBegin={0}
                                animationDuration={800}
                                animationEasing="ease-out"
                              />
                            </BarChart>
                          </ResponsiveContainer>
                        </Col>
                        <Col xs={24} lg={10}>
                          <Table
                            rowKey="id"
                            size="small"
                            dataSource={billingOverview.services}
                            pagination={{ pageSize: 10, size: 'small' }}
                            columns={[
                              { title: '服務', dataIndex: 'service', key: 'service', ellipsis: true },
                              {
                                title: '費用',
                                dataIndex: 'amount',
                                key: 'amount',
                                render: (v: number) => <Text strong>{`${billingOverview.currency} ${v.toFixed(2)}`}</Text>,
                                sorter: (a, b) => b.amount - a.amount,
                                defaultSortOrder: 'ascend' as const,
                              },
                            ]}
                          />
                        </Col>
                      </Row>
                    )}
                    {billingOverview.sync_error && (
                      <Alert type="warning" message={billingOverview.sync_error} showIcon style={{ marginTop: 12 }} />
                    )}
                  </>
                )}
              </Spin>
            </Card>
          )}
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
