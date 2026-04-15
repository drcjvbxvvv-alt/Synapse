import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface GitOpsApp {
  id: number;
  name: string;
  source: 'native' | 'argocd';
  namespace: string;
  repo_url?: string;
  branch?: string;
  path?: string;
  render_type?: string;
  sync_policy?: string;
  status?: string;
  sync_status?: string;
  health_status?: string;
  diff_summary?: string;
}

export interface CreateGitOpsAppRequest {
  name: string;
  source: 'native';
  git_provider_id?: number;
  repo_url: string;
  branch: string;
  path: string;
  render_type: 'raw' | 'kustomize' | 'helm';
  helm_values?: string;
  namespace: string;
  sync_policy: 'auto' | 'manual';
  sync_interval?: number;
}

export interface UpdateGitOpsAppRequest {
  repo_url?: string;
  branch?: string;
  path?: string;
  render_type?: string;
  helm_values?: string;
  sync_policy?: string;
  sync_interval?: number;
}

export interface DiffResult {
  resources: Array<{
    group: string;
    version: string;
    kind: string;
    name: string;
    namespace: string;
    action: 'added' | 'modified' | 'deleted' | 'unchanged';
    diff?: string;
  }>;
}

// ─── Service ───────────────────────────────────────────────────────────────

const gitopsService = {
  list(clusterId: number, source?: string): Promise<{ items: GitOpsApp[]; total: number }> {
    const params = source ? `?source=${source}` : '';
    return request.get(`/clusters/${clusterId}/gitops/apps${params}`);
  },

  get(clusterId: number, id: number): Promise<GitOpsApp> {
    return request.get(`/clusters/${clusterId}/gitops/apps/${id}`);
  },

  create(clusterId: number, data: CreateGitOpsAppRequest): Promise<GitOpsApp> {
    return request.post(`/clusters/${clusterId}/gitops/apps`, data);
  },

  update(clusterId: number, id: number, data: UpdateGitOpsAppRequest): Promise<void> {
    return request.put(`/clusters/${clusterId}/gitops/apps/${id}`, data);
  },

  delete(clusterId: number, id: number): Promise<void> {
    return request.delete(`/clusters/${clusterId}/gitops/apps/${id}`);
  },

  triggerSync(clusterId: number, id: number): Promise<void> {
    return request.post(`/clusters/${clusterId}/gitops/apps/${id}/sync`);
  },

  getDiff(clusterId: number, id: number): Promise<DiffResult> {
    return request.get(`/clusters/${clusterId}/gitops/apps/${id}/diff`);
  },
};

export default gitopsService;
