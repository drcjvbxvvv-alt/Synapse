import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu } from 'antd';
import {
  AppstoreOutlined,
  ClusterOutlined,
  KeyOutlined,
  UserOutlined,
  AuditOutlined,
  HistoryOutlined,
  BranchesOutlined,
  AlertOutlined,
  SettingOutlined,
  RocketOutlined,
  ContainerOutlined,
  ApiOutlined,
  HddOutlined,
  TagsOutlined,
  DesktopOutlined,
  UploadOutlined,
  DeploymentUnitOutlined,
  BarChartOutlined,
  FileTextOutlined,
  DollarOutlined,
  SafetyOutlined,
  SafetyCertificateOutlined,
  ThunderboltOutlined,
  LineChartOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { MenuProps as AntMenuProps } from 'antd';
import type { PermissionType } from '../types';
import { tokenManager } from '../services/authService';
import { usePermission } from '../hooks/usePermission';
import {
  MAIN_MENU_PERMISSIONS,
  CLUSTER_MENU_PERMISSIONS,
  hasPermission,
  isPlatformAdmin,
} from '../config/menuPermissions';
import styles from './AppSider.module.css';

const { Sider } = Layout;
type MenuItem = Required<AntMenuProps>['items'][number];

interface AppSiderProps {
  isClusterDetail: boolean;
}

const AppSider: React.FC<AppSiderProps> = ({ isClusterDetail }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation();

  // ─── 選單展開 / 選取狀態 ────────────────────────────────────────────
  const getInitialOpenKeys = (): string[] => {
    if (isClusterDetail) {
      return ['kubernetes-resources', 'cluster', 'cloud-native-observability'];
    }
    if (location.pathname.startsWith('/access')) return ['access-control'];
    if (location.pathname.startsWith('/audit')) return ['audit-management'];
    return [];
  };

  const [openKeys, setOpenKeys] = useState<string[]>(getInitialOpenKeys);

  // 只在「進入/離開叢集詳情」這個大情境切換時才重置，
  // 不跟蹤 pathname，保留使用者在同一情境下的折疊狀態。
  useEffect(() => {
    if (isClusterDetail) {
      setOpenKeys(['kubernetes-resources', 'cluster', 'cloud-native-observability']);
    } else {
      const path = location.pathname;
      if (path.startsWith('/access')) setOpenKeys(['access-control']);
      else if (path.startsWith('/audit')) setOpenKeys(['audit-management']);
      else setOpenKeys([]);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isClusterDetail]);

  const getSelectedKeys = useCallback((): string[] => {
    const path = location.pathname;
    if (path.match(/\/clusters\/[^/]+\/overview/)) return ['cluster-overview'];
    if (path.match(/\/clusters\/[^/]+\/autoscaling/)) return ['k8s-autoscaling'];
    if (path.match(/\/clusters\/[^/]+\/workloads/)) return ['k8s-workloads'];
    if (path.match(/\/clusters\/[^/]+\/pods/)) return ['k8s-pods'];
    if (path.match(/\/clusters\/[^/]+\/network/)) return ['k8s-network'];
    if (path.match(/\/clusters\/[^/]+\/services/)) return ['k8s-services'];
    if (path.match(/\/clusters\/[^/]+\/storage/)) return ['k8s-storage'];
    if (path.match(/\/clusters\/[^/]+\/configs/)) return ['k8s-configs'];
    if (path.match(/\/clusters\/[^/]+\/namespaces/)) return ['k8s-namespaces'];
    if (path.match(/\/clusters\/[^/]+\/nodes/)) return ['cluster-nodes'];
    if (path.match(/\/clusters\/[^/]+\/config-center/)) return ['cluster-config'];
    if (path.match(/\/clusters\/[^/]+\/upgrade/)) return ['cluster-upgrade'];
    if (path.match(/\/clusters\/[^/]+\/plugins/)) return ['cluster-plugins'];
    if (path.match(/\/clusters\/[^/]+\/helm/)) return ['cluster-helm'];
    if (path.match(/\/clusters\/[^/]+\/crds/)) return ['cluster-crds'];
    if (path.match(/\/clusters\/[^/]+\/event-alerts/)) return ['cluster-event-alerts'];
    if (path === '/cost-insights') return ['cost-insights'];
    if (path.match(/\/clusters\/[^/]+\/cost-insights/)) return ['cluster-cost'];
    if (path.match(/\/clusters\/[^/]+\/security/)) return ['cluster-security'];
    if (path.match(/\/clusters\/[^/]+\/certificates/)) return ['cluster-certificates'];
    if (path.match(/\/clusters\/[^/]+\/slos/)) return ['cluster-slos'];
    if (path.match(/\/clusters\/[^/]+\/chaos/)) return ['cluster-chaos'];
    if (path.match(/\/clusters\/[^/]+\/compliance/)) return ['cluster-compliance'];
    if (path.match(/\/clusters\/[^/]+\/monitoring/)) return ['observability-monitoring'];
    if (path.match(/\/clusters\/[^/]+\/logs/)) return ['observability-logs'];
    if (path.match(/\/clusters\/[^/]+\/alerts/)) return ['observability-alerts'];
    if (path === '/overview' || path === '/') return ['overview'];
    if (path.startsWith('/clusters') && !path.match(/\/clusters\/[^/]+\//)) return ['cluster-management'];
    if (path === '/access/users') return ['access-users'];
    if (path === '/access/user-groups') return ['access-user-groups'];
    if (path === '/access/permissions') return ['access-permissions'];
    if (path.startsWith('/permissions')) return ['access-permissions'];
    if (path === '/audit/operations') return ['audit-operations'];
    if (path === '/audit/commands') return ['audit-commands'];
    if (path.startsWith('/audit')) return ['audit-operations'];
    if (path === '/alerts') return ['alert-center'];
    if (path.startsWith('/multicluster')) return ['multicluster'];
    if (path.startsWith('/settings')) return ['system-settings'];
    return ['overview'];
  }, [location.pathname]);

  // ─── 叢集 ID 輔助 ───────────────────────────────────────────────────
  const clusterNav = useCallback((sub: string) => {
    const m = location.pathname.match(/\/clusters\/([^/]+)/);
    if (m) navigate(`/clusters/${m[1]}/${sub}`);
  }, [location.pathname, navigate]);

  // ─── 選單項定義 ─────────────────────────────────────────────────────
  const mainMenuItems: MenuItem[] = useMemo(() => [
    { key: 'overview', icon: <AppstoreOutlined />, label: t('common:menu.overview'), onClick: () => navigate('/overview') },
    { key: 'cluster-management', icon: <ClusterOutlined />, label: t('common:menu.clusters'), onClick: () => navigate('/clusters') },
    {
      key: 'access-control', icon: <KeyOutlined />, label: t('common:menu.accessControl', '訪問控制'),
      children: [
        { key: 'access-users', icon: <UserOutlined />, label: t('common:menu.userManagement', '使用者管理'), onClick: () => navigate('/access/users') },
        { key: 'access-user-groups', icon: <ClusterOutlined />, label: t('common:menu.userGroups', '使用者組管理'), onClick: () => navigate('/access/user-groups') },
        { key: 'access-permissions', icon: <KeyOutlined />, label: t('common:menu.permissions', '權限分配'), onClick: () => navigate('/access/permissions') },
      ],
    },
    {
      key: 'audit-management', icon: <AuditOutlined />, label: t('common:menu.audit'),
      children: [
        { key: 'audit-operations', icon: <AuditOutlined />, label: t('common:menu.operationLogs'), onClick: () => navigate('/audit/operations') },
        { key: 'audit-commands', icon: <HistoryOutlined />, label: t('common:menu.commandHistory'), onClick: () => navigate('/audit/commands') },
      ],
    },
    { key: 'cost-insights', icon: <DollarOutlined />, label: t('common:menu.costInsights'), onClick: () => navigate('/cost-insights') },
    { key: 'multicluster', icon: <BranchesOutlined />, label: t('common:menu.multiCluster', '多叢集工作流程'), onClick: () => navigate('/multicluster') },
    { key: 'alert-center', icon: <AlertOutlined />, label: t('common:menu.alerts'), onClick: () => navigate('/alerts') },
    { key: 'system-settings', icon: <SettingOutlined />, label: t('common:menu.settings'), onClick: () => navigate('/settings') },
  ], [t, navigate]);

  const clusterDetailMenuItems: MenuItem[] = useMemo(() => [
    { key: 'cluster-overview', label: t('common:menu.overview'), onClick: () => clusterNav('overview') },
    {
      key: 'kubernetes-resources', label: t('common:menu.kubernetesResources'),
      children: [
        { key: 'k8s-workloads', icon: <RocketOutlined />, label: t('common:menu.workloads'), onClick: () => clusterNav('workloads') },
        { key: 'k8s-autoscaling', icon: <ThunderboltOutlined />, label: t('common:menu.autoscaling', '彈性伸縮'), onClick: () => clusterNav('autoscaling') },
        { key: 'k8s-pods', icon: <ContainerOutlined />, label: t('common:menu.pods'), onClick: () => clusterNav('pods') },
        { key: 'k8s-network', icon: <ApiOutlined />, label: t('common:menu.serviceAndRoutes'), onClick: () => clusterNav('network?tab=service') },
        { key: 'k8s-storage', icon: <HddOutlined />, label: t('common:menu.storage'), onClick: () => clusterNav('storage') },
        { key: 'k8s-configs', icon: <KeyOutlined />, label: t('common:menu.configsAndSecrets'), onClick: () => clusterNav('configs') },
        { key: 'k8s-namespaces', icon: <TagsOutlined />, label: t('common:menu.namespaces'), onClick: () => clusterNav('namespaces') },
      ],
    },
    {
      key: 'cluster', label: t('common:menu.clusterSection'),
      children: [
        { key: 'cluster-nodes', icon: <DesktopOutlined />, label: t('common:menu.nodeManagement'), onClick: () => clusterNav('nodes') },
        { key: 'cluster-config', icon: <SettingOutlined />, label: t('common:menu.configCenter'), onClick: () => clusterNav('config-center') },
        { key: 'cluster-upgrade', icon: <UploadOutlined />, label: t('common:menu.clusterUpgrade'), onClick: () => clusterNav('upgrade') },
        { key: 'cluster-plugins', icon: <BranchesOutlined />, label: t('common:menu.gitopsApps'), onClick: () => clusterNav('plugins') },
        { key: 'cluster-helm', icon: <DeploymentUnitOutlined />, label: t('common:menu.helmReleases'), onClick: () => clusterNav('helm') },
        { key: 'cluster-crds', icon: <ApiOutlined />, label: t('common:menu.crdManagement', 'CRD 管理'), onClick: () => clusterNav('crds') },
      ],
    },
    {
      key: 'cloud-native-observability', label: t('common:menu.observability'),
      children: [
        { key: 'observability-monitoring', icon: <BarChartOutlined />, label: t('common:menu.monitoring'), onClick: () => clusterNav('monitoring') },
        { key: 'observability-logs', icon: <FileTextOutlined />, label: t('common:menu.logs'), onClick: () => clusterNav('logs') },
        { key: 'observability-alerts', icon: <AlertOutlined />, label: t('common:menu.alerts'), onClick: () => clusterNav('alerts') },
        { key: 'cluster-event-alerts', icon: <AlertOutlined />, label: t('common:menu.eventAlerts', 'Event 告警'), onClick: () => clusterNav('event-alerts') },
        { key: 'cluster-cost', icon: <DollarOutlined />, label: t('common:menu.costAnalysis', '成本分析'), onClick: () => clusterNav('cost-insights') },
        { key: 'cluster-security', icon: <SafetyOutlined />, label: t('common:menu.securityScan', '安全掃描'), onClick: () => clusterNav('security') },
        { key: 'cluster-certificates', icon: <SafetyCertificateOutlined />, label: t('common:menu.certificates', '憑證管理'), onClick: () => clusterNav('certificates') },
        { key: 'cluster-slos', icon: <LineChartOutlined />, label: t('common:menu.slos', 'SLO 管理'), onClick: () => clusterNav('slos') },
        { key: 'cluster-chaos', icon: <ThunderboltOutlined />, label: t('common:menu.chaos', '混沌工程'), onClick: () => clusterNav('chaos') },
        { key: 'cluster-compliance', icon: <SafetyCertificateOutlined />, label: t('common:menu.compliance', '合規管理'), onClick: () => clusterNav('compliance') },
      ],
    },
  ], [t, clusterNav]);

  // ─── 權限過濾 ────────────────────────────────────────────────────────
  const currentUser = tokenManager.getUser();
  const { currentClusterPermission, clusterPermissions } = usePermission();
  const allPerms = useMemo(() => Array.from(clusterPermissions.values()), [clusterPermissions]);
  const isUserPlatformAdmin = useMemo(
    () => isPlatformAdmin(currentUser?.username, allPerms),
    [currentUser, allPerms]
  );
  const currentPermissionType = currentClusterPermission?.permission_type as PermissionType | undefined;

  const filterMainMenuItems = useCallback((items: MenuItem[]): MenuItem[] =>
    items.filter(item => {
      if (!item || typeof item !== 'object' || !('key' in item)) return true;
      const config = MAIN_MENU_PERMISSIONS[item.key as string];
      if (!config) return true;
      if (config.platformAdminOnly && !isUserPlatformAdmin) return false;
      if ('children' in item && Array.isArray(item.children)) {
        const filtered = filterMainMenuItems(item.children as MenuItem[]);
        if (filtered.length === 0) return false;
        (item as MenuItem & { children: MenuItem[] }).children = filtered;
      }
      return true;
    }),
  [isUserPlatformAdmin]);

  const filterClusterMenuItems = useCallback((items: MenuItem[]): MenuItem[] =>
    items.filter(item => {
      if (!item || typeof item !== 'object' || !('key' in item)) return true;
      const config = CLUSTER_MENU_PERMISSIONS[item.key as string];
      if (!config) return true;
      if (config.requiredPermission && !hasPermission(currentPermissionType, config.requiredPermission)) return false;
      if ('children' in item && Array.isArray(item.children)) {
        const filtered = filterClusterMenuItems(item.children as MenuItem[]);
        if (filtered.length === 0) return false;
        (item as MenuItem & { children: MenuItem[] }).children = filtered;
      }
      return true;
    }),
  [currentPermissionType]);

  const menuItems = useMemo(() =>
    isClusterDetail
      ? filterClusterMenuItems([...clusterDetailMenuItems])
      : filterMainMenuItems([...mainMenuItems]),
  [isClusterDetail, filterClusterMenuItems, filterMainMenuItems, clusterDetailMenuItems, mainMenuItems]);

  // ─── 渲染 ────────────────────────────────────────────────────────────
  const siderTop = isClusterDetail ? 112 : 52;

  return (
    <Sider
      width={192}
      style={{
        position: 'fixed',
        left: 0,
        top: siderTop,
        bottom: 0,
        zIndex: 999,
        background: '#f8fafc',
        boxShadow: '2px 0 12px 0 rgba(0, 0, 0, 0.08)',
        borderRight: '1px solid #e0e0e0',
        overflow: 'hidden',
      }}
    >
      <div className={styles.siderScroll}>
        <Menu
          mode="inline"
          selectedKeys={getSelectedKeys()}
          openKeys={openKeys}
          onOpenChange={setOpenKeys}
          items={menuItems}
          className={styles.menu}
          style={{
            height: 'auto',
            minHeight: '100%',
            borderRight: 0,
            background: 'transparent',
            padding: '6px 8px',
          }}
        />
      </div>
    </Sider>
  );
};

export default React.memo(AppSider);
