import React, { Suspense, useEffect, useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { ConfigProvider, App as AntdApp, Spin, theme as antdTheme } from 'antd';
import { useTranslation } from 'react-i18next';
import zhCN from 'antd/locale/zh_CN';
import zhTW from 'antd/locale/zh_TW';
import enUS from 'antd/locale/en_US';
import MainLayout from './layouts/MainLayout';
import ClusterList from './pages/cluster/ClusterList';
import ClusterDetail from './pages/cluster/ClusterDetail';
import ClusterImport from './pages/cluster/ClusterImport';
import ConfigCenter from './pages/cluster/ConfigCenter';
import ClusterUpgrade from './pages/cluster/ClusterUpgrade';
import NodeList from './pages/node/NodeList';
import NodeDetail from './pages/node/NodeDetail';
import PodList from './pages/pod/PodList';
import PodDetail from './pages/pod/PodDetail';
import PodLogs from './pages/pod/PodLogs';
import PodTerminal from './pages/pod/PodTerminal';
import WorkloadList from './pages/workload/WorkloadList';
import WorkloadDetail from './pages/workload/WorkloadDetail';
import DeploymentCreate from './pages/workload/DeploymentCreate';
import DeploymentDetail from './pages/workload/DeploymentDetail';
import RolloutDetail from './pages/workload/RolloutDetail';
import YAMLEditor from './pages/yaml/YAMLEditor';
import GlobalSearch from './pages/search/GlobalSearch';
import KubectlTerminalPage from './pages/terminal/kubectlTerminal';
import { ConfigSecretManagement, ConfigMapDetail, SecretDetail } from './pages/config';
import ConfigMapEdit from './pages/config/ConfigMapEdit';
import SecretEdit from './pages/config/SecretEdit';
import ConfigMapCreate from './pages/config/ConfigMapCreate';
import SecretCreate from './pages/config/SecretCreate';
import { NamespaceList, NamespaceDetail } from './pages/namespace';
import NetworkList from './pages/network/NetworkList';
import ServiceEdit from './pages/network/ServiceEdit';
import IngressEdit from './pages/network/IngressEdit';
import StorageList from './pages/storage/StorageList';
import Login from './pages/auth/Login';
import SystemSettings from './pages/settings/SystemSettings';
import UserProfile from './pages/profile/UserProfile';
import Overview from './pages/overview/Overview';
import { AlertCenter, GlobalAlertCenter } from './pages/alert';
import { CommandHistory, OperationLogs } from './pages/audit';
import { LogCenter, EventLogs } from './pages/logs';
import ArgoCDConfigPage from './pages/plugins/ArgoCDConfigPage';
import ArgoCDApplicationsPage from './pages/plugins/ArgoCDApplicationsPage';
import { PermissionManagement } from './pages/permission';
import { UserManagement, UserGroupManagement } from './pages/access';
import { MonitoringCenter } from './pages/om';
import HelmList from './pages/helm/HelmList';
import CRDList from './pages/crd/CRDList';
import CRDResources from './pages/crd/CRDResources';
import EventAlertRules from './pages/alert/EventAlertRules';
import CostDashboard from './pages/cost/CostDashboard';
import SecurityDashboard from './pages/security/SecurityDashboard';
import { PermissionProvider } from './contexts/PermissionContext.tsx';
import { tokenManager } from './services/authService';
import { PermissionGuard } from './components/PermissionGuard';
import ErrorBoundary from './components/ErrorBoundary';
import './App.css';

// 认证保护组件
interface RequireAuthProps {
  children: React.ReactNode;
}

const RequireAuth: React.FC<RequireAuthProps> = ({ children }) => {
  const location = useLocation();
  
  if (!tokenManager.isLoggedIn()) {
    // 重定向到登录页，保存当前位置
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
};

// Ant Design locale 映射
const antdLocaleMap: Record<string, typeof zhCN> = {
  'zh-TW': zhTW,
  'zh-CN': zhCN,
  'en-US': enUS,
};

// 内部 App 组件（需要访问 i18n hook）
const AppContent: React.FC = () => {
  const { i18n } = useTranslation();
  const currentLocale = antdLocaleMap[i18n.language] || zhTW;

  const [isDark, setIsDark] = useState(
    () => window.matchMedia('(prefers-color-scheme: dark)').matches
  );

  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (e: MediaQueryListEvent) => setIsDark(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  const antdThemeConfig = {
    algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: '#006eff',
      colorLink: '#006eff',
      borderRadius: 6,
      fontFamily: "-apple-system, BlinkMacSystemFont, 'Inter', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', Arial, sans-serif",
      fontSize: 14,
      colorBgContainer: isDark ? '#1e293b' : '#ffffff',
      colorBgLayout: isDark ? '#0f172a' : '#f1f5f9',
      colorBgElevated: isDark ? '#1e293b' : '#ffffff',
      colorBorder: isDark ? '#334155' : '#e2e8f0',
      colorBorderSecondary: isDark ? '#334155' : '#f1f5f9',
      colorText: isDark ? '#f1f5f9' : '#0f172a',
      colorTextSecondary: isDark ? '#94a3b8' : '#64748b',
      colorTextTertiary: isDark ? '#64748b' : '#94a3b8',
      boxShadow: isDark
        ? '0 1px 3px rgba(0,0,0,0.3), 0 1px 2px rgba(0,0,0,0.2)'
        : '0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04)',
    },
    components: {
      Menu: {
        itemBorderRadius: 6,
        itemHeight: 34,
        itemMarginInline: 8,
        subMenuItemBorderRadius: 6,
        colorItemBg: 'transparent',
        colorItemBgSelected: isDark ? 'rgba(0,110,255,0.15)' : '#e6f0ff',
        colorItemTextSelected: '#006eff',
        colorItemTextHover: '#006eff',
        colorItemBgHover: isDark ? 'rgba(0,110,255,0.10)' : '#e6f0ff',
      },
      Table: {
        headerBg: isDark ? '#273449' : '#f8fafc',
        rowHoverBg: isDark ? '#273449' : '#f8fafc',
        borderColor: isDark ? '#334155' : '#e2e8f0',
      },
      Card: {
        borderRadiusLG: 8,
      },
    },
  };

  return (
    <ConfigProvider locale={currentLocale} theme={antdThemeConfig}>
      <AntdApp>
        <Router>
          <ErrorBoundary>
          <Routes>
            {/* 登录页面 - 不需要认证 */}
            <Route path="/login" element={<Login />} />
            
            {/* 受保护的路由 */}
            <Route path="/" element={
              <RequireAuth>
                <PermissionProvider>
                  <MainLayout />
                </PermissionProvider>
              </RequireAuth>
            }>
              <Route index element={<Navigate to="/overview" replace />} />
              <Route path="overview" element={<Overview />} />
              <Route path="clusters" element={<ClusterList />} />
              <Route path="clusters/:id/overview" element={<ClusterDetail />} />
              {/* 配置中心 - 需要运维权限 */}
              <Route path="clusters/:clusterId/config-center" element={
                <PermissionGuard requiredPermission="ops">
                  <ConfigCenter />
                </PermissionGuard>
              } />
              {/* 集群升级 - 需要管理员权限 */}
              <Route path="clusters/:clusterId/upgrade" element={
                <PermissionGuard requiredPermission="admin">
                  <ClusterUpgrade />
                </PermissionGuard>
              } />
              <Route path="clusters/import" element={<ClusterImport />} />
              <Route path="clusters/:id/terminal" element={<KubectlTerminalPage  />} />
              {/* 节点管理 - 需要运维权限 */}
              <Route path="clusters/:clusterId/nodes" element={
                <PermissionGuard requiredPermission="ops">
                  <NodeList />
                </PermissionGuard>
              } />
              <Route path="clusters/:clusterId/nodes/:nodeName" element={
                <PermissionGuard requiredPermission="ops">
                  <NodeDetail />
                </PermissionGuard>
              } />
              <Route path="nodes" element={<NodeList />} />
              <Route path="nodes/:id" element={<NodeDetail />} />
              <Route path="clusters/:clusterId/pods" element={<PodList />} />
              <Route path="clusters/:clusterId/pods/:namespace/:name" element={<PodDetail />} />
              <Route path="clusters/:clusterId/pods/:namespace/:name/logs" element={<PodLogs />} />
              <Route path="clusters/:clusterId/pods/:namespace/:name/terminal" element={<PodTerminal />} />
              <Route path="clusters/:clusterId/workloads" element={<WorkloadList />} />
              <Route path="clusters/:clusterId/workloads/create" element={<DeploymentCreate />} />
              <Route path="clusters/:clusterId/workloads/deployment/:namespace/:name" element={<DeploymentDetail />} />
              <Route path="clusters/:clusterId/workloads/rollout/:namespace/:name" element={<RolloutDetail />} />
              <Route path="clusters/:clusterId/workloads/:type/:namespace/:name" element={<WorkloadDetail />} />
              <Route path="clusters/:clusterId/workloads/:namespace/:name" element={<WorkloadDetail />} />
              <Route path="clusters/:clusterId/yaml/apply" element={<YAMLEditor />} />
              <Route path="workloads" element={<WorkloadList />} />
              <Route path="workloads/:type/:namespace/:name" element={<WorkloadDetail />} />
              <Route path="search" element={<GlobalSearch />} />
              {/* 全局告警中心路由 */}
              <Route path="alerts" element={<GlobalAlertCenter />} />
              {/* 命名空间路由 */}
              <Route path="clusters/:clusterId/namespaces" element={<NamespaceList />} />
              <Route path="clusters/:clusterId/namespaces/:namespace" element={<NamespaceDetail />} />
              {/* 配置与密钥路由 */}
              <Route path="clusters/:clusterId/configs" element={<ConfigSecretManagement />} />
              <Route path="clusters/:clusterId/configs/configmap/create" element={<ConfigMapCreate />} />
              <Route path="clusters/:clusterId/configs/configmap/:namespace/:name" element={<ConfigMapDetail />} />
              <Route path="clusters/:clusterId/configs/configmap/:namespace/:name/edit" element={<ConfigMapEdit />} />
              <Route path="clusters/:clusterId/configs/secret/create" element={<SecretCreate />} />
              <Route path="clusters/:clusterId/configs/secret/:namespace/:name" element={<SecretDetail />} />
              <Route path="clusters/:clusterId/configs/secret/:namespace/:name/edit" element={<SecretEdit />} />
              {/* 网络管理路由（Service和Ingress） */}
              <Route path="clusters/:clusterId/network" element={<NetworkList />} />
              <Route path="clusters/:clusterId/network/service/:namespace/:name/edit" element={<ServiceEdit />} />
              <Route path="clusters/:clusterId/network/ingress/:namespace/:name/edit" element={<IngressEdit />} />
              {/* 存储管理路由（PVC、PV、StorageClass） */}
              <Route path="clusters/:clusterId/storage" element={<StorageList />} />
              {/* 告警中心路由 */}
              <Route path="clusters/:clusterId/alerts" element={<AlertCenter />} />
              {/* 日志中心路由 */}
              <Route path="clusters/:clusterId/logs" element={<LogCenter />} />
              <Route path="clusters/:clusterId/logs/events" element={<EventLogs />} />
              {/* 监控中心路由 */}
              <Route path="clusters/:clusterId/monitoring" element={<MonitoringCenter />} />
              {/* ArgoCD 应用管理路由 - 需要运维权限 */}
              <Route path="clusters/:clusterId/plugins" element={
                <PermissionGuard requiredPermission="ops">
                  <ArgoCDApplicationsPage />
                </PermissionGuard>
              } />
              <Route path="clusters/:clusterId/argocd" element={
                <PermissionGuard requiredPermission="ops">
                  <ArgoCDApplicationsPage />
                </PermissionGuard>
              } />
              <Route path="clusters/:clusterId/argocd/config" element={
                <PermissionGuard requiredPermission="ops">
                  <ArgoCDConfigPage />
                </PermissionGuard>
              } />
              <Route path="clusters/:clusterId/argocd/applications" element={
                <PermissionGuard requiredPermission="ops">
                  <ArgoCDApplicationsPage />
                </PermissionGuard>
              } />
              <Route path="clusters/:clusterId/helm" element={
                <PermissionGuard requiredPermission="ops">
                  <HelmList />
                </PermissionGuard>
              } />
              {/* CRD 自動發現 */}
              <Route path="clusters/:clusterId/crds" element={<CRDList />} />
              <Route path="clusters/:clusterId/crds/:group/:version/:plural" element={<CRDResources />} />
              {/* Event 告警規則引擎 */}
              <Route path="clusters/:clusterId/event-alerts" element={<EventAlertRules />} />
              {/* 資源成本分析 */}
              <Route path="clusters/:clusterId/cost" element={<CostDashboard />} />
              {/* 安全掃描 */}
              <Route path="clusters/:id/security" element={<SecurityDashboard />} />
              {/* 审计管理路由 - 仅平台管理员 */}
              <Route path="audit/operations" element={
                <PermissionGuard platformAdminOnly>
                  <OperationLogs />
                </PermissionGuard>
              } />
              <Route path="audit/commands" element={
                <PermissionGuard platformAdminOnly>
                  <CommandHistory />
                </PermissionGuard>
              } />
              {/* 访问控制路由 - 仅平台管理员 */}
              <Route path="access/users" element={
                <PermissionGuard platformAdminOnly>
                  <UserManagement />
                </PermissionGuard>
              } />
              <Route path="access/user-groups" element={
                <PermissionGuard platformAdminOnly>
                  <UserGroupManagement />
                </PermissionGuard>
              } />
              <Route path="access/permissions" element={
                <PermissionGuard platformAdminOnly>
                  <PermissionManagement />
                </PermissionGuard>
              } />
              {/* 权限管理路由 - 兼容旧路由 */}
              <Route path="permissions" element={
                <PermissionGuard platformAdminOnly>
                  <PermissionManagement />
                </PermissionGuard>
              } />
              {/* 系统设置路由 - 仅平台管理员 */}
              <Route path="settings" element={
                <PermissionGuard platformAdminOnly>
                  <SystemSettings />
                </PermissionGuard>
              } />
              {/* 个人资料路由 */}
              <Route path="profile" element={<UserProfile />} />
            </Route>
          </Routes>
          </ErrorBoundary>
        </Router>
      </AntdApp>
    </ConfigProvider>
  );
};

// 主 App 组件（包含 Suspense 用于 i18n 加载）
const App: React.FC = () => {
  return (
    <Suspense fallback={
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        height: '100vh' 
      }}>
        <Spin size="large" />
      </div>
    }>
      <AppContent />
    </Suspense>
  );
};

export default App;
