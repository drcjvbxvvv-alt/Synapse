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
import MainLayout from '../layouts/MainLayout';
import { PermissionProvider } from '../contexts/PermissionContext.tsx';
import { PermissionGuard } from '../components/PermissionGuard';
import ErrorBoundary from '../components/ErrorBoundary';
import ErrorPage from '../components/ErrorPage';
import { RequireAuth } from './RequireAuth';

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
import PodLogs from '../pages/pod/PodLogs';
import PodTerminal from '../pages/pod/PodTerminal';
import WorkloadList from '../pages/workload/WorkloadList';
import WorkloadDetail from '../pages/workload/WorkloadDetail';
import DeploymentCreate from '../pages/workload/DeploymentCreate';
import DeploymentDetail from '../pages/workload/DeploymentDetail';
import RolloutDetail from '../pages/workload/RolloutDetail';
import AutoscalingPage from '../pages/workload/AutoscalingPage';
import GlobalSearch from '../pages/search/GlobalSearch';
import { ConfigSecretManagement, ConfigMapDetail, SecretDetail } from '../pages/config';
import ConfigMapEdit from '../pages/config/ConfigMapEdit';
import SecretEdit from '../pages/config/SecretEdit';
import ConfigMapCreate from '../pages/config/ConfigMapCreate';
import SecretCreate from '../pages/config/SecretCreate';
import { NamespaceList, NamespaceDetail } from '../pages/namespace';
import NetworkList from '../pages/network/NetworkList';
import ServiceEdit from '../pages/network/ServiceEdit';
import IngressEdit from '../pages/network/IngressEdit';
import StorageList from '../pages/storage/StorageList';
import { AlertCenter, GlobalAlertCenter } from '../pages/alert';
import EventAlertRules from '../pages/alert/EventAlertRules';
import { CommandHistory, OperationLogs } from '../pages/audit';
import { LogCenter, EventLogs } from '../pages/logs';
import { PermissionManagement } from '../pages/permission';
import { UserManagement, UserGroupManagement } from '../pages/access';
import SystemSettings from '../pages/settings/SystemSettings';
import UserProfile from '../pages/profile/UserProfile';

// ── Lazy imports (heavy / rarely visited) ─────────────────────────────────
const YAMLEditor              = lazy(() => import('../pages/yaml/YAMLEditor'));
const KubectlTerminalPage     = lazy(() => import('../pages/terminal/kubectlTerminal'));
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

// ─── Lazy wrapper ──────────────────────────────────────────────────────────
const S = ({ children }: { children: React.ReactNode }) => (
  <Suspense fallback={<Spin />}>{children}</Suspense>
);

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
        <Route index element={<Navigate to="/overview" replace />} />
        <Route path="overview" element={<Overview />} />

        {/* ── Clusters ─────────────────────────────────────────────────── */}
        <Route path="clusters" element={<ClusterList />} />
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
        <Route path="nodes" element={<NodeList />} />
        <Route path="nodes/:id" element={<NodeDetail />} />

        {/* ── Pods ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/pods" element={<PodList />} />
        <Route path="clusters/:clusterId/pods/:namespace/:name" element={<PodDetail />} />
        <Route path="clusters/:clusterId/pods/:namespace/:name/logs" element={<PodLogs />} />
        <Route path="clusters/:clusterId/pods/:namespace/:name/terminal" element={
          <ErrorBoundary fallbackType="section"><PodTerminal /></ErrorBoundary>
        } />

        {/* ── Workloads ────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/autoscaling" element={<AutoscalingPage />} />
        <Route path="clusters/:clusterId/workloads" element={<WorkloadList />} />
        <Route path="clusters/:clusterId/workloads/create" element={<DeploymentCreate />} />
        <Route path="clusters/:clusterId/workloads/deployment/:namespace/:name" element={<DeploymentDetail />} />
        <Route path="clusters/:clusterId/workloads/rollout/:namespace/:name" element={<RolloutDetail />} />
        <Route path="clusters/:clusterId/workloads/:type/:namespace/:name" element={<WorkloadDetail />} />
        <Route path="clusters/:clusterId/workloads/:namespace/:name" element={<WorkloadDetail />} />
        <Route path="workloads" element={<WorkloadList />} />
        <Route path="workloads/:type/:namespace/:name" element={<WorkloadDetail />} />

        {/* ── YAML ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/yaml/apply" element={
          <ErrorBoundary fallbackType="section"><S><YAMLEditor /></S></ErrorBoundary>
        } />

        {/* ── Search ───────────────────────────────────────────────────── */}
        <Route path="search" element={<GlobalSearch />} />

        {/* ── Alerts ───────────────────────────────────────────────────── */}
        <Route path="alerts" element={<GlobalAlertCenter />} />
        <Route path="clusters/:clusterId/alerts" element={<AlertCenter />} />
        <Route path="clusters/:clusterId/event-alerts" element={<EventAlertRules />} />

        {/* ── Namespaces ───────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/namespaces" element={<NamespaceList />} />
        <Route path="clusters/:clusterId/namespaces/:namespace" element={<NamespaceDetail />} />

        {/* ── Configs & Secrets ────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/configs" element={<ConfigSecretManagement />} />
        <Route path="clusters/:clusterId/configs/configmap/create" element={<ConfigMapCreate />} />
        <Route path="clusters/:clusterId/configs/configmap/:namespace/:name" element={<ConfigMapDetail />} />
        <Route path="clusters/:clusterId/configs/configmap/:namespace/:name/edit" element={<ConfigMapEdit />} />
        <Route path="clusters/:clusterId/configs/secret/create" element={<SecretCreate />} />
        <Route path="clusters/:clusterId/configs/secret/:namespace/:name" element={<SecretDetail />} />
        <Route path="clusters/:clusterId/configs/secret/:namespace/:name/edit" element={<SecretEdit />} />

        {/* ── Network ──────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/network" element={
          <ErrorBoundary fallbackType="section"><NetworkList /></ErrorBoundary>
        } />
        <Route path="clusters/:clusterId/network/service/:namespace/:name/edit" element={<ServiceEdit />} />
        <Route path="clusters/:clusterId/network/ingress/:namespace/:name/edit" element={<IngressEdit />} />

        {/* ── Storage ──────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/storage" element={<StorageList />} />

        {/* ── Logs ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/logs" element={
          <ErrorBoundary fallbackType="section"><LogCenter /></ErrorBoundary>
        } />
        <Route path="clusters/:clusterId/logs/events" element={
          <ErrorBoundary fallbackType="section"><EventLogs /></ErrorBoundary>
        } />

        {/* ── Monitoring ───────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/monitoring" element={
          <ErrorBoundary fallbackType="section"><S><MonitoringCenter /></S></ErrorBoundary>
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
          <PermissionGuard requiredPermission="ops"><S><HelmList /></S></PermissionGuard>
        } />

        {/* ── CRDs ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/crds" element={<S><CRDList /></S>} />
        <Route path="clusters/:clusterId/crds/:group/:version/:plural" element={<S><CRDResources /></S>} />

        {/* ── Cost ─────────────────────────────────────────────────────── */}
        <Route path="clusters/:clusterId/cost-insights" element={<S><CostDashboard /></S>} />
        <Route path="cost-insights" element={<S><GlobalCostInsights /></S>} />

        {/* ── Security ─────────────────────────────────────────────────── */}
        <Route path="clusters/:id/security" element={<S><SecurityDashboard /></S>} />
        <Route path="clusters/:id/certificates" element={<S><CertificateList /></S>} />

        {/* ── SLO / Chaos / Compliance ─────────────────────────────────── */}
        <Route path="clusters/:clusterId/slos" element={<S><SLOListPage /></S>} />
        <Route path="clusters/:clusterId/chaos" element={<S><ChaosPage /></S>} />
        <Route path="clusters/:clusterId/compliance" element={<S><CompliancePage /></S>} />

        {/* ── Multi-cluster ────────────────────────────────────────────── */}
        <Route path="multicluster" element={<S><MultiClusterPage /></S>} />

        {/* ── Audit (platform admin only) ──────────────────────────────── */}
        <Route path="audit/operations" element={
          <PermissionGuard platformAdminOnly><OperationLogs /></PermissionGuard>
        } />
        <Route path="audit/commands" element={
          <PermissionGuard platformAdminOnly><CommandHistory /></PermissionGuard>
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
        {/* Legacy route compatibility */}
        <Route path="permissions" element={
          <PermissionGuard platformAdminOnly><PermissionManagement /></PermissionGuard>
        } />

        {/* ── Settings (platform admin only) ───────────────────────────── */}
        <Route path="settings" element={
          <PermissionGuard platformAdminOnly><SystemSettings /></PermissionGuard>
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
