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
} from '@ant-design/icons';
import type { MenuProps as AntMenuProps } from 'antd';
import type { PermissionType } from '../types';
import SearchDropdown from '../components/SearchDropdown';
import ClusterSelector from '../components/ClusterSelector';
import LanguageSwitcher from '../components/LanguageSwitcher';
import { tokenManager } from '../services/authService';
import { usePermission } from '../hooks/usePermission';
import { 
  MAIN_MENU_PERMISSIONS, 
  CLUSTER_MENU_PERMISSIONS, 
  hasPermission, 
  isPlatformAdmin 
} from '../config/menuPermissions';


const { Header, Sider, Content } = Layout;

// 自定义滚动条和菜单样式
const scrollbarStyles = `
  .custom-scrollbar::-webkit-scrollbar {
    width: 6px;
  }
  
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent;
  }
  
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background-color: #d9d9d9;
    border-radius: 3px;
    transition: background-color 0.2s;
  }
  
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background-color: #bfbfbf;
  }
  
  .custom-scrollbar {
    scrollbar-width: thin;
    scrollbar-color: #d9d9d9 transparent;
  }
  
  .compact-menu .ant-menu-item {
    height: 32px !important;
    line-height: 1.2 !important;
    margin: 2px 0 !important;
    padding: 6px 12px !important;
    font-size: 13px !important;
  }
  
  .compact-menu .ant-menu-submenu-title {
    height: 32px !important;
    line-height: 1.2 !important;
    margin: 2px 0 !important;
    padding: 6px 12px !important;
    font-size: 13px !important;
  }
  
  .compact-menu .ant-menu-item-group-title {
    font-size: 12px !important;
    line-height: 1.2 !important;
    padding: 4px 12px !important;
  }
  
  .compact-menu .ant-menu-submenu .ant-menu-item {
    height: 28px !important;
    line-height: 1.2 !important;
    margin: 1px 0 !important;
    padding: 4px 12px 4px 32px !important;
    font-size: 12px !important;
  }
  
  .compact-menu .ant-menu-submenu-arrow {
    font-size: 10px !important;
  }
  
  .compact-menu .ant-menu-submenu-open > .ant-menu-submenu-title {
    background-color: transparent !important;
    color: rgba(0, 0, 0, 0.88) !important;
  }
  
  .compact-menu .ant-menu-submenu-open > .ant-menu-submenu-title:hover {
    background-color: #f5f5f5 !important;
  }
  
  .compact-menu .ant-menu-submenu-open .ant-menu-submenu-arrow {
    display: none !important;
  }
`;

// 注入样式
if (typeof document !== 'undefined') {
  const styleElement = document.createElement('style');
  styleElement.textContent = scrollbarStyles;
  if (!document.head.querySelector('style[data-scrollbar]')) {
    styleElement.setAttribute('data-scrollbar', 'true');
    document.head.appendChild(styleElement);
  }
}

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
    if (location.pathname.startsWith('/audit')) {
      return ['audit-management'];
    }
    return [];
  }, [isClusterDetail, location.pathname]);

  const [openKeys, setOpenKeys] = useState<string[]>(getDefaultOpenKeys());

  useEffect(() => {
    setOpenKeys(getDefaultOpenKeys());
  }, [getDefaultOpenKeys]);

  // 处理菜单展开/折叠
  const handleOpenChange = (keys: string[]) => {
    setOpenKeys(keys);
  };

  const getSelectedKeys = () => {
    const path = location.pathname;
    
    // 集群详情页面的路由匹配
    if (path.match(/\/clusters\/[^/]+\/overview/)) return ['cluster-overview'];
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
    if (path.match(/\/clusters\/[^/]+\/monitoring/)) return ['observability-monitoring'];
    if (path.match(/\/clusters\/[^/]+\/logs/)) return ['observability-logs'];
    if (path.match(/\/clusters\/[^/]+\/alerts/)) return ['observability-alerts'];
    if (path.match(/\/clusters\/[^/]+\/cost-insights/)) return ['cost-insights'];
    
    // 主页面的路由匹配
    if (path === '/overview' || path === '/') return ['overview'];
    if (path.startsWith('/clusters') && !path.match(/\/clusters\/[^/]+\//)) return ['cluster-management'];
    if (path.startsWith('/permissions')) return ['permission-management'];
    if (path === '/audit/operations') return ['audit-operations'];
    if (path === '/audit/commands') return ['audit-commands'];
    if (path.startsWith('/audit')) return ['audit-operations'];
    if (path === '/alerts') return ['alert-center'];
    if (path.startsWith('/settings')) return ['system-settings'];
    
    return ['overview'];
  };

  // 主页面侧边栏菜单
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
      key: 'permission-management',
      icon: <KeyOutlined />,
      label: t('menu.permissions'),
      onClick: () => navigate('/permissions'),
    },
    {
      key: 'audit-management',
      icon: <AuditOutlined />,
      label: t('menu.audit'),
      children: [
        {
          key: 'audit-operations',
          icon: <AuditOutlined />,
          label: t('menu.operationLogs'),
          onClick: () => navigate('/audit/operations'),
        },
        {
          key: 'audit-commands',
          icon: <HistoryOutlined />,
          label: t('menu.commandHistory'),
          onClick: () => navigate('/audit/commands'),
        },
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

  // 集群详情页侧边栏菜单
  const clusterDetailMenuItems: MenuItem[] = [
    {
      key: 'cluster-overview',
      label: t('menu.overview'),
      onClick: () => {
        const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
        if (clusterMatch) {
          navigate(`/clusters/${clusterMatch[1]}/overview`);
        }
      },
    },
    {
      key: 'kubernetes-resources',
      label: t('menu.kubernetesResources'),
      children: [
        {
          key: 'k8s-workloads',
          icon: <RocketOutlined />,
          label: t('menu.workloads'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/workloads`);
            }
          },
        },
        {
          key: 'k8s-pods',
          icon: <ContainerOutlined />,
          label: t('menu.pods'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/pods`);
            }
          },
        },
        {
          key: 'k8s-network',
          icon: <ApiOutlined />,
          label: t('menu.serviceAndRoutes'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/network?tab=service`);
            }
          },
        },
        {
          key: 'k8s-storage',
          icon: <HddOutlined />,
          label: t('menu.storage'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/storage`);
            }
          },
        },
        {
          key: 'k8s-configs',
          icon: <KeyOutlined />,
          label: t('menu.configsAndSecrets'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/configs`);
            }
          },
        },
        {
          key: 'k8s-namespaces',
          icon: <TagsOutlined />,
          label: t('menu.namespaces'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/namespaces`);
            }
          },
        },
      ],
    },
    {
      key: 'cluster',
      label: t('menu.clusterSection'),
      children: [
        {
          key: 'cluster-nodes',
          icon: <DesktopOutlined />,
          label: t('menu.nodeManagement'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/nodes`);
            }
          },
        },
        {
          key: 'cluster-config',
          icon: <SettingOutlined />,
          label: t('menu.configCenter'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/config-center`);
            }
          },
        },
        {
          key: 'cluster-upgrade',
          icon: <UploadOutlined />,
          label: t('menu.clusterUpgrade'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/upgrade`);
            }
          },
        },
        {
          key: 'cluster-plugins',
          icon: <BranchesOutlined />,
          label: t('menu.gitopsApps'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/plugins`);
            }
          },
        },
      ],
    },
    {
      key: 'cloud-native-observability',
      label: t('menu.observability'),
      children: [
        {
          key: 'observability-monitoring',
          icon: <BarChartOutlined />,
          label: t('menu.monitoring'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/monitoring`);
            }
          },
        },
        {
          key: 'observability-logs',
          icon: <FileTextOutlined />,
          label: t('menu.logs'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/logs`);
            }
          },
        },
        {
          key: 'observability-alerts',
          icon: <AlertOutlined />,
          label: t('menu.alerts'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/alerts`);
            }
          },
        },
      ],
    },
    {
      key: 'cloud-native-cost',
      label: t('menu.costGovernance'),
      children: [
        {
          key: 'cost-insights',
          icon: <EyeOutlined />,
          label: t('menu.costInsights'),
          onClick: () => {
            const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
            if (clusterMatch) {
              navigate(`/clusters/${clusterMatch[1]}/cost-insights`);
            }
          },
        },
      ],
    },
  ];

  // 获取当前用户信息
  const currentUser = tokenManager.getUser();
  const isUserPlatformAdmin = isPlatformAdmin(currentUser?.username);
  
  // 获取当前集群权限（用于集群内层菜单过滤）
  const { currentClusterPermission } = usePermission();
  const currentPermissionType = currentClusterPermission?.permission_type as PermissionType | undefined;

  // 过滤外层主菜单
  const filterMainMenuItems = useCallback((items: MenuItem[]): MenuItem[] => {
    return items.filter((item) => {
      if (!item || typeof item !== 'object' || !('key' in item)) return true;
      const key = item.key as string;
      const config = MAIN_MENU_PERMISSIONS[key];
      
      // 如果没有配置，默认显示
      if (!config) return true;
      
      // 平台管理员专属菜单
      if (config.platformAdminOnly && !isUserPlatformAdmin) {
        return false;
      }
      
      // 递归过滤子菜单
      if ('children' in item && Array.isArray(item.children)) {
        const filteredChildren = filterMainMenuItems(item.children as MenuItem[]);
        if (filteredChildren.length === 0) return false;
        (item as MenuItem & { children: MenuItem[] }).children = filteredChildren;
      }
      
      return true;
    });
  }, [isUserPlatformAdmin]);

  // 过滤集群内层菜单
  const filterClusterMenuItems = useCallback((items: MenuItem[]): MenuItem[] => {
    return items.filter((item) => {
      if (!item || typeof item !== 'object' || !('key' in item)) return true;
      const key = item.key as string;
      const config = CLUSTER_MENU_PERMISSIONS[key];
      
      // 如果没有配置，默认显示
      if (!config) return true;
      
      // 检查权限要求
      if (config.requiredPermission) {
        if (!hasPermission(currentPermissionType, config.requiredPermission)) {
          return false;
        }
      }
      
      // 递归过滤子菜单
      if ('children' in item && Array.isArray(item.children)) {
        const filteredChildren = filterClusterMenuItems(item.children as MenuItem[]);
        if (filteredChildren.length === 0) return false;
        (item as MenuItem & { children: MenuItem[] }).children = filteredChildren;
      }
      
      return true;
    });
  }, [currentPermissionType]);

  const menuItems = useMemo(() => 
    isClusterDetail
      ? filterClusterMenuItems([...clusterDetailMenuItems]) 
      : filterMainMenuItems([...mainMenuItems]),
    [isClusterDetail, filterClusterMenuItems, filterMainMenuItems, clusterDetailMenuItems, mainMenuItems]
  );
  
  // 处理用户名显示，去掉末尾的数字
  const getDisplayName = () => {
    const name = currentUser?.display_name || currentUser?.username || 'User';
    // 去掉末尾的数字，例如 "王植4" -> "王植"
    return name.replace(/\d+$/, '');
  };
  
  // 处理用户菜单点击
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
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: t('menu.profile'),
    },
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: t('menu.settings'),
    },
    {
      type: 'divider',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: t('auth.logout'),
      danger: true,
    },
  ];

  const handleSearch = (value: string) => {
    if (value.trim()) {
      navigate(`/search?q=${encodeURIComponent(value)}`);
    }
  };

  return (
    <Layout style={{ minHeight: '100vh', background: '#fafbfc' }}>
      <Header
        style={{
          position: 'fixed',
          top: 0,
          zIndex: 1000,
          width: '100%',
          height: '48px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 16px',
          background: '#1f2937',
          borderBottom: '1px solid #f0f0f0',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <div 
            style={{ 
              display: 'flex', 
              alignItems: 'center', 
              marginLeft: 16, 
              cursor: 'pointer',
              transition: 'opacity 0.2s'
            }}
            onClick={() => navigate('/')}
            onMouseEnter={(e) => {
              e.currentTarget.style.opacity = '0.8';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.opacity = '1';
            }}
          >
            <img 
              src={kubernetesLogo} 
              alt="Kubernetes" 
              style={{ width: '32px', height: '32px', marginRight: 8 }} 
            />
            <span style={{ fontSize: '18px', fontWeight: 'bold', color: '#ffffff' }}>KubePolaris</span>
          </div>
        </div>

        <div style={{ flex: 1, maxWidth: 600, margin: '0 24px',display: 'flex', alignItems: 'center' }}>
          <SearchDropdown onSearch={handleSearch} />
        </div>

        <Space size="middle">
          <LanguageSwitcher />
          <Badge count={3} size="small" offset={[-8, 10]}>
            <Button type="text" icon={<BellOutlined />} size="large" style={{ color: '#ffffff' }} />
          </Badge>
          <Dropdown menu={{ items: userMenuItems, onClick: handleUserMenuClick }} placement="bottomRight">
            <Space style={{ cursor: 'pointer' }}>
              <Avatar icon={<UserOutlined />} style={{ backgroundColor: '#667eea' }} />
              <span style={{ color: '#ffffff' }}>{getDisplayName()}</span>
            </Space>
          </Dropdown>
        </Space>
      </Header>

      {isClusterDetail && (
        <div style={{
          position: 'fixed',
          top: '48px',
          left: '0',
          right: '0',
          width: '100%',
          height: '48px',
          background: '#f8fafc',
          borderBottom: '1px solid #f0f0f0',
          zIndex: 999,
          display: 'flex',
          alignItems: 'center',
          padding: '0 24px'
        }}>
          <ClusterSelector />
        </div>
      )}

      <Layout style={{ 
        marginTop: isClusterDetail ? 112 : 64,
        }}>
        <Sider
          width={192}
          style={{
            position: 'fixed',
            left: 0,
            top: isClusterDetail ? 112 : 52,
            bottom: 0,
            zIndex: 999,
            background: '#f8fafc',
            boxShadow: '2px 0 12px 0 rgba(0, 0, 0, 0.08)',
            borderRight: '1px solid #e0e0e0',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              height: '100%',
              overflowY: 'auto',
              overflowX: 'hidden',
              scrollbarWidth: 'thin',
              scrollbarColor: '#d9d9d9 transparent',
              padding: '6px 0',
            }}
            className="custom-scrollbar"
          >
            <Menu
              mode="inline"
              selectedKeys={getSelectedKeys()}
              openKeys={openKeys}
              onOpenChange={handleOpenChange}
              items={menuItems}
              className="compact-menu"
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

        <Layout style={{ marginLeft: 192 }}>
          <Content
            style={{
              margin: '0px 4px',
              padding: 16,
              minHeight: 'calc(100vh - 96px)',
              background: '#ffffff',
              borderRadius: '8px',
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.06)',
            }}
          >
            <Outlet />
          </Content>
        </Layout>
      </Layout>
    </Layout>
  );
};

export default MainLayout;