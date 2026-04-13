import api from '../utils/api';

export interface ApprovalRequest {
  id: number;
  clusterID: number;
  clusterName: string;
  namespace: string;
  resourceKind: string;
  resourceName: string;
  action: string;
  requesterID: number;
  requesterName: string;
  approverID?: number;
  approverName?: string;
  status: 'pending' | 'approved' | 'rejected' | 'expired';
  payload?: string;
  reason?: string;
  expiresAt: string;
  approvedAt?: string;
  createdAt: string;
}

export interface NamespaceProtection {
  requireApproval: boolean;
  description: string;
}

export const approvalService = {
  // 全域審批列表
  listApprovals: (params?: { status?: string; clusterID?: number }) =>
    api.get<{ items: ApprovalRequest[]; total: number }>('/approvals', { params }),

  // 待審批數量（導航 badge）
  getPendingCount: () =>
    api.get<{ count: number }>('/approvals/pending-count'),

  // 核准
  approve: (id: number, reason?: string) =>
    api.put(`/approvals/${id}/approve`, { reason }),

  // 拒絕
  reject: (id: number, reason: string) =>
    api.put(`/approvals/${id}/reject`, { reason }),

  // 建立審批請求（在 protected namespace 執行操作時）
  createRequest: (clusterID: number, data: {
    namespace: string;
    resourceKind: string;
    resourceName: string;
    action: string;
    payload?: string;
    expiresInHours?: number;
  }) =>
    api.post(`/clusters/${clusterID}/approvals`, data),

  // 命名空間保護設定
  getProtections: (clusterID: number) =>
    api.get<{ items: Array<{ namespace: string; requireApproval: boolean; description: string }> }>(
      `/clusters/${clusterID}/namespace-protections`
    ),

  getProtectionStatus: (clusterID: number, namespace: string) =>
    api.get<NamespaceProtection>(
      `/clusters/${clusterID}/namespace-protections/${namespace}`
    ),

  setProtection: (clusterID: number, namespace: string, data: NamespaceProtection) =>
    api.put(`/clusters/${clusterID}/namespace-protections/${namespace}`, data),
};
