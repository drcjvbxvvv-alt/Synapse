import api from '../utils/api';

export interface ImageIndex {
  id: number;
  clusterID: number;
  clusterName: string;
  namespace: string;
  workloadKind: string;
  workloadName: string;
  containerName: string;
  image: string;
  imageName: string;
  imageTag: string;
  lastSyncAt: string;
}

export interface CrossClusterWorkload {
  clusterId: number;
  clusterName: string;
  namespace: string;
  kind: string;
  name: string;
  replicas: number;
  ready: number;
  images: string[];
  labels: Record<string, string>;
  createdAt: string;
  status: 'healthy' | 'degraded';
}

export const imageService = {
  search: (params: { q?: string; tag?: string; namespace?: string; cluster?: number; page?: number; limit?: number }) =>
    api.get<{ items: ImageIndex[]; total: number; page: number; limit: number }>('images/search', { params }),

  syncStatus: () =>
    api.get<{ indexed: number; lastSyncAt: string }>('images/status'),

  sync: () =>
    api.post<{ message: string; indexed: number }>('images/sync'),
};

export const crossClusterService = {
  listWorkloads: (params?: { kind?: string; name?: string; namespace?: string; cluster?: number }) =>
    api.get<{ items: CrossClusterWorkload[]; total: number }>('workloads', { params }),

  getStats: () =>
    api.get<{ clusters: Array<{ clusterId: number; clusterName: string; deployments: number; statefulSets: number; daemonSets: number; degraded: number }> }>(
      'workloads/stats'
    ),
};
