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

// ---- Phase 2 效率分析型別 ----

export interface NamespaceEfficiency {
  namespace: string;
  cpu_request_millicores: number;
  memory_request_mib: number;
  cpu_usage_millicores: number;
  memory_usage_mib: number;
  cpu_occupancy_percent: number;
  memory_occupancy_percent: number;
  cpu_efficiency: number;    // 0-1
  memory_efficiency: number; // 0-1
  pod_count: number;
  has_metrics: boolean;
}

export interface RightSizingRecommendation {
  cpu_recommended_millicores: number;
  memory_recommended_mib: number;
  confidence: string; // "medium"
}

export interface WorkloadEfficiency {
  namespace: string;
  name: string;
  kind: string;
  replicas: number;
  cpu_request_millicores: number;
  cpu_usage_millicores: number;
  cpu_efficiency: number;
  memory_request_mib: number;
  memory_usage_mib: number;
  memory_efficiency: number;
  waste_score: number; // 0-1
  has_metrics: boolean;
  rightsizing?: RightSizingRecommendation;
}

// ---- Phase 3 型別 ----

export interface CapacityTrendPoint {
  month: string;                  // "2026-01"
  cpu_occupancy_percent: number;
  memory_occupancy_percent: number;
  node_count: number;
}

export interface ForecastResult {
  based_on_months: number;
  current_cpu_percent: number;
  current_memory_percent: number;
  cpu_80_percent_date: string | null;
  cpu_100_percent_date: string | null;
  memory_80_percent_date: string | null;
  memory_100_percent_date: string | null;
}

export interface WorkloadEfficiencyPage {
  items: WorkloadEfficiency[];
  total: number;
}

export const ResourceService = {
  getSnapshot: (clusterId: string): Promise<ClusterResourceSnapshot> =>
    request.get(`/clusters/${clusterId}/resources/snapshot`),

  getNamespaceOccupancy: (clusterId: string): Promise<NamespaceOccupancy[]> =>
    request.get(`/clusters/${clusterId}/resources/namespaces`),

  getGlobalOverview: (): Promise<GlobalResourceOverview> =>
    request.get('/resources/global/overview'),

  // Phase 2
  getNamespaceEfficiency: (clusterId: string): Promise<NamespaceEfficiency[]> =>
    request.get(`/clusters/${clusterId}/resources/efficiency`),

  getWorkloadEfficiency: (
    clusterId: string,
    namespace?: string,
    page = 1,
    pageSize = 20,
  ): Promise<WorkloadEfficiencyPage> => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (namespace) params.set('namespace', namespace);
    return request.get(`/clusters/${clusterId}/resources/workloads?${params}`);
  },

  getWasteWorkloads: (clusterId: string, cpuThreshold = 0.2): Promise<WorkloadEfficiency[]> =>
    request.get(`/clusters/${clusterId}/resources/waste?cpu_threshold=${cpuThreshold}`),

  // Phase 3
  getTrend: (clusterId: string, months = 6): Promise<CapacityTrendPoint[]> =>
    request.get(`/clusters/${clusterId}/resources/trend?months=${months}`),

  getForecast: (clusterId: string, days = 180): Promise<ForecastResult> =>
    request.get(`/clusters/${clusterId}/resources/forecast?days=${days}`),

  getWasteExportURL: (clusterId: string, cpuThreshold = 0.2): string =>
    `/api/v1/clusters/${clusterId}/resources/waste/export?cpu_threshold=${cpuThreshold}`,
};

// ---- Namespace Budget (6.6) 型別 ----

export interface NamespaceBudget {
  id?: number;
  cluster_id: number;
  namespace: string;
  cpu_cores_limit: number;
  memory_gib_limit: number;
  monthly_cost_limit: number;
  alert_threshold: number;
  enabled: boolean;
}

export interface BudgetAlert {
  namespace: string;
  resource: string;
  limit: number;
  current: number;
  usage_percent: number;
  alert_threshold: number;
  exceeded: boolean;
  alert: boolean;
}

export interface BudgetStatus {
  budget: NamespaceBudget;
  alerts: BudgetAlert[];
  status: string;
}

export const costService = {
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

  // ---- Namespace Budgets (6.6) ----

  listBudgets: (clusterId: string): Promise<NamespaceBudget[]> =>
    request.get(`/clusters/${clusterId}/cost/budgets`),

  upsertBudget: (clusterId: string, namespace: string, budget: Partial<NamespaceBudget>): Promise<NamespaceBudget> =>
    request.put(`/clusters/${clusterId}/cost/budgets/${namespace}`, budget),

  deleteBudget: (clusterId: string, namespace: string): Promise<void> =>
    request.delete(`/clusters/${clusterId}/cost/budgets/${namespace}`),

  checkBudgets: (clusterId: string): Promise<BudgetStatus[]> =>
    request.get(`/clusters/${clusterId}/cost/budgets/check`),
};
