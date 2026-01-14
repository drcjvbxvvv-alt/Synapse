/**
 * 使用权限的 Hook
 * 从 PermissionContext 中获取权限相关的方法和状态
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

