import React, { useState, useEffect, useCallback, useMemo, type ReactNode } from 'react';
import type { MyPermissionsResponse, PermissionType } from '../types';
import permissionService from '../services/permissionService';
import { tokenManager } from '../services/authService';
import { PermissionContext, PermissionLoadingContext, type PermissionContextType } from './PermissionContext';

// 權限Provider元件
export const PermissionProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [clusterPermissions, setClusterPermissions] = useState<Map<number, MyPermissionsResponse>>(new Map());
  const [currentClusterPermission, setCurrentClusterPermission] = useState<MyPermissionsResponse | null>(null);
  const [currentClusterId, setCurrentClusterIdState] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);

  // 載入使用者權限
  const refreshPermissions = useCallback(async () => {
    if (!tokenManager.isLoggedIn()) {
      setClusterPermissions(new Map());
      setCurrentClusterPermission(null);
      return;
    }

    setLoading(true);
    try {
      const response = await permissionService.getMyPermissions();
      const permissions = response || [];
      
      const permMap = new Map<number, MyPermissionsResponse>();
      permissions.forEach((p) => {
        permMap.set(p.cluster_id, p);
      });
      
      setClusterPermissions(permMap);
      
      // 更新當前叢集權限
      if (currentClusterId) {
        setCurrentClusterPermission(permMap.get(currentClusterId) || null);
      }
    } catch (error) {
      console.error('載入權限失敗:', error);
    } finally {
      setLoading(false);
    }
  }, [currentClusterId]);

  // 設定當前叢集
  const setCurrentClusterId = useCallback((clusterId: number | string | null) => {
    if (clusterId === null) {
      setCurrentClusterIdState(null);
      setCurrentClusterPermission(null);
      return;
    }
    
    const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
    setCurrentClusterIdState(id);
    setCurrentClusterPermission(clusterPermissions.get(id) || null);
  }, [clusterPermissions]);

  // 檢查是否有叢集訪問權限
  const hasClusterAccess = useCallback((clusterId: number | string): boolean => {
    const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
    return clusterPermissions.has(id);
  }, [clusterPermissions]);

  // 檢查是否有命名空間訪問權限
  const hasNamespaceAccess = useCallback((clusterId: number | string, namespace: string): boolean => {
    const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
    const permission = clusterPermissions.get(id);
    if (!permission) return false;
    
    const namespaces = permission.namespaces;
    if (namespaces.includes('*')) return true;
    if (namespaces.includes(namespace)) return true;
    
    // 檢查萬用字元匹配
    for (const ns of namespaces) {
      if (ns.endsWith('*') && namespace.startsWith(ns.slice(0, -1))) {
        return true;
      }
    }
    
    return false;
  }, [clusterPermissions]);

  // 檢查是否可以執行操作
  const canPerformAction = useCallback((action: string, clusterId?: number | string): boolean => {
    let permission: MyPermissionsResponse | null = null;
    
    if (clusterId) {
      const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
      permission = clusterPermissions.get(id) || null;
    } else {
      permission = currentClusterPermission;
    }
    
    if (!permission) return false;
    
    const type = permission.permission_type;
    
    switch (type) {
      case 'admin':
        return true;
      case 'ops': {
        // 運維權限：排除節點操作和儲存管理
        const restrictedOps = ['node:cordon', 'node:uncordon', 'node:drain', 'pv:create', 'pv:delete'];
        return !restrictedOps.includes(action);
      }
      case 'dev': {
        // 開發權限：只能操作工作負載相關
        const allowedDev = ['pod:', 'deployment:', 'statefulset:', 'service:', 'configmap:', 'secret:'];
        return allowedDev.some(prefix => action.startsWith(prefix)) || action === 'view';
      }
      case 'readonly':
        return action === 'view' || action === 'list' || action === 'get';
      case 'custom':
        return true; // 自定義權限由 K8s RBAC 控制
      default:
        return false;
    }
  }, [clusterPermissions, currentClusterPermission]);

  // 檢查是否是管理員
  const isAdmin = useCallback((clusterId?: number | string): boolean => {
    let permission: MyPermissionsResponse | null = null;
    
    if (clusterId) {
      const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
      permission = clusterPermissions.get(id) || null;
    } else {
      permission = currentClusterPermission;
    }
    
    return permission?.permission_type === 'admin';
  }, [clusterPermissions, currentClusterPermission]);

  // 檢查是否是隻讀
  const isReadonly = useCallback((clusterId?: number | string): boolean => {
    let permission: MyPermissionsResponse | null = null;
    
    if (clusterId) {
      const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
      permission = clusterPermissions.get(id) || null;
    } else {
      permission = currentClusterPermission;
    }
    
    return permission?.permission_type === 'readonly';
  }, [clusterPermissions, currentClusterPermission]);

  // 檢查是否有寫權限（非只讀權限）
  const canWrite = useCallback((clusterId?: number | string): boolean => {
    let permission: MyPermissionsResponse | null = null;
    
    if (clusterId) {
      const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
      permission = clusterPermissions.get(id) || null;
    } else {
      permission = currentClusterPermission;
    }
    
    if (!permission) return false;
    
    // 只讀權限無法執行寫操作
    return permission.permission_type !== 'readonly';
  }, [clusterPermissions, currentClusterPermission]);

  // 獲取權限型別
  const getPermissionType = useCallback((clusterId: number | string): PermissionType | null => {
    const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
    const permission = clusterPermissions.get(id);
    return permission?.permission_type || null;
  }, [clusterPermissions]);

  // 獲取允許訪問的命名空間列表
  const getAllowedNamespaces = useCallback((clusterId?: number | string): string[] => {
    let permission: MyPermissionsResponse | null = null;
    
    if (clusterId) {
      const id = typeof clusterId === 'string' ? parseInt(clusterId, 10) : clusterId;
      permission = clusterPermissions.get(id) || null;
    } else {
      permission = currentClusterPermission;
    }
    
    if (!permission) return [];
    return permission.namespaces || ['*'];
  }, [clusterPermissions, currentClusterPermission]);

  // 檢查是否有全部命名空間訪問權限
  const hasAllNamespaceAccess = useCallback((clusterId?: number | string): boolean => {
    const namespaces = getAllowedNamespaces(clusterId);
    return namespaces.includes('*');
  }, [getAllowedNamespaces]);

  // 過濾命名空間列表，只返回使用者有權限訪問的
  const filterNamespaces = useCallback((namespaces: string[], clusterId?: number | string): string[] => {
    const allowedNamespaces = getAllowedNamespaces(clusterId);
    
    // 如果有全部權限，返回全部
    if (allowedNamespaces.includes('*')) {
      return namespaces;
    }
    
    // 過濾只保留有權限的命名空間
    return namespaces.filter(ns => {
      // 精確匹配
      if (allowedNamespaces.includes(ns)) {
        return true;
      }
      // 萬用字元匹配
      for (const allowed of allowedNamespaces) {
        if (allowed.endsWith('*')) {
          const prefix = allowed.slice(0, -1);
          if (ns.startsWith(prefix)) {
            return true;
          }
        }
      }
      return false;
    });
  }, [getAllowedNamespaces]);

  // 初始載入
  useEffect(() => {
    refreshPermissions();
  }, [refreshPermissions]);

  // loading is excluded from the main context value so that loading state
  // changes (true→false) don't trigger re-renders in the many components
  // that only need permission-check methods.
  const value = useMemo<PermissionContextType>(() => ({
    clusterPermissions,
    currentClusterPermission,
    hasClusterAccess,
    hasNamespaceAccess,
    canPerformAction,
    isAdmin,
    isReadonly,
    canWrite,
    getPermissionType,
    refreshPermissions,
    setCurrentClusterId,
    getAllowedNamespaces,
    hasAllNamespaceAccess,
    filterNamespaces,
  }), [
    clusterPermissions,
    currentClusterPermission,
    hasClusterAccess,
    hasNamespaceAccess,
    canPerformAction,
    isAdmin,
    isReadonly,
    canWrite,
    getPermissionType,
    refreshPermissions,
    setCurrentClusterId,
    getAllowedNamespaces,
    hasAllNamespaceAccess,
    filterNamespaces,
  ]);

  return (
    <PermissionLoadingContext.Provider value={loading}>
      <PermissionContext.Provider value={value}>
        {children}
      </PermissionContext.Provider>
    </PermissionLoadingContext.Provider>
  );
};

