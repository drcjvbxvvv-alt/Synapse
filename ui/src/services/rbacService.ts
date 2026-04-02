import api from '../utils/api';
import type { ApiResponse } from '../types';

// 同步结果类型
export interface SyncResult {
  resource: string;
  name: string;
  action: string;
  error?: string;
}

export interface SyncPermissionsResult {
  success: boolean;
  results: SyncResult[];
  message: string;
}

// 同步状态类型
export interface ResourceStatus {
  resource: string;
  name: string;
  exists: boolean;
}

export interface SyncStatusResult {
  synced: boolean;
  resources: ResourceStatus[];
}

// ClusterRole 类型
export interface ClusterRoleItem {
  name: string;
  labels: Record<string, string>;
  created_at: string;
  rules_count: number;
  is_synapse: boolean;
}

// Synapse 预定义角色信息
export interface SynapseRoleInfo {
  name: string;
  description: string;
  rules_count: number;
}

// PolicyRule 类型（用于创建自定义 ClusterRole）
export interface PolicyRule {
  apiGroups: string[];
  resources: string[];
  verbs: string[];
  resourceNames?: string[];
  nonResourceURLs?: string[];
}

// 同步权限到集群
export const syncPermissions = async (clusterId: number): Promise<ApiResponse<SyncPermissionsResult>> => {
  const response = await api.post(`/clusters/${clusterId}/rbac/sync`);
  return response.data;
};

// 获取同步状态
export const getSyncStatus = async (clusterId: number): Promise<ApiResponse<SyncStatusResult>> => {
  const response = await api.get(`/clusters/${clusterId}/rbac/status`);
  return response.data;
};

// 获取集群中的 ClusterRole 列表
export const listClusterRoles = async (clusterId: number): Promise<ApiResponse<ClusterRoleItem[]>> => {
  const response = await api.get(`/clusters/${clusterId}/rbac/clusterroles`);
  return response.data;
};

// 创建自定义 ClusterRole
export const createCustomClusterRole = async (
  clusterId: number,
  name: string,
  rules: PolicyRule[]
): Promise<ApiResponse<null>> => {
  const response = await api.post(`/clusters/${clusterId}/rbac/clusterroles`, { name, rules });
  return response.data;
};

// 删除 ClusterRole
export const deleteClusterRole = async (clusterId: number, name: string): Promise<ApiResponse<null>> => {
  const response = await api.delete(`/clusters/${clusterId}/rbac/clusterroles/${name}`);
  return response.data;
};

// 获取 Synapse 预定义角色信息
export const getSynapseRoles = async (): Promise<ApiResponse<SynapseRoleInfo[]>> => {
  const response = await api.get('/permissions/synapse-roles');
  return response.data;
};

export const rbacService = {
  syncPermissions,
  getSyncStatus,
  listClusterRoles,
  createCustomClusterRole,
  deleteClusterRole,
  getSynapseRoles,
};

export default rbacService;

