import { request } from '../utils/api';

// 终端会话列表项
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

// 会话列表响应
export interface SessionListResponse {
  items: TerminalSessionItem[];
  total: number;
  page: number;
  pageSize: number;
}

// 会话详情
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

// 命令记录
export interface TerminalCommand {
  id: number;
  session_id: number;
  timestamp: string;
  raw_input: string;
  parsed_cmd: string;
  exit_code: number | null;
}

// 命令列表响应
export interface CommandListResponse {
  items: TerminalCommand[];
  total: number;
  page: number;
  pageSize: number;
}

// 会话统计
export interface SessionStats {
  total_sessions: number;
  active_sessions: number;
  total_commands: number;
  kubectl_sessions: number;
  pod_sessions: number;
  node_sessions: number;
}

// 会话列表查询参数
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

// ==================== 操作审计相关类型 ====================

// 操作日志列表项
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

// 操作日志列表响应
export interface OperationLogListResponse {
  items: OperationLogItem[];
  total: number;
  page: number;
  pageSize: number;
}

// 操作日志详情（含请求体）
export interface OperationLogDetail extends OperationLogItem {
  query: string;
  request_body: string;
  user_agent: string;
}

// 模块统计
export interface ModuleStat {
  module: string;
  module_name: string;
  count: number;
}

// 操作统计
export interface ActionStat {
  action: string;
  action_name: string;
  count: number;
}

// 用户操作统计
export interface UserOperationStat {
  user_id: number;
  username: string;
  count: number;
}

// 操作日志统计
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

// 操作日志查询参数
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

// 模块/操作选项
export interface ModuleOption {
  key: string;
  name: string;
}

export const auditService = {
  // ==================== 终端会话审计 ====================
  
  // 获取终端会话列表
  getTerminalSessions: (params?: SessionListParams) => {
    return request.get<SessionListResponse>('/audit/terminal/sessions', { params });
  },

  // 获取终端会话详情
  getTerminalSession: (sessionId: number) => {
    return request.get<SessionDetailResponse>(`/audit/terminal/sessions/${sessionId}`);
  },

  // 获取终端命令记录
  getTerminalCommands: (sessionId: number, params?: { page?: number; pageSize?: number }) => {
    return request.get<CommandListResponse>(`/audit/terminal/sessions/${sessionId}/commands`, { params });
  },

  // 获取终端会话统计
  getTerminalStats: () => {
    return request.get<SessionStats>('/audit/terminal/stats');
  },

  // ==================== 操作审计 ====================

  // 获取操作日志列表
  getOperationLogs: (params?: OperationLogListParams) => {
    return request.get<OperationLogListResponse>('/audit/operations', { params });
  },

  // 获取操作日志详情
  getOperationLog: (id: number) => {
    return request.get<OperationLogDetail>(`/audit/operations/${id}`);
  },

  // 获取操作日志统计
  getOperationLogStats: (params?: { startTime?: string; endTime?: string }) => {
    return request.get<OperationLogStats>('/audit/operations/stats', { params });
  },

  // 获取模块列表
  getModules: () => {
    return request.get<ModuleOption[]>('/audit/modules');
  },

  // 获取操作列表
  getActions: () => {
    return request.get<ModuleOption[]>('/audit/actions');
  },
};

