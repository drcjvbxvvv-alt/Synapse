import { createContext } from 'react';
import type { MyPermissionsResponse, PermissionType } from '../types';

// 权限上下文类型
export interface PermissionContextType {
  // 用户在所有集群的权限
  clusterPermissions: Map<number, MyPermissionsResponse>;
  // 当前集群权限
  currentClusterPermission: MyPermissionsResponse | null;
  // 加载状态
  loading: boolean;
  // 权限检查方法
  hasClusterAccess: (clusterId: number | string) => boolean;
  hasNamespaceAccess: (clusterId: number | string, namespace: string) => boolean;
  canPerformAction: (action: string, clusterId?: number | string) => boolean;
  isAdmin: (clusterId?: number | string) => boolean;
  isReadonly: (clusterId?: number | string) => boolean;
  canWrite: (clusterId?: number | string) => boolean; // 检查是否有写权限
  // 获取权限类型
  getPermissionType: (clusterId: number | string) => PermissionType | null;
  // 刷新权限
  refreshPermissions: () => Promise<void>;
  // 设置当前集群
  setCurrentClusterId: (clusterId: number | string | null) => void;
  // 命名空间权限相关
  getAllowedNamespaces: (clusterId?: number | string) => string[];
  hasAllNamespaceAccess: (clusterId?: number | string) => boolean;
  filterNamespaces: (namespaces: string[], clusterId?: number | string) => string[];
}

export const PermissionContext = createContext<PermissionContextType | null>(null);

