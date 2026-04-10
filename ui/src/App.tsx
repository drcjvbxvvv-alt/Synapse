import React, { Suspense, useEffect, useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { ConfigProvider, App as AntdApp, Spin } from 'antd';
import { useTranslation } from 'react-i18next';
// import zhCN from 'antd/locale/zh_CN'; // zh-CN 已停用，僅使用 zh-TW
import zhTW from 'antd/locale/zh_TW';
import enUS from 'antd/locale/en_US';
import MainLayout from './layouts/MainLayout';
import { synapseTheme } from './config/theme';
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
import GlobalCostInsights from './pages/cost/GlobalCostInsights';
import SecurityDashboard from './pages/security/SecurityDashboard';
import CertificateList from './pages/security/CertificateList';
import MultiClusterPage from './pages/multicluster';
import { PermissionProvider } from './contexts/PermissionContext.tsx';
import { tokenManager, silentRefresh } from './services/authService';
import { PermissionGuard } from './components/PermissionGuard';
import ErrorBoundary from './components/ErrorBoundary';
import ErrorPage from './components/ErrorPage'
import PipelineRunDemo from './pages/pipeline/PipelineRunDemo';
import AutoscalingPage from './pages/workload/AutoscalingPage';
import './App.css';

// ── Auth 初始化 ────────────────────────────────────────────────────────────
//
// 頁面重新整理後 accessToken 為 null，若 localStorage 中有 user 資訊（上次有登入過），
// 嘗試 silent refresh（用 httpOnly cookie 中的 refresh token）。
// 期間顯示全頁 Spinner，避免 RequireAuth 誤判為未登入並導向登入頁。
const useAuthInit = () => {
  const hasStoredSession = !!localStorage.getItem('user');
  const alreadyHasToken  = tokenManager.isLoggedIn();

  // 若 memory 已有 token，或 localStorage 無 session 記錄，不需要 refresh
  const [authReady, setAuthReady] = useState(alreadyHasToken || !hasStoredSession);

  useEffect(() => {
    if (authReady) return;

    silentRefresh().finally(() => {
      setAuthReady(true);
    });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return authReady;
};

// 認證保護元件
interface RequireAuthProps {
  children: React.ReactNode;
}

const RequireAuth: React.FC<RequireAuthProps> = ({ children }) => {
  const location = useLocation();

  if (!tokenManager.isLoggedIn()) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
};

// Ant Design locale 對映
const antdLocaleMap: Record<string, typeof zhTW> = {
  'zh-TW': zhTW,
  // 'zh-CN': zhCN, // zh-CN 已停用
  'en-US': enUS,
};

// 內部 App 元件（需要訪問 i18n hook）
const AppContent: React.FC = () => {
  const { i18n } = useTranslation();
  const currentLocale = antdLocaleMap[i18n.language] || zhTW;
  const authReady = useAuthInit();

  if (!authReady) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <ConfigProvider locale={currentLocale} theme={synapseTheme}>
      <AntdApp>
        <Router>
          <ErrorBoundary>
          <Routes>
            {/* 登入頁面 - 不需要認證 */}
            <Route path="/login" element={<Login />} />
            
            {/* 受保護的路由 */}
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
              {/* 配置中心 - 需要運維權限 */}
              <Route path="clusters/:clusterId/config-center" element={
                <PermissionGuard requiredPermission="ops">
                  <ConfigCenter />
                </PermissionGuard>
              } />
              {/* 叢集升級 - 需要管理員權限 */}
              <Route path="clusters/:clusterId/upgrade" element={
                <PermissionGuard requiredPermission="admin">
                  <ClusterUpgrade />
                </PermissionGuard>
              } />
              <Route path="clusters/import" element={<ClusterImport />} />
              <Route path="clusters/:id/terminal" element={<ErrorBoundary fallbackType="section"><KubectlTerminalPage /></ErrorBoundary>} />
              {/* 節點管理 - 需要運維權限 */}
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
              <Route path="clusters/:clusterId/pods/:namespace/:name/terminal" element={<ErrorBoundary fallbackType="section"><PodTerminal /></ErrorBoundary>} />
              <Route path="clusters/:clusterId/autoscaling" element={<AutoscalingPage />} />
              <Route path="clusters/:clusterId/workloads" element={<WorkloadList />} />
              <Route path="clusters/:clusterId/workloads/create" element={<DeploymentCreate />} />
              <Route path="clusters/:clusterId/workloads/deployment/:namespace/:name" element={<DeploymentDetail />} />
              <Route path="clusters/:clusterId/workloads/rollout/:namespace/:name" element={<RolloutDetail />} />
              <Route path="clusters/:clusterId/workloads/:type/:namespace/:name" element={<WorkloadDetail />} />
              <Route path="clusters/:clusterId/workloads/:namespace/:name" element={<WorkloadDetail />} />
              <Route path="clusters/:clusterId/yaml/apply" element={<ErrorBoundary fallbackType="section"><YAMLEditor /></ErrorBoundary>} />
              <Route path="workloads" element={<WorkloadList />} />
              <Route path="workloads/:type/:namespace/:name" element={<WorkloadDetail />} />
              <Route path="search" element={<GlobalSearch />} />
              {/* 全域性告警中心路由 */}
              <Route path="alerts" element={<GlobalAlertCenter />} />
              {/* 命名空間路由 */}
              <Route path="clusters/:clusterId/namespaces" element={<NamespaceList />} />
              <Route path="clusters/:clusterId/namespaces/:namespace" element={<NamespaceDetail />} />
              {/* 配置與金鑰路由 */}
              <Route path="clusters/:clusterId/configs" element={<ConfigSecretManagement />} />
              <Route path="clusters/:clusterId/configs/configmap/create" element={<ConfigMapCreate />} />
              <Route path="clusters/:clusterId/configs/configmap/:namespace/:name" element={<ConfigMapDetail />} />
              <Route path="clusters/:clusterId/configs/configmap/:namespace/:name/edit" element={<ConfigMapEdit />} />
              <Route path="clusters/:clusterId/configs/secret/create" element={<SecretCreate />} />
              <Route path="clusters/:clusterId/configs/secret/:namespace/:name" element={<SecretDetail />} />
              <Route path="clusters/:clusterId/configs/secret/:namespace/:name/edit" element={<SecretEdit />} />
              {/* 網路管理路由（Service和Ingress） */}
              <Route path="clusters/:clusterId/network" element={<ErrorBoundary fallbackType="section"><NetworkList /></ErrorBoundary>} />
              <Route path="clusters/:clusterId/network/service/:namespace/:name/edit" element={<ServiceEdit />} />
              <Route path="clusters/:clusterId/network/ingress/:namespace/:name/edit" element={<IngressEdit />} />
              {/* 儲存管理路由（PVC、PV、StorageClass） */}
              <Route path="clusters/:clusterId/storage" element={<StorageList />} />
              {/* 告警中心路由 */}
              <Route path="clusters/:clusterId/alerts" element={<AlertCenter />} />
              {/* 日誌中心路由 */}
              <Route path="clusters/:clusterId/logs" element={<ErrorBoundary fallbackType="section"><LogCenter /></ErrorBoundary>} />
              <Route path="clusters/:clusterId/logs/events" element={<ErrorBoundary fallbackType="section"><EventLogs /></ErrorBoundary>} />
              {/* 監控中心路由 */}
              <Route path="clusters/:clusterId/monitoring" element={<ErrorBoundary fallbackType="section"><MonitoringCenter /></ErrorBoundary>} />
              {/* ArgoCD 應用管理路由 - 需要運維權限 */}
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
              <Route path="clusters/:clusterId/cost-insights" element={<CostDashboard />} />
              {/* 安全掃描 */}
              <Route path="clusters/:id/security" element={<SecurityDashboard />} />
              {/* cert-manager 憑證管理 */}
              <Route path="clusters/:id/certificates" element={<CertificateList />} />
              {/* 成本洞察 - 跨叢集全局視角 */}
              <Route path="cost-insights" element={<GlobalCostInsights />} />
              {/* 多叢集工作流程 */}
              <Route path="multicluster" element={<MultiClusterPage />} />
              {/* 審計管理路由 - 僅平臺管理員 */}
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
              {/* 訪問控制路由 - 僅平臺管理員 */}
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
              {/* 權限管理路由 - 相容舊路由 */}
              <Route path="permissions" element={
                <PermissionGuard platformAdminOnly>
                  <PermissionManagement />
                </PermissionGuard>
              } />
              {/* 系統設定路由 - 僅平臺管理員 */}
              <Route path="settings" element={
                <PermissionGuard platformAdminOnly>
                  <SystemSettings />
                </PermissionGuard>
              } />
              {/* 個人資料路由 */}
              <Route path="profile" element={<UserProfile />} />
            </Route>
            {/* Pipeline 動畫展示（設計參照） */}
            <Route path="pipeline-demo" element={<PipelineRunDemo />} />
            {/* 404 — 未匹配路由 */}
            <Route path="*" element={<ErrorPage status={404} showHome showBack={false} />} />
          </Routes>
          </ErrorBoundary>
        </Router>
      </AntdApp>
    </ConfigProvider>
  );
};

// 主 App 元件（包含 Suspense 用於 i18n 載入）
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
