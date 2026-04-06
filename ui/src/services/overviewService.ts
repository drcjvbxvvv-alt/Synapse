import { request } from '../utils/api';

// ========== 型別定義 ==========

// 叢集統計資料
export interface ClusterStatsData {
  total: number;
  healthy: number;
  unhealthy: number;
  unknown: number;
}

// 節點統計資料
export interface NodeStatsData {
  total: number;
  ready: number;
  notReady: number;
}

// Pod 統計資料
export interface PodStatsData {
  total: number;
  running: number;
  pending: number;
  failed: number;
  succeeded: number;
}

// 版本分佈
export interface VersionDistribution {
  version: string;
  count: number;
  clusters: string[];
}

// 總覽統計響應
export interface OverviewStatsResponse {
  clusterStats: ClusterStatsData;
  nodeStats: NodeStatsData;
  podStats: PodStatsData;
  versionDistribution: VersionDistribution[];
}

// 資源使用資料
export interface ResourceUsageData {
  usagePercent: number;
  used: number;
  total: number;
  unit: string;
}

// 資源使用率響應
export interface ResourceUsageResponse {
  cpu: ResourceUsageData;
  memory: ResourceUsageData;
  storage: ResourceUsageData;
}

// 叢集資源計數
export interface ClusterResourceCount {
  clusterId: number;
  clusterName: string;
  value: number;
}

// 資源分佈響應
export interface ResourceDistributionResponse {
  podDistribution: ClusterResourceCount[];
  nodeDistribution: ClusterResourceCount[];
  cpuDistribution: ClusterResourceCount[];
  memoryDistribution: ClusterResourceCount[];
}

// 資料點
export interface DataPoint {
  timestamp: number;
  value: number;
}

// 叢集趨勢序列
export interface ClusterTrendSeries {
  clusterId: number;
  clusterName: string;
  dataPoints: DataPoint[];
}

// 趨勢資料響應
export interface TrendResponse {
  podTrends: ClusterTrendSeries[];
  nodeTrends: ClusterTrendSeries[];
}

// 異常工作負載
export interface AbnormalWorkload {
  name: string;
  namespace: string;
  clusterId: number;
  clusterName: string;
  type: string;
  reason: string;
  message: string;
  duration: string;
  severity: 'warning' | 'critical';
}

// 叢集告警計數
export interface ClusterAlertCount {
  clusterId: number;
  clusterName: string;
  total: number;
  firing: number;
}

// 全域性告警統計
export interface GlobalAlertStats {
  total: number;
  firing: number;
  pending: number;
  resolved: number;
  suppressed: number;
  bySeverity: Record<string, number>;
  byCluster: ClusterAlertCount[];
  enabledCount: number;
}

// ========== API 服務 ==========

export const overviewService = {
  // 獲取總覽統計資料
  getStats: () => {
    return request.get<OverviewStatsResponse>('/overview/stats');
  },

  // 獲取資源使用率
  getResourceUsage: () => {
    return request.get<ResourceUsageResponse>('/overview/resource-usage');
  },

  // 獲取資源分佈
  getDistribution: () => {
    return request.get<ResourceDistributionResponse>('/overview/distribution');
  },

  // 獲取趨勢資料
  getTrends: (params?: { timeRange?: string; step?: string }) => {
    return request.get<TrendResponse>('/overview/trends', { params });
  },

  // 獲取異常工作負載
  getAbnormalWorkloads: (params?: { limit?: number }) => {
    return request.get<AbnormalWorkload[]>('/overview/abnormal-workloads', { params });
  },

  // 獲取全域性告警統計
  getAlertStats: () => {
    return request.get<GlobalAlertStats>('/overview/alert-stats');
  },
};

