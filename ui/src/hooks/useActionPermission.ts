/**
 * 操作權限檢查 Hook
 * 用於在元件內檢查是否有權限執行某個操作
 */

import { usePermission } from './usePermission';
import { tokenManager } from '../services/authService';
import { isPlatformAdmin } from '../config/menuPermissions';
import type { PermissionType } from '../types';

export const useActionPermission = () => {
  const { currentClusterPermission } = usePermission();
  const currentUser = tokenManager.getUser();

  const canPerform = (action: string): boolean => {
    // 平臺管理員可以執行所有操作
    if (isPlatformAdmin(currentUser?.username)) {
      return true;
    }

    const userPermission = currentClusterPermission?.permission_type as PermissionType | undefined;
    if (!userPermission) return false;

    // 管理員權限可以執行所有操作
    if (userPermission === 'admin') return true;

    // 只讀權限只能檢視
    if (userPermission === 'readonly') {
      return ['view', 'list', 'get'].includes(action);
    }

    // 開發權限
    if (userPermission === 'dev') {
      const allowedPrefixes = ['pod:', 'deployment:', 'statefulset:', 'service:', 'configmap:', 'secret:', 'ingress:', 'job:', 'cronjob:'];
      return allowedPrefixes.some(prefix => action.startsWith(prefix)) || ['view', 'list', 'get'].includes(action);
    }

    // 運維權限（排除節點和儲存的高危操作）
    if (userPermission === 'ops') {
      const restricted = ['node:cordon', 'node:uncordon', 'node:drain', 'pv:delete', 'storageclass:delete'];
      return !restricted.includes(action);
    }

    return false;
  };

  return { canPerform };
};

