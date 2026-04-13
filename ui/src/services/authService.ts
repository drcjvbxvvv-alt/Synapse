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

  // 更新個人資料
  updateProfile: (data: { display_name?: string; email?: string }): Promise<ApiResponse<User>> => {
    return request.put<User>('/auth/me', data);
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
//
// P0-6 安全強化：Access Token 僅儲存於記憶體，不寫入 localStorage
// - 目的：XSS 攻擊無法透過 `localStorage.token` 取得 token
// - 取捨：頁面重新整理後 token 消失，使用者需要重新登入
//        （Phase 1 將補 httpOnly cookie + Refresh Token 雙 token 機制）
// - user / permissions / expiresAt 仍保留於 localStorage，
//   僅用於「知道上次曾登入」以提供較佳的 UX 導引，非認證依據
let accessToken: string | null = null;
let accessTokenExpiresAt: number | null = null;

export const tokenManager = {
  // 獲取 token（僅從記憶體）
  getToken: (): string | null => {
    return accessToken;
  },

  // 設定 token（僅寫入記憶體）
  setToken: (token: string): void => {
    accessToken = token;
  },

  // 移除 token（僅清理記憶體）
  removeToken: (): void => {
    accessToken = null;
    accessTokenExpiresAt = null;
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
  //
  // P0-6 後：以「記憶體中是否有未過期的 token」為準。
  // 頁面重新整理後 accessToken 為 null，此函式將回傳 false，
  // 呼叫方（路由守衛）應導向登入頁，由使用者重新取得 token。
  isLoggedIn: (): boolean => {
    if (!accessToken || !accessTokenExpiresAt) {
      return false;
    }
    if (Date.now() / 1000 > accessTokenExpiresAt) {
      tokenManager.clear();
      return false;
    }
    return true;
  },

  // 設定過期時間
  //
  // 為協助前端 UI 顯示「上次登入時間」以及 session 倒數計時，
  // 仍將過期時間寫入 localStorage，但 token 本身只留在記憶體中。
  // localStorage 中的 expiresAt 只是 UX 提示，並非認證依據。
  setExpiresAt: (expiresAt: number | undefined | null): void => {
    const val = expiresAt ?? Math.floor(Date.now() / 1000) + 86400;
    accessTokenExpiresAt = val;
    localStorage.setItem('token_expires_at', String(val));
  },

  // 獲取過期時間（UI 顯示用）
  getExpiresAt: (): number | null => {
    return accessTokenExpiresAt;
  },

  // 清除所有認證資訊
  clear: (): void => {
    accessToken = null;
    accessTokenExpiresAt = null;
    // 舊版相容：舊鍵若還存在也一併清理
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    localStorage.removeItem('token_expires_at');
    localStorage.removeItem('permissions');
  },
};

// ── Silent Refresh ──────────────────────────────────────────────────────────
//
// P1：httpOnly cookie 中的 refresh token → 換取新 access token。
// 頁面重新整理後 accessToken 為 null，呼叫此函式可在不重新登入的情況下
// 恢復 session。成功時回傳 true，失敗時回傳 false（應導向登入頁）。
//
// 注意：此函式使用原生 fetch（而非 axios instance），避免 axios interceptor
// 偵測到 401 時觸發循環導向邏輯。
export async function silentRefresh(): Promise<boolean> {
  try {
    const baseURL = (import.meta as unknown as { env: { VITE_API_BASE_URL?: string } }).env.VITE_API_BASE_URL || '/api/v1';
    const res = await fetch(`${baseURL}/auth/refresh`, {
      method: 'POST',
      credentials: 'include', // 傳送 httpOnly cookie
      headers: { 'Content-Type': 'application/json' },
    });

    if (!res.ok) return false;

    const body = await res.json();
    const data = body?.data ?? body;
    if (!data?.token) return false;

    tokenManager.setToken(data.token);
    tokenManager.setExpiresAt(data.expires_at);
    if (data.user) tokenManager.setUser(data.user);
    if (data.permissions) tokenManager.setPermissions(data.permissions);

    return true;
  } catch {
    return false;
  }
}

export default authService;
