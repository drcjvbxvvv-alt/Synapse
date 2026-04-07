import { request } from '../utils/api';
import type { PaginatedResponse } from '../types';

export interface CostConfig {
  id?: number;
  cluster_id?: number;
  cpu_price_per_core: number;
  mem_price_per_gib: number;
  currency: string;
}

export interface CostItem {
  name: string;
  cpu_request: number;
  cpu_usage: number;
  cpu_util: number;
  mem_request: number;
  mem_usage: number;
  mem_util: number;
  pod_count: number;
  est_cost: number;
  currency: string;
}

export interface CostOverview {
  month: string;
  total_cost: number;
  currency: string;
  top_namespace: string;
  waste_percent: number;
  snapshot_count: number;
  config: CostConfig;
}

export interface TrendPoint {
  month: string;
  total: number;
  breakdown: { namespace: string; cost: number }[];
}

export interface WasteItem {
  namespace: string;
  workload: string;
  cpu_request: number;
  cpu_avg_usage: number;
  cpu_util: number;
  mem_request: number;
  mem_avg_usage: number;
  mem_util: number;
  wasted_cost: number;
  currency: string;
  days: number;
}

// ---- 資源治理（Phase 1）型別 ----

export interface ResourceMetrics {
  cpu_millicores: number;
  memory_mib: number;
}

export interface OccupancyPercent {
  cpu: number;
  memory: number;
}

export interface ClusterResourceSnapshot {
  cluster_id: number;
  collected_at: string;
  allocatable: ResourceMetrics;
  requested: ResourceMetrics;
  occupancy: OccupancyPercent;
  headroom: ResourceMetrics;
  node_count: number;
  pod_count: number;
  has_metrics: boolean;
}

export interface NamespaceOccupancy {
  namespace: string;
  cpu_request_millicores: number;
  memory_request_mib: number;
  cpu_occupancy_percent: number;
  memory_occupancy_percent: number;
  pod_count: number;
}

export interface ClusterResourceSummary {
  cluster_id: number;
  cluster_name: string;
  cpu_occupancy_percent: number;
  memory_occupancy_percent: number;
  allocatable_cpu_millicores: number;
  allocatable_memory_mib: number;
  requested_cpu_millicores: number;
  requested_memory_mib: number;
  node_count: number;
  pod_count: number;
  informer_ready: boolean;
}

export interface GlobalResourceOverview {
  collected_at: string;
  cluster_count: number;
  ready_count: number;
  avg_cpu_occupancy_percent: number;
  avg_memory_occupancy_percent: number;
  clusters: ClusterResourceSummary[];
}

export const ResourceService = {
  getSnapshot: (clusterId: string): Promise<ClusterResourceSnapshot> =>
    request.get(`/clusters/${clusterId}/resources/snapshot`),

  getNamespaceOccupancy: (clusterId: string): Promise<NamespaceOccupancy[]> =>
    request.get(`/clusters/${clusterId}/resources/namespaces`),

  getGlobalOverview: (): Promise<GlobalResourceOverview> =>
    request.get('/resources/global/overview'),
};

export const CostService = {
  getConfig: (clusterId: string): Promise<CostConfig> =>
    request.get(`/clusters/${clusterId}/cost/config`),

  updateConfig: (clusterId: string, cfg: CostConfig): Promise<CostConfig> =>
    request.put(`/clusters/${clusterId}/cost/config`, cfg),

  getOverview: (clusterId: string, month?: string): Promise<CostOverview> =>
    request.get(`/clusters/${clusterId}/cost/overview${month ? `?month=${month}` : ''}`),

  getNamespaceCosts: (clusterId: string, month?: string): Promise<CostItem[]> =>
    request.get(`/clusters/${clusterId}/cost/namespaces${month ? `?month=${month}` : ''}`),

  getWorkloadCosts: (
    clusterId: string,
    month?: string,
    namespace?: string,
    page = 1,
    pageSize = 20,
  ): Promise<PaginatedResponse<CostItem>> => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (month) params.set('month', month);
    if (namespace && namespace !== '_all_') params.set('namespace', namespace);
    return request.get(`/clusters/${clusterId}/cost/workloads?${params}`);
  },

  getTrend: (clusterId: string, months = 6): Promise<TrendPoint[]> =>
    request.get(`/clusters/${clusterId}/cost/trend?months=${months}`),

  getWaste: (clusterId: string): Promise<WasteItem[]> =>
    request.get(`/clusters/${clusterId}/cost/waste`),

  getExportURL: (clusterId: string, month?: string): string => {
    const m = month ?? new Date().toISOString().slice(0, 7);
    return `/api/v1/clusters/${clusterId}/cost/export?month=${m}`;
  },
};
