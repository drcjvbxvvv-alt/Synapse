import axios from 'axios';

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
    axios.get<{ items: ImageIndex[]; total: number; page: number; limit: number }>('/api/v1/images/search', { params }),

  syncStatus: () =>
    axios.get<{ indexed: number; lastSyncAt: string }>('/api/v1/images/status'),

  sync: () =>
    axios.post<{ message: string; indexed: number }>('/api/v1/images/sync'),
};

export const crossClusterService = {
  listWorkloads: (params?: { kind?: string; name?: string; namespace?: string; cluster?: number }) =>
    axios.get<{ items: CrossClusterWorkload[]; total: number }>('/api/v1/workloads', { params }),

  getStats: () =>
    axios.get<{ clusters: Array<{ clusterId: number; clusterName: string; deployments: number; statefulSets: number; daemonSets: number; degraded: number }> }>(
      '/api/v1/workloads/stats'
    ),
};
