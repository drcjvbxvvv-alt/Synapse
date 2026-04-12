import { request } from '../utils/api';

// Terminal session list item
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

// Session list response
export interface SessionListResponse {
  items: TerminalSessionItem[];
  total: number;
  page: number;
  pageSize: number;
}

// Session detail
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

// Terminal command record
export interface TerminalCommand {
  id: number;
  session_id: number;
  timestamp: string;
  raw_input: string;
  parsed_cmd: string;
  exit_code: number | null;
}

// Command list response
export interface CommandListResponse {
  items: TerminalCommand[];
  total: number;
  page: number;
  pageSize: number;
}

// Session statistics
export interface SessionStats {
  total_sessions: number;
  active_sessions: number;
  total_commands: number;
  kubectl_sessions: number;
  pod_sessions: number;
  node_sessions: number;
}

// Session list query parameters
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

// ==================== Operation audit related types ====================

// Operation log list item
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

// Operation log list response
export interface OperationLogListResponse {
  items: OperationLogItem[];
  total: number;
  page: number;
  pageSize: number;
}

// Operation log detail (includes request body)
export interface OperationLogDetail extends OperationLogItem {
  query: string;
  request_body: string;
  user_agent: string;
}

// Module statistics
export interface ModuleStat {
  module: string;
  module_name: string;
  count: number;
}

// Action statistics
export interface ActionStat {
  action: string;
  action_name: string;
  count: number;
}

// User operation statistics
export interface UserOperationStat {
  user_id: number;
  username: string;
  count: number;
}

// Operation log statistics
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

// Operation log query parameters
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

// Module/action options
export interface ModuleOption {
  key: string;
  name: string;
}

export const auditService = {
  // ==================== Terminal session audit ====================

  // Get terminal session list
  getTerminalSessions: (params?: SessionListParams) => {
    return request.get<SessionListResponse>('/audit/terminal/sessions', { params });
  },

  // Get terminal session detail
  getTerminalSession: (sessionId: number) => {
    return request.get<SessionDetailResponse>(`/audit/terminal/sessions/${sessionId}`);
  },

  // Get terminal command records
  getTerminalCommands: (sessionId: number, params?: { page?: number; pageSize?: number }) => {
    return request.get<CommandListResponse>(`/audit/terminal/sessions/${sessionId}/commands`, { params });
  },

  // Get terminal session statistics
  getTerminalStats: () => {
    return request.get<SessionStats>('/audit/terminal/stats');
  },

  // ==================== Operation audit ====================

  // Get operation log list
  getOperationLogs: (params?: OperationLogListParams) => {
    return request.get<OperationLogListResponse>('/audit/operations', { params });
  },

  // Get operation log detail
  getOperationLog: (id: number) => {
    return request.get<OperationLogDetail>(`/audit/operations/${id}`);
  },

  // Get operation log statistics
  getOperationLogStats: (params?: { startTime?: string; endTime?: string }) => {
    return request.get<OperationLogStats>('/audit/operations/stats', { params });
  },

  // Get modules list
  getModules: () => {
    return request.get<ModuleOption[]>('/audit/modules');
  },

  // Get actions list
  getActions: () => {
    return request.get<ModuleOption[]>('/audit/actions');
  },
};

