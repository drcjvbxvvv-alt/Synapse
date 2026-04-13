/**
 * 權限守衛元件
 * 用於保護需要特定權限才能訪問的路由和元件
 */

import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import { tokenManager } from '../services/authService';
import { usePermission, usePermissionLoading } from '../hooks/usePermission';
import {
  isPlatformAdmin,
  hasPermission,
  ROUTE_PERMISSIONS,
  CLUSTER_ROUTE_PERMISSIONS
} from '../config/menuPermissions';
import type { PermissionType } from '../types';
import ErrorPage from './ErrorPage';

interface PermissionGuardProps {
  children: React.ReactNode;
  platformAdminOnly?: boolean;
  requiredPermission?: PermissionType;
  requiredFeature?: string;
  fallback?: React.ReactNode;
}

/**
 * 權限守衛元件
 * 根據使用者權限決定是否渲染子元件
 */
export const PermissionGuard: React.FC<PermissionGuardProps> = ({
  children,
  platformAdminOnly = false,
  requiredPermission,
  requiredFeature,
  fallback,
}) => {
  const { t } = useTranslation('components');
  const currentUser = tokenManager.getUser();
  const { currentClusterPermission, clusterPermissions, hasFeature } = usePermission();
  const loading = usePermissionLoading();

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 200 }}>
        <Spin size="large" />
      </div>
    );
  }

  // 檢查平臺管理員權限
  if (platformAdminOnly) {
    const allPerms = Array.from(clusterPermissions.values());
    if (!isPlatformAdmin(currentUser?.username, allPerms)) {
      if (fallback) return <>{fallback}</>;
      return (
        <ErrorPage
          status={403}
          title={t('permissionGuard.noAccess')}
          subTitle={t('permissionGuard.platformAdminOnly')}
          showBack
          showHome
        />
      );
    }
  }

  // 檢查叢集級別權限
  if (requiredPermission) {
    const userPermission = currentClusterPermission?.permission_type as PermissionType | undefined;
    if (!hasPermission(userPermission, requiredPermission)) {
      if (fallback) return <>{fallback}</>;
      return (
        <ErrorPage
          status={403}
          title={t('permissionGuard.insufficientPermission')}
          subTitle={t('permissionGuard.requirePermission', { permission: getPermissionLabel(requiredPermission, t) })}
          showBack
          showHome
        />
      );
    }
  }

  // 檢查功能策略
  if (requiredFeature && !hasFeature(requiredFeature)) {
    if (fallback) return <>{fallback}</>;
    return (
      <ErrorPage
        status={403}
        title={t('permissionGuard.featureDisabled')}
        subTitle={t('permissionGuard.featureDisabledDesc')}
        showBack
        showHome
      />
    );
  }

  return <>{children}</>;
};

/**
 * 平臺管理員路由守衛
 */
export const PlatformAdminGuard: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const location = useLocation();
  const currentUser = tokenManager.getUser();
  const { clusterPermissions } = usePermission();

  const routeConfig = ROUTE_PERMISSIONS[location.pathname];
  const allPerms = Array.from(clusterPermissions.values());
  
  if (routeConfig?.platformAdminOnly && !isPlatformAdmin(currentUser?.username, allPerms)) {
    return <Navigate to="/overview" replace />;
  }

  return <>{children}</>;
};

/**
 * 叢集權限路由守衛
 */
export const ClusterPermissionGuard: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useTranslation('components');
  const location = useLocation();
  const { currentClusterPermission } = usePermission();
  const loading = usePermissionLoading();

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 200 }}>
        <Spin size="large" />
      </div>
    );
  }

  const pathMatch = location.pathname.match(/\/clusters\/[^/]+(.+)/);
  const subPath = pathMatch ? pathMatch[1] : '';

  for (const [routePattern, requiredPermission] of Object.entries(CLUSTER_ROUTE_PERMISSIONS)) {
    if (subPath.startsWith(routePattern)) {
      const userPermission = currentClusterPermission?.permission_type as PermissionType | undefined;
      if (!hasPermission(userPermission, requiredPermission)) {
        return (
          <ErrorPage
            status={403}
            title={t('permissionGuard.insufficientPermission')}
            subTitle={t('permissionGuard.requirePermission', { permission: getPermissionLabel(requiredPermission, t) })}
            showBack
            showHome
          />
        );
      }
      break;
    }
  }

  return <>{children}</>;
};

const getPermissionLabel = (type: PermissionType, t: (key: string) => string): string => {
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
