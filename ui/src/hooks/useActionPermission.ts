/**
 * 操作权限检查 Hook
 * 用于在组件内检查是否有权限执行某个操作
 */

import { usePermission } from './usePermission';
import { tokenManager } from '../services/authService';
import { isPlatformAdmin } from '../config/menuPermissions';
import type { PermissionType } from '../types';

export const useActionPermission = () => {
  const { currentClusterPermission } = usePermission();
  const currentUser = tokenManager.getUser();

  const canPerform = (action: string): boolean => {
    // 平台管理员可以执行所有操作
    if (isPlatformAdmin(currentUser?.username)) {
      return true;
    }

    const userPermission = currentClusterPermission?.permission_type as PermissionType | undefined;
    if (!userPermission) return false;

    // 管理员权限可以执行所有操作
    if (userPermission === 'admin') return true;

    // 只读权限只能查看
    if (userPermission === 'readonly') {
      return ['view', 'list', 'get'].includes(action);
    }

    // 开发权限
    if (userPermission === 'dev') {
      const allowedPrefixes = ['pod:', 'deployment:', 'statefulset:', 'service:', 'configmap:', 'secret:', 'ingress:', 'job:', 'cronjob:'];
      return allowedPrefixes.some(prefix => action.startsWith(prefix)) || ['view', 'list', 'get'].includes(action);
    }

    // 运维权限（排除节点和存储的高危操作）
    if (userPermission === 'ops') {
      const restricted = ['node:cordon', 'node:uncordon', 'node:drain', 'pv:delete', 'storageclass:delete'];
      return !restricted.includes(action);
    }

    return false;
  };

  return { canPerform };
};

