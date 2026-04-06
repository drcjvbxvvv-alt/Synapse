import { request } from '../utils/api';

// ========== 型別定義 ==========

// 認證配置
export interface MonitoringAuth {
  type: 'none' | 'basic' | 'bearer';
  username?: string;
  password?: string;
  token?: string;
}

// Alertmanager 配置
export interface AlertManagerConfig {
  enabled: boolean;
  endpoint: string;
  auth?: MonitoringAuth | null;
  options?: Record<string, unknown>;
}

// 告警狀態
export interface AlertStatus {
  state: 'active' | 'suppressed' | 'resolved';
  silencedBy: string[];
  inhibitedBy: string[];
}

// 告警
export interface Alert {
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL: string;
  fingerprint: string;
  status: AlertStatus;
}

// 告警分組
export interface AlertGroup {
  labels: Record<string, string>;
  receiver: string;
  alerts: Alert[];
}

// 匹配器
export interface Matcher {
  name: string;
  value: string;
  isRegex: boolean;
  isEqual: boolean;
}

// 靜默狀態
export interface SilenceStatus {
  state: 'active' | 'pending' | 'expired';
}

// 靜默規則
export interface Silence {
  id: string;
  matchers: Matcher[];
  startsAt: string;
  endsAt: string;
  createdBy: string;
  comment: string;
  status: SilenceStatus;
  updatedAt?: string;
}

// 建立靜默規則請求
export interface CreateSilenceRequest {
  matchers: Matcher[];
  startsAt: string;
  endsAt: string;
  createdBy: string;
  comment: string;
}

// 告警統計
export interface AlertStats {
  total: number;
  firing: number;
  pending: number;
  resolved: number;
  suppressed: number;
  bySeverity: Record<string, number>;
}

// 接收器
export interface Receiver {
  name: string;
}

// Alertmanager 狀態
export interface AlertManagerStatus {
  cluster: {
    name: string;
    status: string;
    peers: { name: string; address: string }[];
  };
  versionInfo: {
    version: string;
    revision: string;
    branch: string;
    buildUser: string;
    buildDate: string;
    goVersion: string;
  };
  config: {
    original: string;
  };
  uptime: string;
}

// 告警查詢參數
export interface AlertQueryParams {
  severity?: string;
  alertname?: string;
  filter?: string;
}

// ========== API 服務 ==========

export const alertService = {
  // ========== 配置相關 ==========
  
  // 獲取 Alertmanager 配置
  getConfig: (clusterId: string | number) => {
    return request.get<AlertManagerConfig>(`/clusters/${clusterId}/alertmanager/config`);
  },

  // 更新 Alertmanager 配置
  updateConfig: (clusterId: string | number, config: AlertManagerConfig) => {
    return request.put<void>(`/clusters/${clusterId}/alertmanager/config`, config);
  },

  // 測試 Alertmanager 連線
  testConnection: (clusterId: string | number, config: AlertManagerConfig) => {
    return request.post<void>(`/clusters/${clusterId}/alertmanager/test-connection`, config);
  },

  // 獲取 Alertmanager 狀態
  getStatus: (clusterId: string | number) => {
    return request.get<AlertManagerStatus>(`/clusters/${clusterId}/alertmanager/status`);
  },

  // 獲取配置模板
  getConfigTemplate: (clusterId: string | number) => {
    return request.get<AlertManagerConfig>(`/clusters/${clusterId}/alertmanager/template`);
  },

  // ========== 告警相關 ==========

  // 獲取告警列表
  getAlerts: (clusterId: string | number, params?: AlertQueryParams) => {
    return request.get<Alert[]>(`/clusters/${clusterId}/alerts`, { params });
  },

  // 獲取告警分組
  getAlertGroups: (clusterId: string | number) => {
    return request.get<AlertGroup[]>(`/clusters/${clusterId}/alerts/groups`);
  },

  // 獲取告警統計
  getAlertStats: (clusterId: string | number) => {
    return request.get<AlertStats>(`/clusters/${clusterId}/alerts/stats`);
  },

  // ========== 靜默規則相關 ==========

  // 獲取靜默規則列表
  getSilences: (clusterId: string | number) => {
    return request.get<Silence[]>(`/clusters/${clusterId}/silences`);
  },

  // 建立靜默規則
  createSilence: (clusterId: string | number, silence: CreateSilenceRequest) => {
    return request.post<Silence>(`/clusters/${clusterId}/silences`, silence);
  },

  // 刪除靜默規則
  deleteSilence: (clusterId: string | number, silenceId: string) => {
    return request.delete<void>(`/clusters/${clusterId}/silences/${silenceId}`);
  },

  // ========== 接收器相關 ==========

  // 獲取接收器列表
  getReceivers: (clusterId: string | number) => {
    return request.get<Receiver[]>(`/clusters/${clusterId}/receivers`);
  },
};

export default alertService;

