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
