import { request } from '../utils/api';

// 終端會話列表項
export interface TerminalSessionItem {
  id: number;
  user_id: number;
  username: string;
  display_name: string;
  cluster_id: number;
  cluster_name: string;
  target_type: 'kubectl' | 'pod' | 'node';
  target_ref: string;
  namespace: string;
  pod: string;
  container: string;
  node: string;
  start_at: string;
  end_at: string | null;
  input_size: number;
  status: 'active' | 'closed' | 'error';
  command_count: number;
}

// 會話列表響應
export interface SessionListResponse {
  items: TerminalSessionItem[];
  total: number;
  page: number;
  pageSize: number;
}

// 會話詳情
export interface SessionDetailResponse {
  id: number;
  user_id: number;
  username: string;
  display_name: string;
  cluster_id: number;
  cluster_name: string;
  target_type: string;
  target_ref: string;
  namespace: string;
  pod: string;
  container: string;
  node: string;
  start_at: string;
  end_at: string | null;
  input_size: number;
  status: string;
  command_count: number;
  duration: string;
}

// 命令記錄
export interface TerminalCommand {
  id: number;
  session_id: number;
  timestamp: string;
  raw_input: string;
  parsed_cmd: string;
  exit_code: number | null;
}

// 命令列表響應
export interface CommandListResponse {
  items: TerminalCommand[];
  total: number;
  page: number;
  pageSize: number;
}

// 會話統計
export interface SessionStats {
  total_sessions: number;
  active_sessions: number;
  total_commands: number;
  kubectl_sessions: number;
  pod_sessions: number;
  node_sessions: number;
}

// 會話列表查詢參數
export interface SessionListParams {
  page?: number;
  pageSize?: number;
  userId?: number;
  clusterId?: number;
  targetType?: 'kubectl' | 'pod' | 'node';
  status?: 'active' | 'closed' | 'error';
  startTime?: string;
  endTime?: string;
  keyword?: string;
}

// ==================== 操作審計相關型別 ====================

// 操作日誌列表項
export interface OperationLogItem {
  id: number;
  user_id: number | null;
  username: string;
  method: string;
  path: string;
  module: string;
  module_name: string;
  action: string;
  action_name: string;
  cluster_id: number | null;
  cluster_name: string;
  namespace: string;
  resource_type: string;
  resource_name: string;
  status_code: number;
  success: boolean;
  error_message: string;
  client_ip: string;
  duration: number;
  created_at: string;
}

// 操作日誌列表響應
export interface OperationLogListResponse {
  items: OperationLogItem[];
  total: number;
  page: number;
  pageSize: number;
}

// 操作日誌詳情（含請求體）
export interface OperationLogDetail extends OperationLogItem {
  query: string;
  request_body: string;
  user_agent: string;
}

// 模組統計
export interface ModuleStat {
  module: string;
  module_name: string;
  count: number;
}

// 操作統計
export interface ActionStat {
  action: string;
  action_name: string;
  count: number;
}

// 使用者操作統計
export interface UserOperationStat {
  user_id: number;
  username: string;
  count: number;
}

// 操作日誌統計
export interface OperationLogStats {
  total_count: number;
  today_count: number;
  success_count: number;
  failed_count: number;
  module_stats: ModuleStat[];
  action_stats: ActionStat[];
  recent_failures: OperationLogItem[];
  user_stats: UserOperationStat[];
}

// 操作日誌查詢參數
export interface OperationLogListParams {
  page?: number;
  pageSize?: number;
  userId?: number;
  username?: string;
  module?: string;
  action?: string;
  resourceType?: string;
  clusterId?: number;
  success?: boolean;
  startTime?: string;
  endTime?: string;
  keyword?: string;
}

// 模組/操作選項
export interface ModuleOption {
  key: string;
  name: string;
}

export const auditService = {
  // ==================== 終端會話審計 ====================
  
  // 獲取終端會話列表
  getTerminalSessions: (params?: SessionListParams) => {
    return request.get<SessionListResponse>('/audit/terminal/sessions', { params });
  },

  // 獲取終端會話詳情
  getTerminalSession: (sessionId: number) => {
    return request.get<SessionDetailResponse>(`/audit/terminal/sessions/${sessionId}`);
  },

  // 獲取終端命令記錄
  getTerminalCommands: (sessionId: number, params?: { page?: number; pageSize?: number }) => {
    return request.get<CommandListResponse>(`/audit/terminal/sessions/${sessionId}/commands`, { params });
  },

  // 獲取終端會話統計
  getTerminalStats: () => {
    return request.get<SessionStats>('/audit/terminal/stats');
  },

  // ==================== 操作審計 ====================

  // 獲取操作日誌列表
  getOperationLogs: (params?: OperationLogListParams) => {
    return request.get<OperationLogListResponse>('/audit/operations', { params });
  },

  // 獲取操作日誌詳情
  getOperationLog: (id: number) => {
    return request.get<OperationLogDetail>(`/audit/operations/${id}`);
  },

  // 獲取操作日誌統計
  getOperationLogStats: (params?: { startTime?: string; endTime?: string }) => {
    return request.get<OperationLogStats>('/audit/operations/stats', { params });
  },

  // 獲取模組列表
  getModules: () => {
    return request.get<ModuleOption[]>('/audit/modules');
  },

  // 獲取操作列表
  getActions: () => {
    return request.get<ModuleOption[]>('/audit/actions');
  },
};

