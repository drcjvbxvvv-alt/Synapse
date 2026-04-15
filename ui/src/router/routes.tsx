/**
 * AppRoutes — all application route definitions.
 *
 * This file owns:
 *   - All page component imports (lazy + eager)
 *   - The full <Routes> tree
 *
 * App.tsx owns:
 *   - Provider stack (ConfigProvider, AntdApp, Router)
 *   - Auth bootstrap (useAuthInit, RequireAuth)
 *
 * Adding a new page: import here, add a <Route> here. Touch nothing else.
 */
import React, { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import MainLayout from '../layouts/MainLayout';
import { PermissionProvider } from '../contexts/PermissionContext.tsx';
import { PermissionGuard } from '../components/PermissionGuard';
import ErrorBoundary from '../components/ErrorBoundary';
import ErrorPage from '../components/ErrorPage';
import { RequireAuth } from './RequireAuth';
import { usePermission, usePermissionLoading } from '../hooks/usePermission';
import { tokenManager } from '../services/authService';
import { isPlatformAdmin } from '../config/menuPermissions';

// ── Eager imports (small / always needed) ──────────────────────────────────
import Login from '../pages/auth/Login';
import Overview from '../pages/overview/Overview';
import ClusterList from '../pages/cluster/ClusterList';
import ClusterDetail from '../pages/cluster/ClusterDetail';
import ClusterImport from '../pages/cluster/ClusterImport';
import ConfigCenter from '../pages/cluster/ConfigCenter';
import ClusterUpgrade from '../pages/cluster/ClusterUpgrade';
import NodeList from '../pages/node/NodeList';
import NodeDetail from '../pages/node/NodeDetail';
import PodList from '../pages/pod/PodList';
import PodDetail from '../pages/pod/PodDetail';
import WorkloadList from '../pages/workload/WorkloadList';
import WorkloadDetail from '../pages/workload/WorkloadDetail';
import DeploymentDetail from '../pages/workload/DeploymentDetail';
import RolloutDetail from '../pages/workload/RolloutDetail';
import AutoscalingPage from '../pages/workload/AutoscalingPage';
import { ConfigSecretManagement, ConfigMapDetail, SecretDetail } from '../pages/config';
import { NamespaceList, NamespaceDetail } from '../pages/namespace';
import NetworkList from '../pages/network/NetworkList';
import StorageList from '../pages/storage/StorageList';
import { AlertCenter, GlobalAlertCenter } from '../pages/alert';
import { LogCenter, EventLogs } from '../pages/logs';
import { PermissionManagement } from '../pages/permission';
import { UserManagement, UserGroupManagement, FeaturePolicyPage } from '../pages/access';
import SystemSettings from '../pages/settings/SystemSettings';
import UserProfile from '../pages/profile/UserProfile';

// ── Lazy imports (heavy / rarely visited) ─────────────────────────────────
// Previously eager — moved to lazy to reduce initial bundle size (F-BUNDLE-1)
const PodLogs                 = lazy(() => import('../pages/pod/PodLogs'));
const PodTerminal             = lazy(() => import('../pages/pod/PodTerminal'));
const DeploymentCreate        = lazy(() => import('../pages/workload/DeploymentCreate'));
const GlobalSearch            = lazy(() => import('../pages/search/GlobalSearch'));
const ConfigMapEdit           = lazy(() => import('../pages/config/ConfigMapEdit'));
const SecretEdit              = lazy(() => import('../pages/config/SecretEdit'));
const ConfigMapCreate         = lazy(() => import('../pages/config/ConfigMapCreate'));
const SecretCreate            = lazy(() => import('../pages/config/SecretCreate'));
const ServiceEdit             = lazy(() => import('../pages/network/ServiceEdit'));
const IngressEdit             = lazy(() => import('../pages/network/IngressEdit'));
const EventAlertRules         = lazy(() => import('../pages/alert/EventAlertRules'));
const OperationLogs           = lazy(() => import('../pages/audit/OperationLogs'));
const CommandHistory          = lazy(() => import('../pages/audit/CommandHistory'));
const YAMLEditor              = lazy(() => import('../pages/yaml/YAMLEditor'));
const KubectlTerminalPage     = lazy(() => import('../pages/terminal/KubectlTerminal'));
const ArgoCDConfigPage        = lazy(() => import('../pages/plugins/ArgoCDConfigPage'));
const ArgoCDApplicationsPage  = lazy(() => import('../pages/plugins/ArgoCDApplicationsPage'));
const MonitoringCenter        = lazy(() => import('../pages/om').then(m => ({ default: m.MonitoringCenter })));
const HelmList                = lazy(() => import('../pages/helm/HelmList'));
const CRDList                 = lazy(() => import('../pages/crd/CRDList'));
const CRDResources            = lazy(() => import('../pages/crd/CRDResources'));
const CostDashboard           = lazy(() => import('../pages/cost/CostDashboard'));
const GlobalCostInsights      = lazy(() => import('../pages/cost/GlobalCostInsights'));
const SecurityDashboard       = lazy(() => import('../pages/security/SecurityDashboard'));
const CertificateList         = lazy(() => import('../pages/security/CertificateList'));
const MultiClusterPage        = lazy(() => import('../pages/multicluster'));
const SLOListPage             = lazy(() => import('../pages/slo'));
const ChaosPage               = lazy(() => import('../pages/chaos'));
const CompliancePage          = lazy(() => import('../pages/compliance'));
const PipelineRunDemo         = lazy(() => import('../pages/pipeline/PipelineRunDemo'));
const PipelineList            = lazy(() => import('../pages/pipeline/PipelineList'));
const PipelineRunDetail       = lazy(() => import('../pages/pipeline/PipelineRunDetail'));

// ─── Lazy wrapper ──────────────────────────────────────────────────────────
const S = ({ children }: { children: React.ReactNode }) => (
  <Suspense fallback={<Spin />}>{children}</Suspense>
);

// ─── Redirect helpers ──────────────────────────────────────────────────────

/**
 * Returns the default destination for the current user:
 *   admin → /clusters   (cluster list)
 *   others → /clusters/:id/overview  (first assigned cluster)
 *   no clusters → /overview
 */
function useDefaultDestination(): string | null {
  const { clusterPermissions } = usePermission();
  const loading = usePermissionLoading();
  const user = tokenManager.getUser();

  if (loading) return null;

  const allPerms = Array.from(clusterPermissions.values());

  if (isPlatformAdmin(user?.username, allPerms)) {
    return '/clusters';
  }

  const first = allPerms[0];
  return first ? `/clusters/${first.cluster_id}/overview` : '/overview';
}

/**
 * Root-level redirect: admins go to the cluster list,
 * everyone else goes directly to their first assigned cluster.
 */
const HomeRedirect: React.FC = () => {
  const loading = usePermissionLoading();
  const dest = useDefaultDestination();

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  return <Navigate to={dest!} replace />;
};

/**
 * Guard for all top-level (non-cluster) pages.
 * Only admin or ops may access platform-wide views.
 */
const TopLevelGuard: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const loading = usePermissionLoading();
  const { clusterPermissions } = usePermission();
  const user = tokenManager.getUser();
  const { t } = useTranslation('components');

  if (loading) return <Spin style={{ display: 'block', margin: '80px auto' }} />;

  const allPerms = Array.from(clusterPermissions.values());
  const canAccess =
    isPlatformAdmin(user?.username, allPerms) ||
    allPerms.some(p => p.permission_type === 'ops');

  if (!canAccess) {
    return (
      <ErrorPage
        status={403}
        title={t('permissionGuard.insufficientPermission')}
        subTitle={t('permissionGuard.adminOpsOnly')}
        showBack
      />
    );
  }

  return <>{children}</>;
};

/**
 * Guards the /clusters list page.
 * Non-admin users are redirected to their first assigned cluster instead.
 */
const ClusterListRoute: React.FC = () => {
  const loading = usePermissionLoading();
  const dest = useDefaultDestination();
  const user = tokenManager.getUser();
  const { clusterPermissions } = usePermission();

  if (loading) return <Spin style={{ display: 'block', margin: '80px auto' }} />;

  const allPerms = Array.from(clusterPermissions.values());
  if (!isPlatformAdmin(user?.username, allPerms)) {
    return <Navigate to={dest ?? '/overview'} replace />;
  }

  return <ClusterList />;
};

// ─── Routes ────────────────────────────────────────────────────────────────

export function AppRoutes() {
  return (
    <Routes>
      {/* Public */}
      <Route path="/login" element={<Login />} />

      {/* Protected layout shell */}
      <Route
        path="/"
        element={
          <RequireAuth>
            <PermissionProvider>
              <MainLayout />
            </PermissionProvider>
          </RequireAuth>
        }
      >
        <Route index element={<HomeRedirect />} />
        <Route path="overview" element={<TopLevelGuard><Overview /></TopLevelGuard>} />

        {/* ── Clusters ─────────────────────────────────────────────────── */}
        <Route path="clusters" element={<ClusterListRoute />} />
        <Route path="clusters/import" element={<ClusterImport />} />
        <Route path="clusters/:id/overview" element={<ClusterDetail />} />
        <Route path="clusters/:clusterId/config-center" element={
          <PermissionGuard requiredPermission="ops"><ConfigCenter /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/upgrade" element={
          <PermissionGuard requiredPermission="admin"><ClusterUpgrade /></PermissionGuard>
        } />
        <Route path="clusters/:id/terminal" element={
          <ErrorBoundary fallbackType="section"><S><KubectlTerminalPage /></S></ErrorBoundary>
        } />

        {/* ── Nodes ────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/nodes" element={
          <PermissionGuard requiredPermission="ops"><NodeList /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/nodes/:nodeName" element={
          <PermissionGuard requiredPermission="ops"><NodeDetail /></PermissionGuard>
        } />
        <Route path="nodes" element={<TopLevelGuard><NodeList /></TopLevelGuard>} />
        <Route path="nodes/:id" element={<TopLevelGuard><NodeDetail /></TopLevelGuard>} />

        {/* ── Pods ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/pods" element={
          <PermissionGuard requiredFeature="workload:view"><PodList /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/pods/:namespace/:name" element={
          <PermissionGuard requiredFeature="workload:view"><PodDetail /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/pods/:namespace/:name/logs" element={
          <PermissionGuard requiredFeature="workload:view"><S><PodLogs /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/pods/:namespace/:name/terminal" element={
          <ErrorBoundary fallbackType="section"><S><PodTerminal /></S></ErrorBoundary>
        } />

        {/* ── Workloads ────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/autoscaling" element={
          <PermissionGuard requiredFeature="workload:view"><AutoscalingPage /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/workloads" element={
          <PermissionGuard requiredFeature="workload:view"><WorkloadList /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/workloads/create" element={
          <PermissionGuard requiredFeature="workload:view"><S><DeploymentCreate /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/workloads/deployment/:namespace/:name" element={
          <PermissionGuard requiredFeature="workload:view"><DeploymentDetail /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/workloads/rollout/:namespace/:name" element={
          <PermissionGuard requiredFeature="workload:view"><RolloutDetail /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/workloads/:type/:namespace/:name" element={
          <PermissionGuard requiredFeature="workload:view"><WorkloadDetail /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/workloads/:namespace/:name" element={
          <PermissionGuard requiredFeature="workload:view"><WorkloadDetail /></PermissionGuard>
        } />
        <Route path="workloads" element={<TopLevelGuard><WorkloadList /></TopLevelGuard>} />
        <Route path="workloads/:type/:namespace/:name" element={<TopLevelGuard><WorkloadDetail /></TopLevelGuard>} />

        {/* ── YAML ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/yaml/apply" element={
          <ErrorBoundary fallbackType="section"><S><YAMLEditor /></S></ErrorBoundary>
        } />

        {/* ── Search ───────────────────────────────────────────────────── */}
        <Route path="search" element={<TopLevelGuard><S><GlobalSearch /></S></TopLevelGuard>} />

        {/* ── Alerts ───────────────────────────────────────────────────── */}
        <Route path="alerts" element={<TopLevelGuard><GlobalAlertCenter /></TopLevelGuard>} />
        <Route path="clusters/:clusterId/alerts" element={<AlertCenter />} />
        <Route path="clusters/:clusterId/event-alerts" element={<S><EventAlertRules /></S>} />

        {/* ── Namespaces ───────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/namespaces" element={<PermissionGuard requiredFeature="namespace:view"><NamespaceList /></PermissionGuard>} />
        <Route path="clusters/:clusterId/namespaces/:namespace" element={<PermissionGuard requiredFeature="namespace:view"><NamespaceDetail /></PermissionGuard>} />

        {/* ── Configs & Secrets ────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/configs" element={
          <PermissionGuard requiredFeature="config:view"><ConfigSecretManagement /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/configs/configmap/create" element={
          <PermissionGuard requiredFeature="config:view"><S><ConfigMapCreate /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/configs/configmap/:namespace/:name" element={
          <PermissionGuard requiredFeature="config:view"><ConfigMapDetail /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/configs/configmap/:namespace/:name/edit" element={
          <PermissionGuard requiredFeature="config:view"><S><ConfigMapEdit /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/configs/secret/create" element={
          <PermissionGuard requiredFeature="config:view"><S><SecretCreate /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/configs/secret/:namespace/:name" element={
          <PermissionGuard requiredFeature="config:view"><SecretDetail /></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/configs/secret/:namespace/:name/edit" element={
          <PermissionGuard requiredFeature="config:view"><S><SecretEdit /></S></PermissionGuard>
        } />

        {/* ── Network ──────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/network" element={
          <PermissionGuard requiredFeature="network:view">
            <ErrorBoundary fallbackType="section"><NetworkList /></ErrorBoundary>
          </PermissionGuard>
        } />
        <Route path="clusters/:clusterId/network/service/:namespace/:name/edit" element={
          <PermissionGuard requiredFeature="network:view"><S><ServiceEdit /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/network/ingress/:namespace/:name/edit" element={
          <PermissionGuard requiredFeature="network:view"><S><IngressEdit /></S></PermissionGuard>
        } />

        {/* ── Storage ──────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/storage" element={
          <PermissionGuard requiredFeature="storage:view"><StorageList /></PermissionGuard>
        } />

        {/* ── Logs ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/logs" element={
          <PermissionGuard requiredFeature="logs:view">
            <ErrorBoundary fallbackType="section"><LogCenter /></ErrorBoundary>
          </PermissionGuard>
        } />
        <Route path="clusters/:clusterId/logs/events" element={
          <PermissionGuard requiredFeature="logs:view">
            <ErrorBoundary fallbackType="section"><EventLogs /></ErrorBoundary>
          </PermissionGuard>
        } />

        {/* ── Monitoring ───────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/monitoring" element={
          <PermissionGuard requiredFeature="monitoring:view">
            <ErrorBoundary fallbackType="section"><S><MonitoringCenter /></S></ErrorBoundary>
          </PermissionGuard>
        } />

        {/* ── ArgoCD ───────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/plugins" element={
          <PermissionGuard requiredPermission="ops"><S><ArgoCDApplicationsPage /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/argocd" element={
          <PermissionGuard requiredPermission="ops"><S><ArgoCDApplicationsPage /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/argocd/config" element={
          <PermissionGuard requiredPermission="ops"><S><ArgoCDConfigPage /></S></PermissionGuard>
        } />
        <Route path="clusters/:clusterId/argocd/applications" element={
          <PermissionGuard requiredPermission="ops"><S><ArgoCDApplicationsPage /></S></PermissionGuard>
        } />

        {/* ── Helm ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/helm" element={
          <PermissionGuard requiredPermission="ops" requiredFeature="helm:view"><S><HelmList /></S></PermissionGuard>
        } />

        {/* ── CRDs ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/crds" element={<S><CRDList /></S>} />
        <Route path="clusters/:clusterId/crds/:group/:version/:plural" element={<S><CRDResources /></S>} />

        {/* ── Cost ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/cost-insights" element={<S><CostDashboard /></S>} />
        <Route path="cost-insights" element={<TopLevelGuard><S><GlobalCostInsights /></S></TopLevelGuard>} />

        {/* ── Security ─────────────────────────────────────────────────── */}
        <Route path="clusters/:id/security" element={<S><SecurityDashboard /></S>} />
        <Route path="clusters/:id/certificates" element={<S><CertificateList /></S>} />

        {/* ── Pipelines ────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/pipelines" element={<S><PipelineList /></S>} />
        <Route path="clusters/:clusterId/pipelines/:pipelineId/runs/:runId" element={<S><PipelineRunDetail /></S>} />

        {/* ── SLO / Chaos / Compliance ─────────────────────────────────── */}
        <Route path="clusters/:clusterId/slos" element={<S><SLOListPage /></S>} />
        <Route path="clusters/:clusterId/chaos" element={<S><ChaosPage /></S>} />
        <Route path="clusters/:clusterId/compliance" element={<S><CompliancePage /></S>} />

        {/* ── Multi-cluster ────────────────────────────────────────────── */}
        <Route path="multicluster" element={<TopLevelGuard><S><MultiClusterPage /></S></TopLevelGuard>} />

        {/* ── Audit (platform admin only) ──────────────────────────────── */}
        <Route path="audit/operations" element={
          <PermissionGuard platformAdminOnly><S><OperationLogs /></S></PermissionGuard>
        } />
        <Route path="audit/commands" element={
          <PermissionGuard platformAdminOnly><S><CommandHistory /></S></PermissionGuard>
        } />

        {/* ── Access control (platform admin only) ─────────────────────── */}
        <Route path="access/users" element={
          <PermissionGuard platformAdminOnly><UserManagement /></PermissionGuard>
        } />
        <Route path="access/user-groups" element={
          <PermissionGuard platformAdminOnly><UserGroupManagement /></PermissionGuard>
        } />
        <Route path="access/permissions" element={
          <PermissionGuard platformAdminOnly><PermissionManagement /></PermissionGuard>
        } />
        <Route path="access/feature-policy" element={
          <PermissionGuard platformAdminOnly><FeaturePolicyPage /></PermissionGuard>
        } />
        {/* Legacy route compatibility */}
        <Route path="permissions" element={
          <PermissionGuard platformAdminOnly><PermissionManagement /></PermissionGuard>
        } />

        {/* ── Settings (admin + ops) ───────────────────────────────────── */}
        <Route path="settings" element={
          <TopLevelGuard><SystemSettings /></TopLevelGuard>
        } />

        {/* ── Profile ──────────────────────────────────────────────────── */}
        <Route path="profile" element={<UserProfile />} />
      </Route>

      {/* Design reference */}
      <Route path="pipeline-demo" element={<S><PipelineRunDemo /></S>} />

      {/* 404 */}
      <Route path="*" element={<ErrorPage status={404} showHome showBack={false} />} />
    </Routes>
  );
}
