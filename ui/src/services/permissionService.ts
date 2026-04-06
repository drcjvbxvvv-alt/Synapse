import api from '../utils/api';
import type {
  ApiResponse,
  PermissionTypeInfo,
  UserGroup,
  ClusterPermission,
  CreateClusterPermissionRequest,
  UpdateClusterPermissionRequest,
  MyPermissionsResponse,
  CreateUserGroupRequest,
  UpdateUserGroupRequest,
  User,
} from '../types';

const BASE_URL = '/permissions';

// ========== 權限型別 ==========

// 獲取權限型別列表
export const getPermissionTypes = async (): Promise<ApiResponse<PermissionTypeInfo[]>> => {
  const response = await api.get(`${BASE_URL}/types`);
  return response.data;
};

// ========== 使用者列表 ==========

// 獲取使用者列表（用於權限分配）
export const getUsers = async (): Promise<User[]> => {
  const response = await api.get(`${BASE_URL}/users`);
  return response.data?.items ?? response.data ?? [];
};

// ========== 使用者組管理 ==========

// 獲取使用者組列表
export const getUserGroups = async (): Promise<UserGroup[]> => {
  const response = await api.get(`${BASE_URL}/user-groups`);
  return response.data?.items ?? response.data ?? [];
};

// 獲取使用者組詳情
export const getUserGroup = async (id: number): Promise<ApiResponse<UserGroup>> => {
  const response = await api.get(`${BASE_URL}/user-groups/${id}`);
  return response.data;
};

// 建立使用者組
export const createUserGroup = async (data: CreateUserGroupRequest): Promise<ApiResponse<UserGroup>> => {
  const response = await api.post(`${BASE_URL}/user-groups`, data);
  return response.data;
};

// 更新使用者組
export const updateUserGroup = async (id: number, data: UpdateUserGroupRequest): Promise<ApiResponse<UserGroup>> => {
  const response = await api.put(`${BASE_URL}/user-groups/${id}`, data);
  return response.data;
};

// 刪除使用者組
export const deleteUserGroup = async (id: number): Promise<ApiResponse<null>> => {
  const response = await api.delete(`${BASE_URL}/user-groups/${id}`);
  return response.data;
};

// 新增使用者到使用者組
export const addUserToGroup = async (groupId: number, userId: number): Promise<ApiResponse<null>> => {
  const response = await api.post(`${BASE_URL}/user-groups/${groupId}/users`, { user_id: userId });
  return response.data;
};

// 從使用者組移除使用者
export const removeUserFromGroup = async (groupId: number, userId: number): Promise<ApiResponse<null>> => {
  const response = await api.delete(`${BASE_URL}/user-groups/${groupId}/users/${userId}`);
  return response.data;
};

// ========== 叢集權限管理 ==========

// 獲取所有叢集權限列表
export const getAllClusterPermissions = async (): Promise<ClusterPermission[]> => {
  const response = await api.get(`${BASE_URL}/cluster-permissions`);
  return response.data?.items ?? response.data ?? [];
};

// 獲取指定叢集的權限列表
export const getClusterPermissions = async (clusterId: number): Promise<ClusterPermission[]> => {
  const response = await api.get(`${BASE_URL}/cluster-permissions`, {
    params: { cluster_id: clusterId }
  });
  return response.data?.items ?? response.data ?? [];
};

// 獲取權限詳情
export const getClusterPermission = async (id: number): Promise<ApiResponse<ClusterPermission>> => {
  const response = await api.get(`${BASE_URL}/cluster-permissions/${id}`);
  return response.data;
};

// 建立叢集權限
export const createClusterPermission = async (data: CreateClusterPermissionRequest): Promise<ApiResponse<ClusterPermission>> => {
  const response = await api.post(`${BASE_URL}/cluster-permissions`, data);
  return response.data;
};

// 更新叢集權限
export const updateClusterPermission = async (id: number, data: UpdateClusterPermissionRequest): Promise<ApiResponse<ClusterPermission>> => {
  const response = await api.put(`${BASE_URL}/cluster-permissions/${id}`, data);
  return response.data;
};

// 刪除叢集權限
export const deleteClusterPermission = async (id: number): Promise<ApiResponse<null>> => {
  const response = await api.delete(`${BASE_URL}/cluster-permissions/${id}`);
  return response.data;
};

// 批次刪除叢集權限
export const batchDeleteClusterPermissions = async (ids: number[]): Promise<ApiResponse<null>> => {
  const response = await api.post(`${BASE_URL}/cluster-permissions/batch-delete`, { ids });
  return response.data;
};

// ========== 當前使用者權限 ==========

// 獲取當前使用者的所有權限
export const getMyPermissions = async (): Promise<ApiResponse<MyPermissionsResponse[]>> => {
  const response = await api.get(`${BASE_URL}/my-permissions`);
  return response.data;
};

// 獲取當前使用者在指定叢集的權限
export const getMyClusterPermission = async (clusterId: number | string): Promise<ApiResponse<MyPermissionsResponse>> => {
  const response = await api.get(`/api/v1/clusters/${clusterId}/my-permissions`);
  return response.data;
};

// ========== 權限工具函式 ==========

// 權限型別顯示名稱對映
export const permissionTypeNames: Record<string, string> = {
  admin: '管理員權限',
  ops: '運維權限',
  dev: '開發權限',
  readonly: '只讀權限',
  custom: '自定義權限',
};

// 權限型別顏色對映
export const permissionTypeColors: Record<string, string> = {
  admin: 'red',
  ops: 'orange',
  dev: 'blue',
  readonly: 'green',
  custom: 'purple',
};

// 獲取權限型別顯示名稱
export const getPermissionTypeName = (type: string): string => {
  return permissionTypeNames[type] || type;
};

// 獲取權限型別顏色
export const getPermissionTypeColor = (type: string): string => {
  return permissionTypeColors[type] || 'default';
};

// 檢查是否有全部命名空間權限
export const hasAllNamespaceAccess = (namespaces: string[]): boolean => {
  return namespaces.includes('*');
};

// 格式化命名空間顯示
export const formatNamespaces = (namespaces: string[]): string => {
  if (hasAllNamespaceAccess(namespaces)) {
    return '全部命名空間';
  }
  return namespaces.join(', ');
};

// 權限型別是否允許部分命名空間
export const permissionTypeAllowsPartialNamespaces: Record<string, boolean> = {
  admin: false,  // 管理員必須是全部命名空間
  ops: false,    // 運維必須是全部命名空間  
  dev: true,     // 開發可以選擇部分命名空間
  readonly: true, // 只讀可以選擇部分命名空間
  custom: true,  // 自定義可以選擇部分命名空間
};

// 權限型別是否必須全部命名空間
export const permissionTypeRequiresAllNamespaces: Record<string, boolean> = {
  admin: true,   // 管理員必須是全部命名空間
  ops: true,     // 運維必須是全部命名空間
  dev: false,
  readonly: false,
  custom: false,
};

// 檢查權限型別是否允許選擇部分命名空間
export const allowsPartialNamespaces = (permissionType: string): boolean => {
  return permissionTypeAllowsPartialNamespaces[permissionType] ?? true;
};

// 檢查權限型別是否必須選擇全部命名空間
export const requiresAllNamespaces = (permissionType: string): boolean => {
  return permissionTypeRequiresAllNamespaces[permissionType] ?? false;
};

export const permissionService = {
  getPermissionTypes,
  getUsers,
  getUserGroups,
  getUserGroup,
  createUserGroup,
  updateUserGroup,
  deleteUserGroup,
  addUserToGroup,
  removeUserFromGroup,
  getAllClusterPermissions,
  getClusterPermissions,
  getClusterPermission,
  createClusterPermission,
  updateClusterPermission,
  deleteClusterPermission,
  batchDeleteClusterPermissions,
  getMyPermissions,
  getMyClusterPermission,
};

export default permissionService;

