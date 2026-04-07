import { request } from '../utils/api';
import type { ApiResponse, User, LDAPConfig, SSHConfig, GrafanaConfig, GrafanaDashboardSyncStatus, GrafanaDataSourceSyncStatus, MyPermissionsResponse } from '../types';

// 登入請求參數
export interface LoginRequest {
  username: string;
  password: string;
  auth_type?: 'local' | 'ldap';
}

// 登入響應
export interface LoginResponse {
  token: string;
  user: User;
  expires_at: number;
  permissions?: MyPermissionsResponse[];
}

// 認證狀態
export interface AuthStatus {
  ldap_enabled: boolean;
}

// 修改密碼請求
export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

// LDAP測試認證請求
export interface TestLDAPAuthRequest {
  username: string;
  password: string;
  server?: string;
  port?: number;
  use_tls?: boolean;
  skip_tls_verify?: boolean;
  bind_dn?: string;
  bind_password?: string;
  base_dn?: string;
  user_filter?: string;
  username_attr?: string;
  email_attr?: string;
  display_name_attr?: string;
  group_filter?: string;
  group_attr?: string;
}

// LDAP測試認證響應
export interface TestLDAPAuthResponse {
  success: boolean;
  error?: string;
  username?: string;
  email?: string;
  display_name?: string;
  groups?: string[];
}

// 認證服務
export const authService = {
  // 使用者登入
  login: (data: LoginRequest): Promise<ApiResponse<LoginResponse>> => {
    return request.post<LoginResponse>('/auth/login', data);
  },

  // 使用者登出
  logout: (): Promise<ApiResponse<null>> => {
    return request.post<null>('/auth/logout');
  },

  // 獲取當前使用者資訊
  getProfile: (): Promise<ApiResponse<User>> => {
    return request.get<User>('/auth/me');
  },

  // 獲取認證狀態（是否啟用LDAP）
  getAuthStatus: (): Promise<ApiResponse<AuthStatus>> => {
    return request.get<AuthStatus>('/auth/status');
  },

  // 修改密碼
  changePassword: (data: ChangePasswordRequest): Promise<ApiResponse<null>> => {
    return request.post<null>('/auth/change-password', data);
  },
};

// 系統設定服務
export const systemSettingService = {
  // 獲取LDAP配置
  getLDAPConfig: (): Promise<ApiResponse<LDAPConfig>> => {
    return request.get<LDAPConfig>('/system/ldap/config');
  },

  // 更新LDAP配置
  updateLDAPConfig: (config: LDAPConfig): Promise<ApiResponse<null>> => {
    return request.put<null>('/system/ldap/config', config);
  },

  // 測試LDAP連線
  testLDAPConnection: (config: LDAPConfig): Promise<ApiResponse<{ success: boolean; error?: string }>> => {
    return request.post<{ success: boolean; error?: string }>('/system/ldap/test-connection', config);
  },

  // 測試LDAP使用者認證
  testLDAPAuth: (data: TestLDAPAuthRequest): Promise<ApiResponse<TestLDAPAuthResponse>> => {
    return request.post<TestLDAPAuthResponse>('/system/ldap/test-auth', data);
  },

  // 獲取SSH配置
  getSSHConfig: (): Promise<ApiResponse<SSHConfig>> => {
    return request.get<SSHConfig>('/system/ssh/config');
  },

  // 更新SSH配置
  updateSSHConfig: (config: SSHConfig): Promise<ApiResponse<null>> => {
    return request.put<null>('/system/ssh/config', config);
  },

  // 獲取SSH憑據（用於自動連線）
  getSSHCredentials: (): Promise<ApiResponse<SSHConfig>> => {
    return request.get<SSHConfig>('/system/ssh/credentials');
  },

  // 獲取 Grafana 配置
  getGrafanaConfig: (): Promise<ApiResponse<GrafanaConfig>> => {
    return request.get<GrafanaConfig>('/system/grafana/config');
  },

  // 更新 Grafana 配置
  updateGrafanaConfig: (config: GrafanaConfig): Promise<ApiResponse<null>> => {
    return request.put<null>('/system/grafana/config', config);
  },

  // 測試 Grafana 連線
  testGrafanaConnection: (config: GrafanaConfig): Promise<ApiResponse<{ success: boolean; error?: string }>> => {
    return request.post<{ success: boolean; error?: string }>('/system/grafana/test-connection', config);
  },

  // 獲取 Grafana Dashboard 同步狀態
  getGrafanaDashboardStatus: (): Promise<ApiResponse<GrafanaDashboardSyncStatus>> => {
    return request.get<GrafanaDashboardSyncStatus>('/system/grafana/dashboard-status');
  },

  // 同步 Grafana Dashboard
  syncGrafanaDashboards: (): Promise<ApiResponse<GrafanaDashboardSyncStatus>> => {
    return request.post<GrafanaDashboardSyncStatus>('/system/grafana/sync-dashboards');
  },

  // 獲取 Grafana 資料來源同步狀態
  getGrafanaDataSourceStatus: (): Promise<ApiResponse<GrafanaDataSourceSyncStatus>> => {
    return request.get<GrafanaDataSourceSyncStatus>('/system/grafana/datasource-status');
  },

  // 同步 Grafana 資料來源
  syncGrafanaDataSources: (): Promise<ApiResponse<GrafanaDataSourceSyncStatus>> => {
    return request.post<GrafanaDataSourceSyncStatus>('/system/grafana/sync-datasources');
  },
};

// Token 管理工具
export const tokenManager = {
  // 獲取 token
  getToken: (): string | null => {
    return localStorage.getItem('token');
  },

  // 設定 token
  setToken: (token: string): void => {
    localStorage.setItem('token', token);
  },

  // 移除 token
  removeToken: (): void => {
    localStorage.removeItem('token');
  },

  // 獲取使用者資訊
  getUser: (): User | null => {
    const userStr = localStorage.getItem('user');
    if (userStr) {
      try {
        return JSON.parse(userStr);
      } catch {
        return null;
      }
    }
    return null;
  },

  // 設定使用者資訊
  setUser: (user: User): void => {
    localStorage.setItem('user', JSON.stringify(user));
  },

  // 移除使用者資訊
  removeUser: (): void => {
    localStorage.removeItem('user');
  },

  // 獲取權限資訊
  getPermissions: (): MyPermissionsResponse[] => {
    const permStr = localStorage.getItem('permissions');
    if (permStr) {
      try {
        return JSON.parse(permStr);
      } catch {
        return [];
      }
    }
    return [];
  },

  // 設定權限資訊
  setPermissions: (permissions: MyPermissionsResponse[]): void => {
    localStorage.setItem('permissions', JSON.stringify(permissions));
  },

  // 移除權限資訊
  removePermissions: (): void => {
    localStorage.removeItem('permissions');
  },

  // 檢查是否已登入
  isLoggedIn: (): boolean => {
    const token = localStorage.getItem('token');
    const expiresAt = localStorage.getItem('token_expires_at');
    
    if (!token || !expiresAt) {
      return false;
    }

    // 檢查 token 是否過期
    const expiresAtNum = parseInt(expiresAt, 10);
    if (Date.now() / 1000 > expiresAtNum) {
      // Token 已過期，清理儲存
      tokenManager.clear();
      return false;
    }

    return true;
  },

  // 設定過期時間
  setExpiresAt: (expiresAt: number | undefined | null): void => {
    const val = expiresAt ?? Math.floor(Date.now() / 1000) + 86400;
    localStorage.setItem('token_expires_at', String(val));
  },

  // 清除所有認證資訊
  clear: (): void => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    localStorage.removeItem('token_expires_at');
    localStorage.removeItem('permissions');
  },
};

export default authService;
