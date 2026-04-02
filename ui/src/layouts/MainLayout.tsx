import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import kubernetesLogo from '../assets/kubernetes.png';
import {
  Layout,
  Menu,
  Button,
  Badge,
  Dropdown,
  Avatar,
  Space,
  message,
} from 'antd';
import {
  ClusterOutlined,
  DesktopOutlined,
  RocketOutlined,
  AppstoreOutlined,
  BellOutlined,
  UserOutlined,
  BarChartOutlined,
  SettingOutlined,
  FileTextOutlined,
  AlertOutlined,
  EyeOutlined,
  UploadOutlined,
  ApiOutlined,
  HddOutlined,
  KeyOutlined,
  TagsOutlined,
  ContainerOutlined,
  LogoutOutlined,
  HistoryOutlined,
  AuditOutlined,
  BranchesOutlined,
  DeploymentUnitOutlined,
  DollarOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import type { MenuProps as AntMenuProps } from 'antd';
import type { PermissionType } from '../types';
import SearchDropdown from '../components/SearchDropdown';
import ClusterSelector from '../components/ClusterSelector';
import LanguageSwitcher from '../components/LanguageSwitcher';
import { tokenManager } from '../services/authService';
import { usePermission } from '../hooks/usePermission';
import AIChatPanel from '../components/AIChat/AIChatPanel';
import {
  MAIN_MENU_PERMISSIONS,
  CLUSTER_MENU_PERMISSIONS,
  hasPermission,
  isPlatformAdmin,
} from '../config/menuPermissions';

const { Content } = Layout;

type MenuItem = Required<AntMenuProps>['items'][number];

const MainLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation();

  const isClusterDetail = !!location.pathname.match(/\/clusters\/[^/]+\//);

  const getDefaultOpenKeys = useCallback(() => {
    if (isClusterDetail) {
      return ['kubernetes-resources', 'cluster', 'cloud-native-observability', 'cloud-native-cost'];
    }
    if (location.pathname.startsWith('/access')) return ['access-control'];
    if (location.pathname.startsWith('/audit')) return ['audit-management'];
    return [];
  }, [isClusterDetail, location.pathname]);

  const [openKeys, setOpenKeys] = useState<string[]>(getDefaultOpenKeys());

  useEffect(() => {
    setOpenKeys(getDefaultOpenKeys());
  }, [getDefaultOpenKeys]);

  const handleOpenChange = (keys: string[]) => setOpenKeys(keys);

  const getSelectedKeys = () => {
    const path = location.pathname;
    if (path.match(/\/clusters\/[^/]+\/overview/)) return ['cluster-overview'];
    if (path.match(/\/clusters\/[^/]+\/workloads/)) return ['k8s-workloads'];
    if (path.match(/\/clusters\/[^/]+\/pods/)) return ['k8s-pods'];
    if (path.match(/\/clusters\/[^/]+\/network/)) return ['k8s-network'];
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
    if (path.match(/\/clusters\/[^/]+\/cost/)) return ['cluster-cost'];
    if (path.match(/\/clusters\/[^/]+\/security/)) return ['cluster-security'];
    if (path.match(/\/clusters\/[^/]+\/monitoring/)) return ['observability-monitoring'];
    if (path.match(/\/clusters\/[^/]+\/logs/)) return ['observability-logs'];
    if (path.match(/\/clusters\/[^/]+\/alerts/)) return ['observability-alerts'];
    if (path.match(/\/clusters\/[^/]+\/cost-insights/)) return ['cost-insights'];
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
    if (path.startsWith('/settings')) return ['system-settings'];
    return ['overview'];
  };

  // ─── Main menu ───────────────────────────────────────────────────
  const mainMenuItems: MenuItem[] = [
    {
      key: 'overview',
      icon: <AppstoreOutlined />,
      label: t('menu.overview'),
      onClick: () => navigate('/overview'),
    },
    {
      key: 'cluster-management',
      icon: <ClusterOutlined />,
      label: t('menu.clusters'),
      onClick: () => navigate('/clusters'),
    },
    {
      key: 'access-control',
      icon: <KeyOutlined />,
      label: '访问控制',
      children: [
        { key: 'access-users', icon: <UserOutlined />, label: '用户管理', onClick: () => navigate('/access/users') },
        { key: 'access-user-groups', icon: <ClusterOutlined />, label: '用户组管理', onClick: () => navigate('/access/user-groups') },
        { key: 'access-permissions', icon: <KeyOutlined />, label: '权限分配', onClick: () => navigate('/access/permissions') },
      ],
    },
    {
      key: 'audit-management',
      icon: <AuditOutlined />,
      label: t('menu.audit'),
      children: [
        { key: 'audit-operations', icon: <AuditOutlined />, label: t('menu.operationLogs'), onClick: () => navigate('/audit/operations') },
        { key: 'audit-commands', icon: <HistoryOutlined />, label: t('menu.commandHistory'), onClick: () => navigate('/audit/commands') },
      ],
    },
    {
      key: 'alert-center',
      icon: <AlertOutlined />,
      label: t('menu.alerts'),
      onClick: () => navigate('/alerts'),
    },
    {
      key: 'system-settings',
      icon: <SettingOutlined />,
      label: t('menu.settings'),
      onClick: () => navigate('/settings'),
    },
  ];

  // ─── Cluster detail menu ─────────────────────────────────────────
  const nav = (path: string) => {
    const m = location.pathname.match(/\/clusters\/([^/]+)/);
    if (m) navigate(`/clusters/${m[1]}/${path}`);
  };

  const clusterDetailMenuItems: MenuItem[] = [
    {
      key: 'cluster-overview',
      label: t('menu.overview'),
      onClick: () => nav('overview'),
    },
    {
      key: 'kubernetes-resources',
      label: t('menu.kubernetesResources'),
      children: [
        { key: 'k8s-workloads', icon: <RocketOutlined />, label: t('menu.workloads'), onClick: () => nav('workloads') },
        { key: 'k8s-pods', icon: <ContainerOutlined />, label: t('menu.pods'), onClick: () => nav('pods') },
        { key: 'k8s-network', icon: <ApiOutlined />, label: t('menu.serviceAndRoutes'), onClick: () => nav('network?tab=service') },
        { key: 'k8s-storage', icon: <HddOutlined />, label: t('menu.storage'), onClick: () => nav('storage') },
        { key: 'k8s-configs', icon: <KeyOutlined />, label: t('menu.configsAndSecrets'), onClick: () => nav('configs') },
        { key: 'k8s-namespaces', icon: <TagsOutlined />, label: t('menu.namespaces'), onClick: () => nav('namespaces') },
      ],
    },
    {
      key: 'cluster',
      label: t('menu.clusterSection'),
      children: [
        { key: 'cluster-nodes', icon: <DesktopOutlined />, label: t('menu.nodeManagement'), onClick: () => nav('nodes') },
        { key: 'cluster-config', icon: <SettingOutlined />, label: t('menu.configCenter'), onClick: () => nav('config-center') },
        { key: 'cluster-upgrade', icon: <UploadOutlined />, label: t('menu.clusterUpgrade'), onClick: () => nav('upgrade') },
        { key: 'cluster-plugins', icon: <BranchesOutlined />, label: t('menu.gitopsApps'), onClick: () => nav('plugins') },
        { key: 'cluster-helm', icon: <DeploymentUnitOutlined />, label: t('menu.helmReleases'), onClick: () => nav('helm') },
        { key: 'cluster-crds', icon: <ApiOutlined />, label: t('menu.crdManagement', 'CRD 管理'), onClick: () => nav('crds') },
      ],
    },
    {
      key: 'cloud-native-observability',
      label: t('menu.observability'),
      children: [
        { key: 'observability-monitoring', icon: <BarChartOutlined />, label: t('menu.monitoring'), onClick: () => nav('monitoring') },
        { key: 'observability-logs', icon: <FileTextOutlined />, label: t('menu.logs'), onClick: () => nav('logs') },
        { key: 'observability-alerts', icon: <AlertOutlined />, label: t('menu.alerts'), onClick: () => nav('alerts') },
        { key: 'cluster-event-alerts', icon: <AlertOutlined />, label: t('menu.eventAlerts', 'Event 告警'), onClick: () => nav('event-alerts') },
        { key: 'cluster-cost', icon: <DollarOutlined />, label: t('menu.costAnalysis', '成本分析'), onClick: () => nav('cost') },
        { key: 'cluster-security', icon: <SafetyOutlined />, label: t('menu.securityScan', '安全掃描'), onClick: () => nav('security') },
      ],
    },
    {
      key: 'cloud-native-cost',
      label: t('menu.costGovernance'),
      children: [
        { key: 'cost-insights', icon: <EyeOutlined />, label: t('menu.costInsights'), onClick: () => nav('cost-insights') },
      ],
    },
  ];

  // ─── Permission filtering ─────────────────────────────────────────
  const currentUser = tokenManager.getUser();
  const { currentClusterPermission, clusterPermissions } = usePermission();
  const allPerms = useMemo(() => Array.from(clusterPermissions.values()), [clusterPermissions]);
  const isUserPlatformAdmin = useMemo(() => isPlatformAdmin(currentUser?.username, allPerms), [currentUser, allPerms]);
  const currentPermissionType = currentClusterPermission?.permission_type as PermissionType | undefined;

  const filterMainMenuItems = useCallback((items: MenuItem[]): MenuItem[] => {
    return items.filter((item) => {
      if (!item || typeof item !== 'object' || !('key' in item)) return true;
      const key = item.key as string;
      const config = MAIN_MENU_PERMISSIONS[key];
      if (!config) return true;
      if (config.platformAdminOnly && !isUserPlatformAdmin) return false;
      if ('children' in item && Array.isArray(item.children)) {
        const filtered = filterMainMenuItems(item.children as MenuItem[]);
        if (filtered.length === 0) return false;
        (item as MenuItem & { children: MenuItem[] }).children = filtered;
      }
      return true;
    });
  }, [isUserPlatformAdmin]);

  const filterClusterMenuItems = useCallback((items: MenuItem[]): MenuItem[] => {
    return items.filter((item) => {
      if (!item || typeof item !== 'object' || !('key' in item)) return true;
      const key = item.key as string;
      const config = CLUSTER_MENU_PERMISSIONS[key];
      if (!config) return true;
      if (config.requiredPermission && !hasPermission(currentPermissionType, config.requiredPermission)) return false;
      if ('children' in item && Array.isArray(item.children)) {
        const filtered = filterClusterMenuItems(item.children as MenuItem[]);
        if (filtered.length === 0) return false;
        (item as MenuItem & { children: MenuItem[] }).children = filtered;
      }
      return true;
    });
  }, [currentPermissionType]);

  const menuItems = useMemo(() =>
    isClusterDetail
      ? filterClusterMenuItems([...clusterDetailMenuItems])
      : filterMainMenuItems([...mainMenuItems]),
    [isClusterDetail, filterClusterMenuItems, filterMainMenuItems]
  );

  // ─── User menu ────────────────────────────────────────────────────
  const getDisplayName = () => {
    const name = currentUser?.display_name || currentUser?.username || 'User';
    return name.replace(/\d+$/, '');
  };

  const handleUserMenuClick: AntMenuProps['onClick'] = ({ key }) => {
    if (key === 'logout') {
      tokenManager.clear();
      message.success(t('auth.logoutSuccess'));
      navigate('/login');
    } else if (key === 'settings') {
      navigate('/settings');
    } else if (key === 'profile') {
      navigate('/profile');
    }
  };

  const userMenuItems: AntMenuProps['items'] = [
    { key: 'profile', icon: <UserOutlined />, label: t('menu.profile') },
    { key: 'settings', icon: <SettingOutlined />, label: t('menu.settings') },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: t('auth.logout'), danger: true },
  ];

  const handleSearch = (value: string) => {
    if (value.trim()) navigate(`/search?q=${encodeURIComponent(value)}`);
  };

  const sidebarTop = isClusterDetail ? 'top-24' : 'top-12';
  const layoutMarginTop = isClusterDetail ? 'mt-24' : 'mt-12';

  return (
    <div className="min-h-screen bg-[var(--color-bg-page)]">

      {/* ── Top Header ── */}
      <header className="fixed top-0 left-0 right-0 z-50 h-12 flex items-center justify-between px-4
                         bg-slate-900 dark:bg-slate-800 border-b border-slate-700/50">
        {/* Logo */}
        <button
          onClick={() => navigate('/')}
          className="flex items-center gap-2 opacity-100 hover:opacity-80 transition-opacity cursor-pointer bg-transparent border-0 p-0"
        >
          <img src={kubernetesLogo} alt="Kubernetes" className="w-7 h-7" />
          <span className="text-white font-semibold text-base tracking-wide">Synapse</span>
        </button>

        {/* Search */}
        <div className="flex-1 max-w-lg mx-6">
          <SearchDropdown onSearch={handleSearch} />
        </div>

        {/* Right actions */}
        <div className="flex items-center gap-1">
          <LanguageSwitcher />
          <Badge count={3} size="small" offset={[-6, 8]}>
            <Button
              type="text"
              icon={<BellOutlined />}
              className="!text-slate-300 hover:!text-white hover:!bg-slate-700"
            />
          </Badge>
          <Dropdown
            menu={{ items: userMenuItems, onClick: handleUserMenuClick }}
            placement="bottomRight"
          >
            <button className="flex items-center gap-2 px-2 py-1 rounded-md hover:bg-slate-700 transition-colors cursor-pointer bg-transparent border-0">
              <Avatar
                icon={<UserOutlined />}
                size={28}
                className="!bg-indigo-500"
              />
              <span className="text-slate-200 text-sm">{getDisplayName()}</span>
            </button>
          </Dropdown>
        </div>
      </header>

      {/* ── Cluster Sub-header ── */}
      {isClusterDetail && (
        <div className="fixed top-12 left-0 right-0 z-40 h-12 flex items-center px-6
                        bg-[var(--color-bg-card)] border-b border-[var(--color-border)]">
          <ClusterSelector />
        </div>
      )}

      {/* ── Body ── */}
      <div className={`flex ${layoutMarginTop}`}>

        {/* ── Sidebar ── */}
        <aside className={`fixed left-0 ${sidebarTop} bottom-0 z-40 w-48
                          bg-[var(--color-bg-sidebar)] border-r border-[var(--color-border)]
                          flex flex-col overflow-hidden`}>
          <div className="flex-1 overflow-y-auto overflow-x-hidden py-1.5 px-0 custom-scrollbar">
            <Menu
              mode="inline"
              selectedKeys={getSelectedKeys()}
              openKeys={openKeys}
              onOpenChange={handleOpenChange}
              items={menuItems}
              className="sidebar-menu !border-0 !bg-transparent"
            />
          </div>
        </aside>

        {/* ── Main Content ── */}
        <Layout className="ml-48 min-h-[calc(100vh-3rem)] !bg-transparent">
          <Content className="m-1 p-4 rounded-lg bg-[var(--color-bg-card)] border border-[var(--color-border)]
                              min-h-[calc(100vh-3.5rem)] shadow-[var(--shadow-card)]">
            <Outlet />
          </Content>
        </Layout>
      </div>

      <AIChatPanel />
    </div>
  );
};

export default MainLayout;
