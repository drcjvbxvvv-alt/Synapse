import type { ApiResponse, Node, PaginatedResponse } from '../types';
import { request } from '../utils/api';

export interface NodeListParams {
  clusterId: string;
  page?: number;
  pageSize?: number;
  status?: string;
  search?: string;
}

export interface NodeOverview {
  totalNodes: number;
  readyNodes: number;
  notReadyNodes: number;
  maintenanceNodes: number;
  cpuUsage: number;
  memoryUsage: number;
  storageUsage: number;
}

export const nodeService = {
  // 獲取節點列表
  getNodes: async (params: NodeListParams): Promise<ApiResponse<PaginatedResponse<Node>>> => {
    const { clusterId, page = 1, pageSize = 10, status, search } = params;
    const queryParams = new URLSearchParams();
    
    if (page) queryParams.append('page', page.toString());
    if (pageSize) queryParams.append('pageSize', pageSize.toString());
    if (status) queryParams.append('status', status);
    if (search) queryParams.append('search', search);
    
    return request.get(`/clusters/${clusterId}/nodes?${queryParams.toString()}`);
  },

  // 獲取節點詳情
  getNode: async (clusterId: string, name: string): Promise<ApiResponse<Node>> => {
    return request.get(`/clusters/${clusterId}/nodes/${name}`);
  },

  // 獲取節點概覽資訊
  getNodeOverview: async (clusterId: string): Promise<ApiResponse<NodeOverview>> => {
    return request.get(`/clusters/${clusterId}/nodes/overview`);
  },

  // 封鎖節點 (Cordon)
  cordonNode: async (clusterId: string, name: string): Promise<ApiResponse<null>> => {
    return request.post(`/clusters/${clusterId}/nodes/${name}/cordon`);
  },

  // 解封節點 (Uncordon)
  uncordonNode: async (clusterId: string, name: string): Promise<ApiResponse<null>> => {
    return request.post(`/clusters/${clusterId}/nodes/${name}/uncordon`);
  },

  // 新增 / 更新節點標籤
  patchNodeLabels: async (clusterId: string, name: string, labels: Record<string, string>): Promise<void> => {
    await request.patch(`/clusters/${clusterId}/nodes/${name}/labels`, { labels });
  },

  // 替換節點污點列表（傳入空陣列 = 清除所有污點）
  patchNodeTaints: async (
    clusterId: string,
    name: string,
    taints: Array<{ key: string; value?: string; effect: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute' }>
  ): Promise<void> => {
    await request.patch(`/clusters/${clusterId}/nodes/${name}/taints`, { taints });
  },

  // 驅逐節點 (Drain)
  drainNode: async (
    clusterId: string, 
    name: string, 
    options: {
      ignoreDaemonSets?: boolean;
      deleteLocalData?: boolean;
      force?: boolean;
      gracePeriodSeconds?: number;
    } = {}
  ): Promise<ApiResponse<null>> => {
    return request.post(`/clusters/${clusterId}/nodes/${name}/drain`, options);
  },
};