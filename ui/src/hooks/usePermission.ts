/**
 * 使用權限的 Hook
 * 從 PermissionContext 中獲取權限相關的方法和狀態
 */

import { useContext } from 'react';
import { PermissionContext, PermissionLoadingContext, type PermissionContextType } from '../contexts/PermissionContext';

export const usePermission = (): PermissionContextType => {
  const context = useContext(PermissionContext);
  if (!context) {
    throw new Error('usePermission must be used within a PermissionProvider');
  }
  return context;
};

/**
 * 僅訂閱 loading 狀態的 Hook。
 * 與 usePermission() 分開，避免 loading 切換時觸發不關心載入狀態的元件重渲染。
 */
export const usePermissionLoading = (): boolean => {
  return useContext(PermissionLoadingContext);
};

