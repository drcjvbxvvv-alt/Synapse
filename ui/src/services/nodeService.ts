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