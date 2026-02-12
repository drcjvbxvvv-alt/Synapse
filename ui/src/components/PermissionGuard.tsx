/**
 * 权限守卫组件
 * 用于保护需要特定权限才能访问的路由和组件
 */

import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { Result, Button } from 'antd';
import { useTranslation } from 'react-i18next';
import { tokenManager } from '../services/authService';
import { usePermission } from '../hooks/usePermission';
import { 
  isPlatformAdmin, 
  hasPermission, 
  ROUTE_PERMISSIONS, 
  CLUSTER_ROUTE_PERMISSIONS 
} from '../config/menuPermissions';
import type { PermissionType } from '../types';

interface PermissionGuardProps {
  children: React.ReactNode;
  // 平台管理员专属
  platformAdminOnly?: boolean;
  // 集群级别权限要求
  requiredPermission?: PermissionType;
  // 自定义无权限时的行为
  fallback?: React.ReactNode;
}

/**
 * 权限守卫组件
 * 根据用户权限决定是否渲染子组件
 */
export const PermissionGuard: React.FC<PermissionGuardProps> = ({
  children,
  platformAdminOnly = false,
  requiredPermission,
  fallback,
}) => {
  const { t } = useTranslation('components');
  const currentUser = tokenManager.getUser();
  const { currentClusterPermission, loading } = usePermission();

  // 如果权限还在加载中，可以显示加载状态
  if (loading) {
    return null;
  }

  // 检查平台管理员权限
  if (platformAdminOnly) {
    if (!isPlatformAdmin(currentUser?.username)) {
      if (fallback) return <>{fallback}</>;
      return (
        <Result
          status="403"
          title={t('permissionGuard.noAccess')}
          subTitle={t('permissionGuard.platformAdminOnly')}
          extra={
            <Button type="primary" onClick={() => window.history.back()}>
              {t('permissionGuard.goBack')}
            </Button>
          }
        />
      );
    }
  }

  // 检查集群级别权限
  if (requiredPermission) {
    const userPermission = currentClusterPermission?.permission_type as PermissionType | undefined;
    if (!hasPermission(userPermission, requiredPermission)) {
      if (fallback) return <>{fallback}</>;
      return (
        <Result
          status="403"
          title={t('permissionGuard.insufficientPermission')}
          subTitle={t('permissionGuard.requirePermission', { permission: getPermissionLabel(requiredPermission, t) })}
          extra={
            <Button type="primary" onClick={() => window.history.back()}>
              {t('permissionGuard.goBack')}
            </Button>
          }
        />
      );
    }
  }

  return <>{children}</>;
};

/**
 * 平台管理员路由守卫
 * 自动检测当前路由是否需要平台管理员权限
 */
export const PlatformAdminGuard: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const location = useLocation();
  const currentUser = tokenManager.getUser();

  // 检查当前路由是否需要平台管理员权限
  const routeConfig = ROUTE_PERMISSIONS[location.pathname];
  
  if (routeConfig?.platformAdminOnly && !isPlatformAdmin(currentUser?.username)) {
    return <Navigate to="/overview" replace />;
  }

  return <>{children}</>;
};

/**
 * 集群权限路由守卫
 * 自动检测当前集群路由是否需要特定权限
 */
export const ClusterPermissionGuard: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useTranslation('components');
  const location = useLocation();
  const { currentClusterPermission, loading } = usePermission();

  if (loading) {
    return null;
  }

  // 从路径中提取子路由部分
  const pathMatch = location.pathname.match(/\/clusters\/[^/]+(.+)/);
  const subPath = pathMatch ? pathMatch[1] : '';

  // 检查是否需要特定权限
  for (const [routePattern, requiredPermission] of Object.entries(CLUSTER_ROUTE_PERMISSIONS)) {
    if (subPath.startsWith(routePattern)) {
      const userPermission = currentClusterPermission?.permission_type as PermissionType | undefined;
      if (!hasPermission(userPermission, requiredPermission)) {
        return (
          <Result
            status="403"
            title={t('permissionGuard.insufficientPermission')}
            subTitle={t('permissionGuard.requirePermission', { permission: getPermissionLabel(requiredPermission, t) })}
            extra={
              <Button type="primary" onClick={() => window.history.back()}>
                {t('permissionGuard.goBack')}
              </Button>
            }
          />
        );
      }
      break;
    }
  }

  return <>{children}</>;
};

// 获取权限类型的显示名称
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const getPermissionLabel = (type: PermissionType, t: any): string => {
  const labels: Record<PermissionType, string> = {
    admin: t('permissionGuard.adminPermission'),
    ops: t('permissionGuard.opsPermission'),
    dev: t('permissionGuard.devPermission'),
    readonly: t('permissionGuard.readonlyPermission'),
    custom: t('permissionGuard.customPermission'),
  };
  return labels[type] || type;
};

export default PermissionGuard;
