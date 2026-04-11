import React from 'react';
import { Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import { useOverview } from './hooks/useOverview';
import { OverviewToolbar } from './components/OverviewToolbar';
import { StatCards } from './components/StatCards';
import { ResourceUsageCards } from './components/ResourceUsageCards';
import { VersionAndAbnormal } from './components/VersionAndAbnormal';
import { DistributionCharts } from './components/DistributionCharts';
import { TrendCharts } from './components/TrendCharts';

const Overview: React.FC = () => {
  const { t } = useTranslation('common');
  const {
    loading,
    stats,
    podTimeRange,
    nodeTimeRange,
    autoRefresh,
    refreshInterval,
    lastRefreshTime,
    setPodTimeRange,
    setNodeTimeRange,
    setAutoRefresh,
    setRefreshInterval,
    fetchData,
    clusterStats,
    nodeStats,
    podStats,
    versionDistribution,
    cpuUsage,
    memoryUsage,
    storageUsage,
    resourceUsage,
    abnormalWorkloads,
    alertStats,
    podDistribution,
    nodeDistribution,
    cpuDistribution,
    memoryDistribution,
    totalNodes,
    totalPods,
    totalCPU,
    totalMemory,
    podTrendData,
    nodeTrendData,
    formatNumber,
    getPieConfig,
    getTrendConfig,
    navigate,
  } = useOverview();

  if (loading && !stats) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 'calc(100vh - 200px)' }}>
        <Spin size="large" tip={t('messages.loading')} />
      </div>
    );
  }

  return (
    <div style={{ padding: '0 4px' }}>
      <OverviewToolbar
        loading={loading}
        lastRefreshTime={lastRefreshTime}
        autoRefresh={autoRefresh}
        refreshInterval={refreshInterval}
        onAutoRefreshChange={setAutoRefresh}
        onRefreshIntervalChange={setRefreshInterval}
        onRefresh={fetchData}
      />

      <StatCards
        clusterStats={clusterStats}
        nodeStats={nodeStats}
        podStats={podStats}
        alertStats={alertStats}
        formatNumber={formatNumber}
        onNavigate={navigate}
      />

      <ResourceUsageCards
        cpuUsage={cpuUsage}
        memoryUsage={memoryUsage}
        storageUsage={storageUsage}
        totalCPU={totalCPU}
        totalMemory={totalMemory}
        resourceUsage={resourceUsage}
        formatNumber={formatNumber}
      />

      <VersionAndAbnormal
        versionDistribution={versionDistribution}
        abnormalWorkloads={abnormalWorkloads}
        onNavigate={navigate}
      />

      <DistributionCharts
        podDistribution={podDistribution}
        nodeDistribution={nodeDistribution}
        cpuDistribution={cpuDistribution}
        memoryDistribution={memoryDistribution}
        totalPods={totalPods}
        totalNodes={totalNodes}
        totalCPU={totalCPU}
        totalMemory={totalMemory}
        formatNumber={formatNumber}
        getPieConfig={getPieConfig}
      />

      <TrendCharts
        podTrendData={podTrendData}
        nodeTrendData={nodeTrendData}
        podTimeRange={podTimeRange}
        nodeTimeRange={nodeTimeRange}
        onPodTimeRangeChange={setPodTimeRange}
        onNodeTimeRangeChange={setNodeTimeRange}
        getTrendConfig={getTrendConfig}
      />
    </div>
  );
};

export default Overview;
