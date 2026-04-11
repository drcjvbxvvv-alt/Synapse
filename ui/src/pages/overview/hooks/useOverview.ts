import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import { overviewService } from '../../../services/overviewService';
import { POLL_INTERVALS } from '../../../config/queryConfig';
import type {
  OverviewStatsResponse,
  ResourceUsageResponse,
  ResourceDistributionResponse,
  TrendResponse,
  AbnormalWorkload,
  ClusterResourceCount,
  GlobalAlertStats,
} from '../../../services/overviewService';

// ─── Interfaces ───────────────────────────────────────────────────────────────

export interface ChartDistribution {
  name: string;
  value: number;
  clusterId?: number;
}

export interface TrendData {
  date: string;
  cluster: string;
  value: number;
}

// ─── Constants ────────────────────────────────────────────────────────────────

export const CHART_COLORS = [
  '#5B8FF9', '#5AD8A6', '#F6BD16', '#E86452', '#6DC8EC',
  '#945FB9', '#FF9845', '#1E9493', '#FF99C3', '#9270CA',
  '#269A99', '#BDD2FD', '#BDEFDB', '#C2C8D5', '#FFC9B7',
  '#A0DC2C', '#946DFF', '#626681', '#EB4185', '#36BFFA',
];

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useOverview() {
  const { t } = useTranslation(['overview', 'common']);
  const navigate = useNavigate();
  const { message } = App.useApp();

  const [loading, setLoading] = useState(true);
  const [podTimeRange, setPodTimeRange] = useState<'7d' | '30d'>('7d');
  const [nodeTimeRange, setNodeTimeRange] = useState<'7d' | '30d'>('7d');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState<number>(POLL_INTERVALS.overview / 1000);
  const [lastRefreshTime, setLastRefreshTime] = useState<Date>(new Date());

  // Data states
  const [stats, setStats] = useState<OverviewStatsResponse | null>(null);
  const [resourceUsage, setResourceUsage] = useState<ResourceUsageResponse | null>(null);
  const [distribution, setDistribution] = useState<ResourceDistributionResponse | null>(null);
  const [trends, setTrends] = useState<TrendResponse | null>(null);
  const [abnormalWorkloads, setAbnormalWorkloads] = useState<AbnormalWorkload[]>([]);
  const [alertStats, setAlertStats] = useState<GlobalAlertStats | null>(null);

  // Fetch all data
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [statsRes, usageRes, distRes, workloadsRes, alertStatsRes] = await Promise.all([
        overviewService.getStats(),
        overviewService.getResourceUsage(),
        overviewService.getDistribution(),
        overviewService.getAbnormalWorkloads({ limit: 20 }),
        overviewService.getAlertStats(),
      ]);

      setStats(statsRes);
      setResourceUsage(usageRes);
      setDistribution(distRes);
      setAbnormalWorkloads(Array.isArray(workloadsRes) ? workloadsRes : []);
      setAlertStats(alertStatsRes);
      setLastRefreshTime(new Date());
    } catch (error) {
      console.error('Failed to fetch overview data:', error);
      message.error(t('common:messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  // Fetch trend data
  const fetchTrends = useCallback(async (podRange: string, nodeRange: string) => {
    try {
      const longerRange = podRange === '30d' || nodeRange === '30d' ? '30d' : '7d';
      const trendsRes = await overviewService.getTrends({ timeRange: longerRange });

      setTrends({
        podTrends: trendsRes?.podTrends || [],
        nodeTrends: trendsRes?.nodeTrends || [],
      });
    } catch (error) {
      console.error('Failed to fetch trend data:', error);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    fetchTrends(podTimeRange, nodeTimeRange);
  }, [fetchTrends, podTimeRange, nodeTimeRange]);

  // Auto-refresh
  useEffect(() => {
    let timer: NodeJS.Timeout;
    if (autoRefresh) {
      timer = setInterval(() => {
        fetchData();
        fetchTrends(podTimeRange, nodeTimeRange);
      }, refreshInterval * 1000);
    }
    return () => {
      if (timer) clearInterval(timer);
    };
  }, [autoRefresh, refreshInterval, fetchData, fetchTrends, podTimeRange, nodeTimeRange]);

  // ─── Data transformations ─────────────────────────────────────────────────

  const convertToChartDistribution = (data: ClusterResourceCount[] | undefined): ChartDistribution[] => {
    if (!data) return [];
    return data.map(item => ({
      name: item.clusterName,
      value: item.value,
      clusterId: item.clusterId,
    }));
  };

  const podDistribution = convertToChartDistribution(distribution?.podDistribution);
  const nodeDistribution = convertToChartDistribution(distribution?.nodeDistribution);
  const cpuDistribution = convertToChartDistribution(distribution?.cpuDistribution);
  const memoryDistribution = convertToChartDistribution(distribution?.memoryDistribution);

  const totalNodes = nodeDistribution.reduce((sum, c) => sum + c.value, 0);
  const totalPods = podDistribution.reduce((sum, c) => sum + c.value, 0);
  const totalCPU = cpuDistribution.reduce((sum, c) => sum + c.value, 0);
  const totalMemory = memoryDistribution.reduce((sum, c) => sum + c.value, 0);

  const convertTrendData = (trendSeries: TrendResponse['podTrends'] | undefined): TrendData[] => {
    if (!trendSeries || trendSeries.length === 0) return [];
    const result: TrendData[] = [];
    trendSeries.forEach(series => {
      let lastValidValue = 0;
      series.dataPoints?.forEach(point => {
        const date = new Date(point.timestamp * 1000);
        const dateStr = `${(date.getMonth() + 1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')}`;
        let value = point.value;
        if (value === null || value === undefined || Number.isNaN(value)) {
          value = lastValidValue;
        } else {
          lastValidValue = value;
        }
        result.push({
          date: dateStr,
          cluster: series.clusterName,
          value: Math.round(value),
        });
      });
    });
    return result;
  };

  const podTrendData = convertTrendData(trends?.podTrends);
  const nodeTrendData = convertTrendData(trends?.nodeTrends);

  // ─── Derived stats ────────────────────────────────────────────────────────

  const clusterStats = stats?.clusterStats || { total: 0, healthy: 0, unhealthy: 0, unknown: 0 };
  const nodeStats = stats?.nodeStats || { total: 0, ready: 0, notReady: 0 };
  const podStats = stats?.podStats || { total: 0, running: 0, pending: 0, failed: 0, succeeded: 0 };
  const versionDistribution = Array.isArray(stats?.versionDistribution) ? (stats?.versionDistribution ?? []) : [];

  const cpuUsage = resourceUsage?.cpu?.usagePercent || 0;
  const memoryUsage = resourceUsage?.memory?.usagePercent || 0;
  const storageUsage = resourceUsage?.storage?.usagePercent || 0;

  // ─── Helpers ──────────────────────────────────────────────────────────────

  const formatNumber = (num: number, unit: string = '') => {
    if (num >= 10000) return `${(num / 10000).toFixed(2)}w${unit}`;
    return `${num}${unit}`;
  };

  // Pie chart config builder
  const getPieConfig = (data: ChartDistribution[], labelSuffix: string = '', title: string = '') => ({
    data,
    angleField: 'value',
    colorField: 'name',
    color: CHART_COLORS,
    radius: 0.85,
    innerRadius: 0.6,
    label: {
      type: 'spider',
      content: ({ value }: { value: number }) => `${value}${labelSuffix}`,
      style: { fontSize: 11 },
    },
    legend: {
      position: 'left' as const,
      layout: 'vertical' as const,
      itemWidth: 150,
      maxRow: 12,
      flipPage: false,
      itemName: { style: { fontSize: 12 } },
    },
    statistic: { title: false as const, content: false as const },
    interactions: [{ type: 'element-active' }, { type: 'pie-legend-active' }],
    state: { active: { style: { lineWidth: 2, stroke: '#fff' } } },
    tooltip: {
      showTitle: true,
      title: () => title || t('distribution.clusterDistribution'),
      customContent: (_: string, items: Array<{ name: string; value: string; color: string; data: ChartDistribution }>) => {
        if (!items || items.length === 0) return '';
        const item = items[0];
        const total = data.reduce((sum, d) => sum + d.value, 0);
        const percent = total > 0 ? ((parseFloat(item.value) / total) * 100).toFixed(1) : '0';
        return `
          <div style="padding: 10px 14px; min-width: 180px;">
            <div style="font-weight: 600; margin-bottom: 10px; color: #1f2937; border-bottom: 1px solid #e5e7eb; padding-bottom: 8px;">
              ${title}
            </div>
            <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 6px;">
              <span style="display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: ${item.color};"></span>
              <span style="color: #6b7280;">${t('distribution.cluster')}:</span>
              <span style="font-weight: 600; color: #1f2937;">${item.name}</span>
            </div>
            <div style="padding-left: 18px; color: #6b7280;">
              ${t('distribution.quantity')}: <span style="font-weight: 600; color: #3b82f6;">${item.value}${labelSuffix}</span>
              <span style="margin-left: 8px; color: #9ca3af;">(${percent}%)</span>
            </div>
          </div>
        `;
      },
    },
    onReady: (plot: { on: (event: string, callback: (evt: { data?: { data?: ChartDistribution } }) => void) => void }) => {
      plot.on('element:click', (evt: { data?: { data?: ChartDistribution } }) => {
        const clusterId = evt.data?.data?.clusterId;
        if (clusterId) {
          navigate(`/clusters/${clusterId}/overview`);
        }
      });
    },
  });

  // Line chart config builder
  const getTrendConfig = (data: TrendData[], yAxisTitle: string) => ({
    data,
    xField: 'date',
    yField: 'value',
    seriesField: 'cluster',
    color: ['#3b82f6', '#10b981', '#f59e0b', '#ef4444'],
    smooth: true,
    lineStyle: { lineWidth: 2 },
    point: { size: 3, shape: 'circle', style: { fill: '#fff', lineWidth: 2 } },
    legend: { position: 'top' as const, marker: { symbol: 'circle' } },
    yAxis: {
      title: { text: yAxisTitle, style: { fontSize: 12 } },
      grid: { line: { style: { stroke: '#f0f0f0', lineDash: [4, 4] } } },
    },
    xAxis: { title: { text: '' }, line: { style: { stroke: '#d9d9d9' } } },
    animation: { appear: { animation: 'path-in', duration: 800 } },
    colorField: 'cluster',
  });

  return {
    // State
    loading,
    stats,
    podTimeRange,
    nodeTimeRange,
    autoRefresh,
    refreshInterval,
    lastRefreshTime,
    // Setters
    setPodTimeRange,
    setNodeTimeRange,
    setAutoRefresh,
    setRefreshInterval,
    // Actions
    fetchData,
    // Derived data
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
    // Helpers
    formatNumber,
    getPieConfig,
    getTrendConfig,
    navigate,
  };
}
