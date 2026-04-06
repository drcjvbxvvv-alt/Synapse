/**
 * 選單權限配置
 * 定義每個選單項需要的權限型別
 */

import type { PermissionType } from '../types';

// 權限型別優先順序（數值越大權限越高）
export const PERMISSION_PRIORITY: Record<PermissionType, number> = {
  readonly: 1,
  dev: 2,
  ops: 3,
  admin: 4,
  custom: 2, // 自定義權限預設與開發權限相同
};

// 檢查是否有足夠的權限
export const hasPermission = (
  userPermission: PermissionType | null | undefined,
  requiredPermission: PermissionType
): boolean => {
  if (!userPermission) return false;
  return PERMISSION_PRIORITY[userPermission] >= PERMISSION_PRIORITY[requiredPermission];
};

// 檢查是否是平臺管理員
// 邏輯：username 為 admin，或者在任意叢集擁有 admin 權限
export const isPlatformAdmin = (username: string | undefined, permissions?: { permission_type: string }[]): boolean => {
  if (username === 'admin') return true;
  if (permissions && permissions.length > 0) {
    return permissions.some(p => p.permission_type === 'admin');
  }
  return false;
};

// 外層側邊欄選單權限配置
export const MAIN_MENU_PERMISSIONS: Record<string, {
  requiredPermission?: PermissionType;  // 叢集級權限要求
  platformAdminOnly?: boolean;          // 是否僅平臺管理員可見
}> = {
  // 所有人可見
  'overview': {},
  'cluster-management': {},
  
  // 訪問控制選單組 - 僅平臺管理員可見
  'access-control': { platformAdminOnly: true },
  'access-users': { platformAdminOnly: true },
  'access-user-groups': { platformAdminOnly: true },
  'access-permissions': { platformAdminOnly: true },

  // 僅平臺管理員可見
  'permission-management': { platformAdminOnly: true },
  'audit-management': { platformAdminOnly: true },
  'audit-operations': { platformAdminOnly: true },
  'audit-commands': { platformAdminOnly: true },
  'system-settings': { platformAdminOnly: true },
};

// 叢集內層側邊欄選單權限配置
export const CLUSTER_MENU_PERMISSIONS: Record<string, {
  requiredPermission?: PermissionType;
  description?: string;
}> = {
  // === 概覽 - 所有人可見 ===
  'cluster-overview': {},
  
  // === Kubernetes資源 - 大部分所有人可見 ===
  'kubernetes-resources': {},
  'k8s-workloads': {},
  'k8s-pods': {},
  'k8s-network': {},
  'k8s-storage': {},           // 儲存管理，所有人可檢視
  'k8s-configs': {},
  'k8s-namespaces': {},        // 命名空間，所有人可檢視
  
  // === 叢集管理 - 需要較高權限 ===
  'cluster': { requiredPermission: 'ops' },           // 叢集分組需要運維權限
  'cluster-nodes': { requiredPermission: 'ops' },     // 節點管理需要運維權限
  'cluster-config': { requiredPermission: 'ops' },    // 配置中心需要運維權限
  'cluster-upgrade': { requiredPermission: 'admin' }, // 叢集升級僅管理員
  'cluster-plugins': { requiredPermission: 'ops' },   // 外掛中心需要運維權限
  
  // === 雲原生觀測 - 大部分所有人可見 ===
  'cloud-native-observability': {},
  'observability-monitoring': {},
  'observability-logs': {},
  'observability-alerts': { requiredPermission: 'dev' }, // 告警中心需要開發及以上權限
  
  // === 雲原生成本治理 - 需要運維權限 ===
  'cloud-native-cost': { requiredPermission: 'ops' },
  'cost-insights': { requiredPermission: 'ops' },
};

// 操作按鈕權限配置
export const ACTION_PERMISSIONS: Record<string, PermissionType> = {
  // Pod 操作
  'pod:delete': 'dev',
  'pod:exec': 'dev',
  'pod:logs': 'readonly',
  
  // 工作負載操作
  'deployment:create': 'dev',
  'deployment:update': 'dev',
  'deployment:delete': 'dev',
  'deployment:scale': 'dev',
  'deployment:restart': 'dev',
  
  'statefulset:create': 'dev',
  'statefulset:update': 'dev',
  'statefulset:delete': 'dev',
  'statefulset:scale': 'dev',
  
  'daemonset:create': 'ops',
  'daemonset:update': 'ops',
  'daemonset:delete': 'ops',
  
  // 服務和路由
  'service:create': 'dev',
  'service:update': 'dev',
  'service:delete': 'dev',
  
  'ingress:create': 'dev',
  'ingress:update': 'dev',
  'ingress:delete': 'dev',
  
  // 配置
  'configmap:create': 'dev',
  'configmap:update': 'dev',
  'configmap:delete': 'dev',
  
  'secret:create': 'dev',
  'secret:update': 'dev',
  'secret:delete': 'dev',
  
  // 儲存
  'pv:create': 'admin',
  'pv:delete': 'admin',
  'pvc:create': 'ops',
  'pvc:delete': 'ops',
  'storageclass:create': 'admin',
  'storageclass:delete': 'admin',
  
  // 節點
  'node:cordon': 'admin',
  'node:uncordon': 'admin',
  'node:drain': 'admin',
  
  // 命名空間
  'namespace:create': 'admin',
  'namespace:delete': 'admin',
  
  // 終端
  'terminal:kubectl': 'dev',
  'terminal:pod': 'dev',
  'terminal:ssh': 'ops',
};

// 檢查是否有操作權限
export const canPerformAction = (
  userPermission: PermissionType | null | undefined,
  action: string
): boolean => {
  const requiredPermission = ACTION_PERMISSIONS[action];
  if (!requiredPermission) return true; // 未定義的操作預設允許
  return hasPermission(userPermission, requiredPermission);
};

// 路由權限配置
export const ROUTE_PERMISSIONS: Record<string, {
  requiredPermission?: PermissionType;
  platformAdminOnly?: boolean;
}> = {
  '/access/users': { platformAdminOnly: true },
  '/access/user-groups': { platformAdminOnly: true },
  '/access/permissions': { platformAdminOnly: true },
  '/permissions': { platformAdminOnly: true },
  '/audit': { platformAdminOnly: true },
  '/audit/operations': { platformAdminOnly: true },
  '/audit/commands': { platformAdminOnly: true },
  '/settings': { platformAdminOnly: true },
};

// 叢集內路由權限配置（基於路徑模式）
export const CLUSTER_ROUTE_PERMISSIONS: Record<string, PermissionType> = {
  '/nodes': 'ops',
  '/config-center': 'ops',
  '/upgrade': 'admin',
  '/plugins': 'ops',
  '/cost-insights': 'ops',
};

export default {
  PERMISSION_PRIORITY,
  hasPermission,
  isPlatformAdmin,
  MAIN_MENU_PERMISSIONS,
  CLUSTER_MENU_PERMISSIONS,
  ACTION_PERMISSIONS,
  canPerformAction,
  ROUTE_PERMISSIONS,
  CLUSTER_ROUTE_PERMISSIONS,
};
