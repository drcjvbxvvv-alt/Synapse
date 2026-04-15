import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface RolloutInfo {
  id: string;
  name: string;
  namespace: string;
  type: string;
  status: 'Healthy' | 'Stopped' | 'Degraded' | string;
  replicas: number;
  ready_replicas: number;
  available_replicas: number;
  updated_replicas: number;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  selector: Record<string, string>;
  created_at: string;
  images: string[];
  strategy: 'Canary' | 'BlueGreen' | string;
  // Canary-specific
  current_weight?: number;
  desired_weight?: number;
  current_step_index?: number;
  current_step_count?: number;
  stable_rs?: string;
  canary_rs?: string;
  // BlueGreen-specific
  active_selector?: string;
  preview_selector?: string;
}

export interface AnalysisRun {
  name: string;
  namespace: string;
  status: string;
  started_at: string;
  completed_at: string | null;
  metrics: AnalysisMetric[];
}

export interface AnalysisMetric {
  name: string;
  phase: string;
  successful: number;
  failed: number;
  error: number;
}

export interface RolloutListResponse {
  items: RolloutInfo[];
  total: number;
}

export interface CRDCheckResponse {
  installed: boolean;
  version: string;
}

// ─── Service ───────────────────────────────────────────────────────────────

const rolloutService = {
  checkCRD(clusterId: number): Promise<CRDCheckResponse> {
    return request.get(`/clusters/${clusterId}/rollouts/crd-check`);
  },

  list(
    clusterId: number,
    params?: { namespace?: string; search?: string; page?: number; pageSize?: number },
  ): Promise<RolloutListResponse> {
    return request.get(`/clusters/${clusterId}/rollouts`, { params });
  },

  getNamespaces(clusterId: number): Promise<{ namespaces: string[] }> {
    return request.get(`/clusters/${clusterId}/rollouts/namespaces`);
  },

  get(clusterId: number, namespace: string, name: string): Promise<RolloutInfo> {
    return request.get(`/clusters/${clusterId}/rollouts/${namespace}/${name}`);
  },

  promote(clusterId: number, namespace: string, name: string): Promise<void> {
    return request.post(`/clusters/${clusterId}/rollouts/${namespace}/${name}/promote`);
  },

  promoteFull(clusterId: number, namespace: string, name: string): Promise<void> {
    return request.post(`/clusters/${clusterId}/rollouts/${namespace}/${name}/promote-full`);
  },

  abort(clusterId: number, namespace: string, name: string): Promise<void> {
    return request.post(`/clusters/${clusterId}/rollouts/${namespace}/${name}/abort`);
  },

  getAnalysisRuns(clusterId: number, namespace: string, name: string): Promise<{ items: AnalysisRun[] }> {
    return request.get(`/clusters/${clusterId}/rollouts/${namespace}/${name}/analysis-runs`);
  },

  delete(clusterId: number, namespace: string, name: string): Promise<void> {
    return request.delete(`/clusters/${clusterId}/rollouts/${namespace}/${name}`);
  },
};

export default rolloutService;
