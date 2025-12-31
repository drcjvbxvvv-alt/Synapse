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

export const auditService = {
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
};

