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

// 接收器（簡易，來自 /receivers）
export interface Receiver {
  name: string;
}

// Email 告警配置
export interface EmailConfig {
  to: string;
  from?: string;
  smarthost?: string;
  authUsername?: string;
  authPassword?: string;
  requireTls?: boolean;
  headers?: Record<string, string>;
}

// Slack 告警配置
export interface SlackConfig {
  apiUrl: string;
  channel: string;
  username?: string;
  iconEmoji?: string;
  text?: string;
  title?: string;
}

// Webhook 告警配置
export interface WebhookConfig {
  url: string;
  sendResolved?: boolean;
  maxAlerts?: number;
}

// PagerDuty 告警配置
export interface PagerdutyConfig {
  routingKey: string;
  serviceKey?: string;
  url?: string;
  description?: string;
}

// 釘釘告警配置
export interface DingtalkConfig {
  apiUrl: string;
  secret?: string;
  message?: string;
}

// 完整 Receiver 配置
export interface ReceiverConfig {
  name: string;
  emailConfigs?: EmailConfig[];
  slackConfigs?: SlackConfig[];
  webhookConfigs?: WebhookConfig[];
  pagerdutyConfigs?: PagerdutyConfig[];
  dingtalkConfigs?: DingtalkConfig[];
}

// 測試 Receiver 請求
export interface TestReceiverRequest {
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
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

  // 獲取接收器列表（簡易）
  getReceivers: (clusterId: string | number) => {
    return request.get<Receiver[]>(`/clusters/${clusterId}/receivers`);
  },

  // 獲取完整 Receiver 設定（含各渠道詳細參數）
  getFullReceivers: (clusterId: string | number) => {
    return request.get<ReceiverConfig[]>(`/clusters/${clusterId}/receivers/full`);
  },

  // 新增 Receiver
  createReceiver: (clusterId: string | number, receiver: ReceiverConfig) => {
    return request.post<void>(`/clusters/${clusterId}/receivers`, receiver);
  },

  // 更新 Receiver
  updateReceiver: (clusterId: string | number, name: string, receiver: ReceiverConfig) => {
    return request.put<void>(`/clusters/${clusterId}/receivers/${encodeURIComponent(name)}`, receiver);
  },

  // 刪除 Receiver
  deleteReceiver: (clusterId: string | number, name: string) => {
    return request.delete<void>(`/clusters/${clusterId}/receivers/${encodeURIComponent(name)}`);
  },

  // 測試 Receiver
  testReceiver: (clusterId: string | number, name: string, req?: TestReceiverRequest) => {
    return request.post<void>(`/clusters/${clusterId}/receivers/${encodeURIComponent(name)}/test`, req ?? {});
  },
};

export default alertService;

