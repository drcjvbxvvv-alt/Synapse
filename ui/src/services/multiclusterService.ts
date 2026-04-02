import { request } from '../utils/api';

export interface MigrateRequest {
  sourceClusterId: string;
  sourceNamespace: string;
  workloadKind: string;
  workloadName: string;
  targetClusterId: string;
  targetNamespace: string;
  syncConfigMaps?: boolean;
  syncSecrets?: boolean;
}

export interface MigrateCheckResult {
  feasible: boolean;
  message: string;
  workloadCpuReq: number;
  workloadMemReq: number;
  targetFreeCpu: number;
  targetFreeMem: number;
  configMapCount: number;
  secretCount: number;
}

export interface MigrateResult {
  success: boolean;
  workloadCreated: boolean;
  configMapsSynced: string[];
  secretsSynced: string[];
  message: string;
}

export interface SyncPolicy {
  id?: number;
  name: string;
  description?: string;
  source_cluster_id: number;
  source_namespace: string;
  resource_type: string; // ConfigMap | Secret
  resource_names: string; // JSON array string
  target_clusters: string; // JSON array of cluster IDs
  conflict_policy: string; // overwrite | skip
  schedule?: string; // cron expr or empty
  enabled?: boolean;
  last_sync_at?: string;
  last_sync_status?: string;
  created_at?: string;
  updated_at?: string;
}

export interface SyncHistory {
  id: number;
  policy_id: number;
  triggered_by: string;
  status: string;
  message: string;
  details: string;
  started_at: string;
  finished_at?: string;
}

export const multiclusterService = {
  // 遷移
  migrateCheck: (data: MigrateRequest) =>
    request.post('/multicluster/migrate/check', data),
  migrate: (data: MigrateRequest) =>
    request.post('/multicluster/migrate', data),

  // 同步策略
  listSyncPolicies: () =>
    request.get('/multicluster/sync-policies'),
  createSyncPolicy: (data: Partial<SyncPolicy>) =>
    request.post('/multicluster/sync-policies', data),
  getSyncPolicy: (id: number) =>
    request.get(`/multicluster/sync-policies/${id}`),
  updateSyncPolicy: (id: number, data: Partial<SyncPolicy>) =>
    request.put(`/multicluster/sync-policies/${id}`, data),
  deleteSyncPolicy: (id: number) =>
    request.delete(`/multicluster/sync-policies/${id}`),
  triggerSync: (id: number) =>
    request.post(`/multicluster/sync-policies/${id}/trigger`, {}),
  getSyncHistory: (id: number) =>
    request.get(`/multicluster/sync-policies/${id}/history`),
};
