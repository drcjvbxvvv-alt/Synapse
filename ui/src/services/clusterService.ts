import { request } from '../utils/api';
import type { Cluster, ClusterStats, PaginatedResponse, K8sEvent } from '../types';

export const clusterService = {
  // 獲取叢集列表
  getClusters: (params?: {
    page?: number;
    pageSize?: number;
    search?: string;
    status?: string;
  }) => {
    return request.get<PaginatedResponse<Cluster>>('/clusters', { params });
  },

  // 獲取叢集詳情
  getCluster: (clusterId: string) => {
    return request.get<Cluster>(`/clusters/${clusterId}`);
  },

  // 匯入叢集
  importCluster: (data: {
    name: string;
    apiServer: string;
    kubeconfig?: string;
    token?: string;
    caCert?: string;
  }) => {
    return request.post<Cluster>('/clusters/import', data);
  },

  // 刪除叢集
  deleteCluster: (clusterId: string) => {
    return request.delete(`/clusters/${clusterId}`);
  },

  // 獲取叢集統計資訊
  getClusterStats: () => {
    return request.get<ClusterStats>('/clusters/stats');
  },

  // 獲取叢集概覽資訊
  getClusterOverview: (clusterId: string) => {
    return request.get(`/clusters/${clusterId}/overview`);
  },

  // 獲取叢集監控資料
  getClusterMetrics: (clusterId: string, params: {
    range: string;
    step?: string;
  }) => {
    return request.get(`/clusters/${clusterId}/metrics`, { params });
  },

  // 測試叢集連線
  testConnection: (data: {
    apiServer: string;
    kubeconfig?: string;
    token?: string;
    caCert?: string;
  }) => {
    return request.post('/clusters/test-connection', data);
  },

  // 獲取叢集K8s事件
  getClusterEvents: (clusterId: string, params?: { search?: string; type?: string }) => {
    return request.get<K8sEvent[]>(`/clusters/${clusterId}/events`, { params });
  },
};