import { useState, useCallback, useEffect } from 'react';
import { Form, App } from 'antd';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  CostService,
  ResourceService,
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
} from '../../../services/costService';
import {
  CloudBillingService,
  type CloudBillingConfig,
  type CloudBillingOverview,
  type UpdateBillingConfigReq,
} from '../../../services/cloudBillingService';

export function useCostDashboard(clusterId: string | undefined) {
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

  // Resource occupancy
  const [snapshot, setSnapshot] = useState<ClusterResourceSnapshot | null>(null);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [nsOccupancy, setNsOccupancy] = useState<NamespaceOccupancy[]>([]);
  const [nsOccLoading, setNsOccLoading] = useState(false);

  // Efficiency analysis
  const [nsEfficiency, setNsEfficiency] = useState<NamespaceEfficiency[]>([]);
  const [nsEffLoading, setNsEffLoading] = useState(false);
  const [wlEfficiency, setWlEfficiency] = useState<WorkloadEfficiency[]>([]);
  const [wlEffTotal, setWlEffTotal] = useState(0);
  const [wlEffPage, setWlEffPage] = useState(1);
  const [wlEffLoading, setWlEffLoading] = useState(false);
  const [wlEffNs, setWlEffNs] = useState('');
  const [wasteItems, setWasteItems] = useState<WorkloadEfficiency[]>([]);
  const [wasteItemsLoading, setWasteItemsLoading] = useState(false);

  // Capacity trend + forecast
  const [capacityTrend, setCapacityTrend] = useState<CapacityTrendPoint[]>([]);
  const [capacityTrendLoading, setCapacityTrendLoading] = useState(false);
  const [forecast, setForecast] = useState<ForecastResult | null>(null);
  const [forecastLoading, setForecastLoading] = useState(false);

  // Cloud billing
  const [billingConfig, setBillingConfig] = useState<CloudBillingConfig | null>(null);
  const [billingConfigLoading, setBillingConfigLoading] = useState(false);
  const [billingOverview, setBillingOverview] = useState<CloudBillingOverview | null>(null);
  const [billingOverviewLoading, setBillingOverviewLoading] = useState(false);
  const [billingProvider, setBillingProvider] = useState<'disabled' | 'aws' | 'gcp'>('disabled');
  const [billingSaving, setBillingSaving] = useState(false);
  const [billingSyncing, setBillingSyncing] = useState(false);
  const [billingMonth, setBillingMonth] = useState(dayjs().format('YYYY-MM'));
  const [billingForm] = Form.useForm<UpdateBillingConfigReq>();

  // ── Data loaders ─────────────────────────────────────────────────────────

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

  // ── Effects ─────────────────────────────────────────────────────────────

  useEffect(() => { loadOverview(); }, [loadOverview]);
  useEffect(() => { loadSnapshot(); }, [loadSnapshot]);
  useEffect(() => { if (activeTab === 'occupancy') loadNsOccupancy(); }, [activeTab, loadNsOccupancy]);
  useEffect(() => { if (activeTab === 'efficiency') loadNsEfficiency(); }, [activeTab, loadNsEfficiency]);
  useEffect(() => { if (activeTab === 'wl-efficiency') loadWlEfficiency(1); }, [activeTab, loadWlEfficiency]);
  useEffect(() => { if (activeTab === 'waste-resources') loadWasteItems(); }, [activeTab, loadWasteItems]);
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

  return {
    // Tab state
    activeTab,
    setActiveTab,
    month,
    setMonth,

    // Overview
    overview,
    overviewLoading,
    loadOverview,

    // Namespace costs
    nsCosts,
    nsLoading,
    loadNsCosts,

    // Workload costs
    wlCosts,
    wlTotal,
    wlPage,
    setWlPage,
    wlLoading,
    loadWorkloads,

    // Trend
    trend,
    trendLoading,
    loadTrend,

    // Waste
    waste,
    wasteLoading,
    loadWaste,

    // Snapshot / Occupancy
    snapshot,
    snapshotLoading,
    loadSnapshot,
    nsOccupancy,
    nsOccLoading,
    loadNsOccupancy,

    // Efficiency
    nsEfficiency,
    nsEffLoading,
    loadNsEfficiency,
    wlEfficiency,
    wlEffTotal,
    wlEffPage,
    setWlEffPage,
    wlEffLoading,
    wlEffNs,
    setWlEffNs,
    loadWlEfficiency,
    wasteItems,
    wasteItemsLoading,
    loadWasteItems,

    // Capacity trend + forecast
    capacityTrend,
    capacityTrendLoading,
    loadCapacityTrend,
    forecast,
    forecastLoading,
    loadForecast,

    // Cloud billing
    billingConfig,
    billingConfigLoading,
    loadBillingConfig,
    billingOverview,
    billingOverviewLoading,
    loadBillingOverview,
    billingProvider,
    setBillingProvider,
    billingSaving,
    billingSyncing,
    billingMonth,
    setBillingMonth,
    billingForm,
    saveBillingConfig,
    syncBilling,

    // Computed
    currency: overview?.config?.currency ?? 'USD',
  };
}

export type CostDashboardState = ReturnType<typeof useCostDashboard>;
