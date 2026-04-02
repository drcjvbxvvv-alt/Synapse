import { request } from '../utils/api';

export interface HelmRelease {
  name: string;
  namespace: string;
  chart: string;
  version: string;
  app_version: string;
  status: string;
  revision: number;
  updated_at: string;
}

export interface HelmReleaseDetail extends HelmRelease {
  values?: Record<string, unknown>;
  notes?: string;
}

export interface HelmHistory {
  revision: number;
  updated_at: string;
  status: string;
  chart: string;
  app_version: string;
  description: string;
}

export interface HelmRepo {
  id: number;
  name: string;
  url: string;
  username?: string;
}

export interface ChartInfo {
  name: string;
  version: string;
  description: string;
  repo_name: string;
}

export interface InstallReleaseRequest {
  namespace: string;
  release_name: string;
  repo_name: string;
  chart_name: string;
  version?: string;
  values?: string;
}

export interface UpgradeReleaseRequest {
  values?: string;
  version?: string;
}

const helmService = {
  listReleases: (clusterId: string, namespace?: string) =>
    request.get<{ items: HelmRelease[]; total: number }>(
      `/clusters/${clusterId}/helm/releases${namespace ? `?namespace=${namespace}` : ''}`
    ),

  getRelease: (clusterId: string, namespace: string, name: string) =>
    request.get<HelmReleaseDetail>(`/clusters/${clusterId}/helm/releases/${namespace}/${name}`),

  getHistory: (clusterId: string, namespace: string, name: string) =>
    request.get<HelmHistory[]>(`/clusters/${clusterId}/helm/releases/${namespace}/${name}/history`),

  getValues: (clusterId: string, namespace: string, name: string, all = false) =>
    request.get<Record<string, unknown>>(
      `/clusters/${clusterId}/helm/releases/${namespace}/${name}/values${all ? '?all=true' : ''}`
    ),

  installRelease: (clusterId: string, data: InstallReleaseRequest) =>
    request.post<HelmRelease>(`/clusters/${clusterId}/helm/releases`, data),

  upgradeRelease: (
    clusterId: string,
    namespace: string,
    name: string,
    data: UpgradeReleaseRequest
  ) =>
    request.put<HelmRelease>(
      `/clusters/${clusterId}/helm/releases/${namespace}/${name}`,
      data
    ),

  rollbackRelease: (
    clusterId: string,
    namespace: string,
    name: string,
    revision: number
  ) =>
    request.post<void>(
      `/clusters/${clusterId}/helm/releases/${namespace}/${name}/rollback`,
      { revision }
    ),

  uninstallRelease: (clusterId: string, namespace: string, name: string) =>
    request.delete<void>(`/clusters/${clusterId}/helm/releases/${namespace}/${name}`),

  listRepos: () => request.get<HelmRepo[]>('/helm/repos'),

  addRepo: (data: {
    name: string;
    url: string;
    username?: string;
    password?: string;
  }) => request.post<HelmRepo>('/helm/repos', data),

  removeRepo: (name: string) => request.delete<void>(`/helm/repos/${name}`),

  searchCharts: (keyword?: string) =>
    request.get<ChartInfo[]>(`/helm/repos/charts${keyword ? `?keyword=${keyword}` : ''}`),
};

export default helmService;
