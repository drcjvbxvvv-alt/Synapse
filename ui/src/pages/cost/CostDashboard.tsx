import React from 'react';
import { useParams } from 'react-router-dom';
import { Card, Tabs, Tag } from 'antd';
import { WarningOutlined, CloudOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import { useCostDashboard } from './hooks/useCostDashboard';
import {
  OccupancyTab,
  EfficiencyTab,
  WorkloadEfficiencyTab,
  CapacityTrendTab,
  WasteResourcesTab,
  OverviewTab,
  NamespaceCostTab,
  WorkloadCostTab,
  TrendTab,
  WasteTab,
  CloudBillingTab,
} from './tabs';

const CostDashboard: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { t } = useTranslation(['cost', 'common']);
  const state = useCostDashboard(clusterId);

  const tabItems = [
    {
      key: 'overview',
      label: t('cost:tabs.overview'),
      children: (
        <OverviewTab
          snapshot={state.snapshot}
          snapshotLoading={state.snapshotLoading}
          onRefresh={state.loadSnapshot}
        />
      ),
    },
    {
      key: 'occupancy',
      label: t('cost:tabs.occupancy'),
      children: (
        <OccupancyTab
          snapshot={state.snapshot}
          snapshotLoading={state.snapshotLoading}
          nsOccupancy={state.nsOccupancy}
          nsOccLoading={state.nsOccLoading}
          onRefresh={() => { state.loadSnapshot(); state.loadNsOccupancy(); }}
        />
      ),
    },
    {
      key: 'efficiency',
      label: t('cost:tabs.efficiency'),
      children: (
        <EfficiencyTab
          nsEfficiency={state.nsEfficiency}
          nsEffLoading={state.nsEffLoading}
          onRefresh={state.loadNsEfficiency}
        />
      ),
    },
    {
      key: 'wl-efficiency',
      label: t('cost:tabs.wlEfficiency'),
      children: (
        <WorkloadEfficiencyTab
          wlEfficiency={state.wlEfficiency}
          wlEffTotal={state.wlEffTotal}
          wlEffPage={state.wlEffPage}
          wlEffLoading={state.wlEffLoading}
          wlEffNs={state.wlEffNs}
          onPageChange={(p) => { state.setWlEffPage(p); state.loadWlEfficiency(p); }}
          onRefresh={() => state.loadWlEfficiency(1, state.wlEffNs)}
        />
      ),
    },
    {
      key: 'capacity-trend',
      label: t('cost:tabs.capacityTrend'),
      children: (
        <CapacityTrendTab
          capacityTrend={state.capacityTrend}
          capacityTrendLoading={state.capacityTrendLoading}
          forecast={state.forecast}
          forecastLoading={state.forecastLoading}
          onRefresh={() => { state.loadCapacityTrend(); state.loadForecast(); }}
        />
      ),
    },
    {
      key: 'waste-resources',
      label: (
        <span>
          <WarningOutlined style={{ color: state.wasteItems.length > 0 ? '#faad14' : undefined }} />
          {' '}{t('cost:tabs.wasteResources')}
          {state.wasteItems.length > 0 && <Tag color="orange" style={{ marginLeft: 4 }}>{state.wasteItems.length}</Tag>}
        </span>
      ),
      children: (
        <WasteResourcesTab
          wasteItems={state.wasteItems}
          wasteItemsLoading={state.wasteItemsLoading}
          nsEfficiency={state.nsEfficiency}
          clusterId={clusterId!}
          onRefresh={state.loadWasteItems}
        />
      ),
    },
    {
      key: 'namespaces',
      label: t('cost:tabs.namespaces'),
      children: (
        <NamespaceCostTab
          nsCosts={state.nsCosts}
          nsLoading={state.nsLoading}
          month={state.month}
          currency={state.currency}
          onMonthChange={state.setMonth}
          onRefresh={state.loadNsCosts}
        />
      ),
    },
    {
      key: 'workloads',
      label: t('cost:tabs.workloads'),
      children: (
        <WorkloadCostTab
          wlCosts={state.wlCosts}
          wlTotal={state.wlTotal}
          wlPage={state.wlPage}
          wlLoading={state.wlLoading}
          month={state.month}
          clusterId={clusterId!}
          onMonthChange={state.setMonth}
          onPageChange={(p) => { state.setWlPage(p); state.loadWorkloads(p); }}
          onRefresh={() => state.loadWorkloads(1)}
        />
      ),
    },
    {
      key: 'trend',
      label: t('cost:tabs.trend'),
      children: (
        <TrendTab
          trend={state.trend}
          trendLoading={state.trendLoading}
          onRefresh={state.loadTrend}
        />
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
        <CloudBillingTab
          billingConfig={state.billingConfig}
          billingConfigLoading={state.billingConfigLoading}
          billingOverview={state.billingOverview}
          billingOverviewLoading={state.billingOverviewLoading}
          billingProvider={state.billingProvider}
          setBillingProvider={state.setBillingProvider}
          billingSaving={state.billingSaving}
          billingSyncing={state.billingSyncing}
          billingMonth={state.billingMonth}
          setBillingMonth={state.setBillingMonth}
          billingForm={state.billingForm}
          saveBillingConfig={state.saveBillingConfig}
          syncBilling={state.syncBilling}
          loadBillingOverview={state.loadBillingOverview}
        />
      ),
    },
    {
      key: 'waste',
      label: (
        <span>
          <WarningOutlined style={{ color: '#faad14' }} /> {t('cost:tabs.waste')}
          {state.waste.length > 0 && <Tag color="red" style={{ marginLeft: 4 }}>{state.waste.length}</Tag>}
        </span>
      ),
      children: (
        <WasteTab
          waste={state.waste}
          wasteLoading={state.wasteLoading}
          onRefresh={state.loadWaste}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card variant="borderless" title={t('cost:title')}>
        <Tabs activeKey={state.activeTab} onChange={state.setActiveTab} items={tabItems} />
      </Card>
    </div>
  );
};

export default CostDashboard;
