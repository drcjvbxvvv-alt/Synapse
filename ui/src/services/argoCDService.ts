import api from '../utils/api';

// ArgoCD 配置
export interface ArgoCDConfig {
  id?: number;
  cluster_id?: number;
  enabled: boolean;
  
  // ArgoCD 伺服器配置
  server_url: string;
  auth_type: 'token' | 'username';
  token?: string;
  username?: string;
  password?: string;
  insecure: boolean;
  
  // Git 倉庫配置
  git_repo_url: string;
  git_branch: string;
  git_path: string;
  git_auth_type: 'https' | 'ssh';
  git_username?: string;
  git_password?: string;
  git_ssh_key?: string;
  
  // ArgoCD 叢集配置
  argocd_cluster_name: string;
  argocd_project: string;
  
  // 狀態
  connection_status?: string;
  last_test_at?: string;
  error_message?: string;
}

// ArgoCD 應用
export interface ArgoCDApplication {
  name: string;
  namespace: string;
  project: string;
  source: ArgoCDSource;
  destination: ArgoCDDestination;
  sync_status: string;
  health_status: string;
  synced_revision: string;
  target_revision: string;
  created_at: string;
  reconciled_at: string;
  resources?: ArgoCDResource[];
  history?: ArgoCDSyncHistory[];
}

export interface ArgoCDSource {
  repo_url: string;
  path: string;
  target_revision: string;
  helm?: ArgoCDHelmSource;
}

export interface ArgoCDHelmSource {
  value_files?: string[];
  values?: string;
  parameters?: { name: string; value: string }[];
}

export interface ArgoCDDestination {
  server: string;
  namespace: string;
  name?: string;
}

export interface ArgoCDResource {
  group: string;
  kind: string;
  namespace: string;
  name: string;
  status: string;
  health: string;
  message?: string;
}

export interface ArgoCDSyncHistory {
  id: number;
  revision: string;
  deployed_at: string;
  source: ArgoCDSource;
}

// 建立應用請求
export interface CreateApplicationRequest {
  name: string;
  namespace?: string;
  project?: string;
  path: string;
  target_revision?: string;
  dest_namespace: string;
  helm_values?: string;
  helm_parameters?: Record<string, string>;
  auto_sync?: boolean;
  self_heal?: boolean;
  prune?: boolean;
}

// 同步請求
export interface SyncApplicationRequest {
  revision?: string;
  prune?: boolean;
  dry_run?: boolean;
}

// 回滾請求
export interface RollbackApplicationRequest {
  revision_id: number;
}

// API 響應型別（後端直接返回資料體，無包裝）
type ApiResponse<T> = T;

export const argoCDService = {
  // ==================== 配置管理 ====================
  
  /**
   * 獲取 ArgoCD 配置
   */
  async getConfig(clusterId: string): Promise<ApiResponse<ArgoCDConfig>> {
    const response = await api.get(`/clusters/${clusterId}/argocd/config`);
    return response.data;
  },

  /**
   * 儲存 ArgoCD 配置
   */
  async saveConfig(clusterId: string, config: Partial<ArgoCDConfig>): Promise<ApiResponse<null>> {
    const response = await api.put(`/clusters/${clusterId}/argocd/config`, config);
    return response.data;
  },

  /**
   * 測試 ArgoCD 連線
   */
  async testConnection(clusterId: string, config: Partial<ArgoCDConfig>): Promise<ApiResponse<{ connected: boolean }>> {
    const response = await api.post(`/clusters/${clusterId}/argocd/test-connection`, config);
    return response.data;
  },

  // ==================== 應用管理 ====================
  
  /**
   * 獲取應用列表
   */
  async listApplications(clusterId: string): Promise<ApiResponse<{ items: ArgoCDApplication[]; total: number }>> {
    const response = await api.get(`/clusters/${clusterId}/argocd/applications`);
    return response.data;
  },

  /**
   * 獲取應用詳情
   */
  async getApplication(clusterId: string, appName: string): Promise<ApiResponse<ArgoCDApplication>> {
    const response = await api.get(`/clusters/${clusterId}/argocd/applications/${appName}`);
    return response.data;
  },

  /**
   * 建立應用
   */
  async createApplication(clusterId: string, request: CreateApplicationRequest): Promise<ApiResponse<ArgoCDApplication>> {
    const response = await api.post(`/clusters/${clusterId}/argocd/applications`, request);
    return response.data;
  },

  /**
   * 更新應用
   */
  async updateApplication(clusterId: string, appName: string, request: CreateApplicationRequest): Promise<ApiResponse<ArgoCDApplication>> {
    const response = await api.put(`/clusters/${clusterId}/argocd/applications/${appName}`, request);
    return response.data;
  },

  /**
   * 刪除應用
   */
  async deleteApplication(clusterId: string, appName: string, cascade = true): Promise<ApiResponse<null>> {
    const response = await api.delete(`/clusters/${clusterId}/argocd/applications/${appName}?cascade=${cascade}`);
    return response.data;
  },

  /**
   * 同步應用
   */
  async syncApplication(clusterId: string, appName: string, request?: SyncApplicationRequest): Promise<ApiResponse<null>> {
    const response = await api.post(`/clusters/${clusterId}/argocd/applications/${appName}/sync`, request || {});
    return response.data;
  },

  /**
   * 回滾應用
   */
  async rollbackApplication(clusterId: string, appName: string, request: RollbackApplicationRequest): Promise<ApiResponse<null>> {
    const response = await api.post(`/clusters/${clusterId}/argocd/applications/${appName}/rollback`, request);
    return response.data;
  },

  /**
   * 獲取應用資源樹
   */
  async getApplicationResources(clusterId: string, appName: string): Promise<ApiResponse<ArgoCDResource[]>> {
    const response = await api.get(`/clusters/${clusterId}/argocd/applications/${appName}/resources`);
    return response.data;
  },
};

export default argoCDService;

