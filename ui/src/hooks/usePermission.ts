/**
 * 使用權限的 Hook
 * 從 PermissionContext 中獲取權限相關的方法和狀態
 */

import { useContext } from 'react';
import { PermissionContext, type PermissionContextType } from '../contexts/PermissionContext';

export const usePermission = (): PermissionContextType => {
  const context = useContext(PermissionContext);
  if (!context) {
    throw new Error('usePermission must be used within a PermissionProvider');
  }
  return context;
};

