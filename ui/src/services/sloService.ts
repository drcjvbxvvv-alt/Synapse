import { request } from '../utils/api';

// ── Types ────────────────────────────────────────────────────────────────────

export type SLIType = 'availability' | 'latency' | 'error_rate' | 'custom';
export type SLOWindow = '7d' | '28d' | '30d';
export type SLOStatusValue = 'ok' | 'warning' | 'critical' | 'unknown';

export interface SLO {
  id: number;
  cluster_id: number;
  name: string;
  description: string;
  namespace: string;
  sli_type: SLIType;
  prom_query: string;
  total_query: string;
  target: number;        // 0-1
  window: SLOWindow;
  burn_rate_warning: number;
  burn_rate_critical: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface SLOStatus {
  slo_id: number;
  sli_value: number | null;
  sli_percent: number | null;
  error_budget_total: number;
  error_budget_used: number;
  error_budget_remaining: number;
  burn_rate_1h: number | null;
  burn_rate_6h: number | null;
  burn_rate_24h: number | null;
  burn_rate_window: number | null;
  status: SLOStatusValue;
  has_data: boolean;
  chaos_active: boolean;
}

export interface CreateSLOPayload {
  name: string;
  description?: string;
  namespace?: string;
  sli_type: SLIType;
  prom_query: string;
  total_query?: string;
  target: number;
  window: SLOWindow;
  burn_rate_warning?: number;
  burn_rate_critical?: number;
  enabled: boolean;
}

// ── API calls ────────────────────────────────────────────────────────────────

const base = (clusterID: number) => `/clusters/${clusterID}/slos`;

export const sloService = {
  list(clusterID: number, namespace?: string): Promise<{ items: SLO[]; total: number }> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return request.get(`${base(clusterID)}${params}`);
  },

  get(clusterID: number, id: number): Promise<SLO> {
    return request.get(`${base(clusterID)}/${id}`);
  },

  create(clusterID: number, payload: CreateSLOPayload): Promise<SLO> {
    return request.post(base(clusterID), payload);
  },

  update(clusterID: number, id: number, payload: CreateSLOPayload): Promise<SLO> {
    return request.put(`${base(clusterID)}/${id}`, payload);
  },

  delete(clusterID: number, id: number): Promise<void> {
    return request.delete(`${base(clusterID)}/${id}`);
  },

  getStatus(clusterID: number, id: number): Promise<SLOStatus> {
    return request.get(`${base(clusterID)}/${id}/status`);
  },
};
