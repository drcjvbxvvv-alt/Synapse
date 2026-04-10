import { request } from '../utils/api';
import { buildWebSocketUrl } from '../utils/wsUrl';
import { tokenManager } from './authService';

// 日誌條目型別
export interface LogEntry {
  id: string;
  timestamp: string;
  type: string;
  level: string;
  cluster_id: number;
  cluster_name: string;
  namespace: string;
  pod_name: string;
  container: string;
  node_name: string;
  message: string;
  labels?: Record<string, string>;
  metadata?: Record<string, unknown>;
}

// K8s 事件日誌型別
export interface EventLogEntry {
  id: string;
  type: string;
  reason: string;
  message: string;
  count: number;
  first_timestamp: string;
  last_timestamp: string;
  namespace: string;
  involved_kind: string;
  involved_name: string;
  source_component: string;
  source_host: string;
}

// 日誌統計型別
export interface LogStats {
  total_count: number;
  error_count: number;
  warn_count: number;
  info_count: number;
  time_distribution?: Array<{ time: string; count: number }>;
  namespace_stats?: Array<{ namespace: string; count: number }>;
  level_stats?: Array<{ level: string; count: number }>;
}

// 日誌流目標
export interface LogStreamTarget {
  namespace: string;
  pod: string;
  container?: string;
}

// 日誌流配置
export interface LogStreamConfig {
  targets: LogStreamTarget[];
  tail_lines?: number;
  since_seconds?: number;
  show_timestamp?: boolean;
  show_source?: boolean;
}

// 日誌查詢參數
export interface LogSearchParams {
  namespaces?: string[];
  pods?: string[];
  containers?: string[];
  levels?: string[];
  keyword?: string;
  regex?: string;
  startTime?: string;
  endTime?: string;
  limit?: number;
  offset?: number;
  direction?: 'forward' | 'backward';
}

// Pod資訊（用於日誌選擇）
export interface LogPodInfo {
  name: string;
  namespace: string;
  status: string;
  containers: string[];
}

// 日誌服務
export const logService = {
  // 獲取容器日誌
  getContainerLogs: (
    clusterId: string,
    params: {
      namespace: string;
      pod: string;
      container?: string;
      tailLines?: number;
      sinceSeconds?: number;
      previous?: boolean;
    }
  ) => {
    const query = new URLSearchParams();
    query.set('namespace', params.namespace);
    query.set('pod', params.pod);
    if (params.container) query.set('container', params.container);
    if (params.tailLines) query.set('tailLines', String(params.tailLines));
    if (params.sinceSeconds) query.set('sinceSeconds', String(params.sinceSeconds));
    if (params.previous) query.set('previous', 'true');

    return request.get<{ logs: string }>(
      `/clusters/${clusterId}/logs/containers?${query.toString()}`
    );
  },

  // 獲取K8s事件日誌
  getEventLogs: (
    clusterId: string,
    params?: {
      namespace?: string;
      resourceType?: string;
      resourceName?: string;
      type?: 'Normal' | 'Warning';
      limit?: number;
    }
  ) => {
    const query = new URLSearchParams();
    if (params?.namespace) query.set('namespace', params.namespace);
    if (params?.resourceType) query.set('resourceType', params.resourceType);
    if (params?.resourceName) query.set('resourceName', params.resourceName);
    if (params?.type) query.set('type', params.type);
    if (params?.limit) query.set('limit', String(params.limit));

    return request.get<{ items: EventLogEntry[]; total: number }>(
      `/clusters/${clusterId}/logs/events?${query.toString()}`
    );
  },

  // 日誌搜尋
  searchLogs: (clusterId: string, params: LogSearchParams) => {
    return request.post<{ items: LogEntry[]; total: number }>(
      `/clusters/${clusterId}/logs/search`,
      params
    );
  },

  // 獲取日誌統計
  getLogStats: (
    clusterId: string,
    params?: { namespace?: string; timeRange?: '1h' | '6h' | '24h' | '7d' }
  ) => {
    const query = new URLSearchParams();
    if (params?.namespace) query.set('namespace', params.namespace);
    if (params?.timeRange) query.set('timeRange', params.timeRange);

    return request.get<LogStats>(
      `/clusters/${clusterId}/logs/stats?${query.toString()}`
    );
  },

  // 獲取命名空間列表
  getNamespaces: (clusterId: string) => {
    return request.get<string[]>(`/clusters/${clusterId}/logs/namespaces`);
  },

  // 獲取Pod列表
  getPods: (clusterId: string, namespace?: string) => {
    const query = namespace ? `?namespace=${namespace}` : '';
    return request.get<LogPodInfo[]>(
      `/clusters/${clusterId}/logs/pods${query}`
    );
  },

  // 匯出日誌
  exportLogs: async (clusterId: string, params: LogSearchParams) => {
    const response = await request.post(
      `/clusters/${clusterId}/logs/export`,
      params,
      { responseType: 'blob' }
    );
    return response;
  },

  // 建立聚合日誌流 WebSocket
  createAggregateLogStream: (
    clusterId: string,
    config: LogStreamConfig
  ): { ws: WebSocket; config: LogStreamConfig } => {
    const token = tokenManager.getToken();
    const url = buildWebSocketUrl(`/ws/clusters/${clusterId}/logs/stream?token=${token}`);
    
    const ws = new WebSocket(url);
    
    // 返回 ws 和 config，讓呼叫者在 onopen 中傳送配置
    return { ws, config };
  },

  // 建立單Pod日誌流 WebSocket
  createSinglePodLogStream: (
    clusterId: string,
    namespace: string,
    podName: string,
    options?: {
      container?: string;
      previous?: boolean;
      tailLines?: number;
      sinceSeconds?: number;
    }
  ): WebSocket => {
    const token = tokenManager.getToken();

    const query = new URLSearchParams();
    query.set('token', token || '');
    if (options?.container) query.set('container', options.container);
    if (options?.previous) query.set('previous', 'true');
    if (options?.tailLines) query.set('tailLines', String(options.tailLines));
    if (options?.sinceSeconds) query.set('sinceSeconds', String(options.sinceSeconds));
    
    const url = buildWebSocketUrl(
      `/ws/clusters/${clusterId}/logs/pod/${namespace}/${podName}?${query.toString()}`
    );
    
    return new WebSocket(url);
  },
};

export default logService;

// ---- External Log Sources (Loki / Elasticsearch) ----

export interface LogSource {
  id: number;
  cluster_id: number;
  type: 'loki' | 'elasticsearch';
  name: string;
  url: string;
  username?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ExternalLogSearchParams {
  query: string;
  index?: string;      // ES only
  startTime?: string;  // RFC3339
  endTime?: string;    // RFC3339
  limit?: number;
}

export const logSourceService = {
  list: (clusterId: string | number) =>
    request.get<LogSource[]>(`/clusters/${clusterId}/log-sources`),

  create: (
    clusterId: string | number,
    data: { type: string; name: string; url: string; username?: string; password?: string; apiKey?: string; enabled: boolean }
  ) => request.post<LogSource>(`/clusters/${clusterId}/log-sources`, data),

  update: (
    clusterId: string | number,
    sourceId: number,
    data: { name?: string; url?: string; username?: string; password?: string; apiKey?: string; enabled?: boolean }
  ) => request.put(`/clusters/${clusterId}/log-sources/${sourceId}`, data),

  delete: (clusterId: string | number, sourceId: number) =>
    request.delete(`/clusters/${clusterId}/log-sources/${sourceId}`),

  search: (clusterId: string | number, sourceId: number, params: ExternalLogSearchParams) =>
    request.post<{ items: LogEntry[]; total: number }>(
      `/clusters/${clusterId}/log-sources/${sourceId}/search`,
      params
    ),
};

