import { request } from '../utils/api';
import type { ApiResponse, PaginatedResponse } from '../types';

export interface EventAlertRule {
  id: number;
  clusterId: number;
  name: string;
  description?: string;
  namespace?: string;
  eventReason?: string;
  eventType?: string;
  minCount: number;
  notifyType: string;
  notifyUrl?: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface EventAlertHistory {
  id: number;
  ruleId: number;
  clusterId: number;
  ruleName: string;
  namespace: string;
  eventReason: string;
  eventType: string;
  message: string;
  involvedObj: string;
  notifyResult: string;
  triggeredAt: string;
}

export type RuleListResponse = ApiResponse<PaginatedResponse<EventAlertRule>>;
export type HistoryListResponse = ApiResponse<PaginatedResponse<EventAlertHistory>>;

export const EventAlertService = {
  listRules: (clusterId: string, page = 1, pageSize = 20): Promise<RuleListResponse> =>
    request.get(`/clusters/${clusterId}/event-alerts/rules?page=${page}&pageSize=${pageSize}`),

  createRule: (clusterId: string, rule: Partial<EventAlertRule>): Promise<ApiResponse<EventAlertRule>> =>
    request.post(`/clusters/${clusterId}/event-alerts/rules`, rule),

  updateRule: (clusterId: string, ruleId: number, rule: Partial<EventAlertRule>): Promise<ApiResponse<EventAlertRule>> =>
    request.put(`/clusters/${clusterId}/event-alerts/rules/${ruleId}`, rule),

  deleteRule: (clusterId: string, ruleId: number): Promise<ApiResponse<null>> =>
    request.delete(`/clusters/${clusterId}/event-alerts/rules/${ruleId}`),

  toggleRule: (clusterId: string, ruleId: number, enabled: boolean): Promise<ApiResponse<{ enabled: boolean }>> =>
    request.put(`/clusters/${clusterId}/event-alerts/rules/${ruleId}/toggle`, { enabled }),

  listHistory: (clusterId: string, page = 1, pageSize = 20): Promise<HistoryListResponse> =>
    request.get(`/clusters/${clusterId}/event-alerts/history?page=${page}&pageSize=${pageSize}`),
};
