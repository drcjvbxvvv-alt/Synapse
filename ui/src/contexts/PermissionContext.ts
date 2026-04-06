import { createContext } from 'react';
import type { MyPermissionsResponse, PermissionType } from '../types';

// 權限上下文型別
export interface PermissionContextType {
  // 使用者在所有叢集的權限
  clusterPermissions: Map<number, MyPermissionsResponse>;
  // 當前叢集權限
  currentClusterPermission: MyPermissionsResponse | null;
  // 載入狀態
  loading: boolean;
  // 權限檢查方法
  hasClusterAccess: (clusterId: number | string) => boolean;
  hasNamespaceAccess: (clusterId: number | string, namespace: string) => boolean;
  canPerformAction: (action: string, clusterId?: number | string) => boolean;
  isAdmin: (clusterId?: number | string) => boolean;
  isReadonly: (clusterId?: number | string) => boolean;
  canWrite: (clusterId?: number | string) => boolean; // 檢查是否有寫權限
  // 獲取權限型別
  getPermissionType: (clusterId: number | string) => PermissionType | null;
  // 重新整理權限
  refreshPermissions: () => Promise<void>;
  // 設定當前叢集
  setCurrentClusterId: (clusterId: number | string | null) => void;
  // 命名空間權限相關
  getAllowedNamespaces: (clusterId?: number | string) => string[];
  hasAllNamespaceAccess: (clusterId?: number | string) => boolean;
  filterNamespaces: (namespaces: string[], clusterId?: number | string) => string[];
}

export const PermissionContext = createContext<PermissionContextType | null>(null);

