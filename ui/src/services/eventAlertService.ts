import { request } from '../utils/api';
import type { PaginatedResponse } from '../types';

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

export const EventAlertService = {
  listRules: (clusterId: string, page = 1, pageSize = 20): Promise<PaginatedResponse<EventAlertRule>> =>
    request.get(`/clusters/${clusterId}/event-alerts/rules?page=${page}&pageSize=${pageSize}`),

  createRule: (clusterId: string, rule: Partial<EventAlertRule>): Promise<EventAlertRule> =>
    request.post(`/clusters/${clusterId}/event-alerts/rules`, rule),

  updateRule: (clusterId: string, ruleId: number, rule: Partial<EventAlertRule>): Promise<EventAlertRule> =>
    request.put(`/clusters/${clusterId}/event-alerts/rules/${ruleId}`, rule),

  deleteRule: (clusterId: string, ruleId: number): Promise<null> =>
    request.delete(`/clusters/${clusterId}/event-alerts/rules/${ruleId}`),

  toggleRule: (clusterId: string, ruleId: number, enabled: boolean): Promise<{ enabled: boolean }> =>
    request.put(`/clusters/${clusterId}/event-alerts/rules/${ruleId}/toggle`, { enabled }),

  listHistory: (clusterId: string, page = 1, pageSize = 20): Promise<PaginatedResponse<EventAlertHistory>> =>
    request.get(`/clusters/${clusterId}/event-alerts/history?page=${page}&pageSize=${pageSize}`),
};
