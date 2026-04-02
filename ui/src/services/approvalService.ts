import axios from 'axios';

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
    axios.get<{ items: ApprovalRequest[]; total: number }>('/api/v1/approvals', { params }),

  // 待審批數量（導航 badge）
  getPendingCount: () =>
    axios.get<{ count: number }>('/api/v1/approvals/pending-count'),

  // 核准
  approve: (id: number, reason?: string) =>
    axios.put(`/api/v1/approvals/${id}/approve`, { reason }),

  // 拒絕
  reject: (id: number, reason: string) =>
    axios.put(`/api/v1/approvals/${id}/reject`, { reason }),

  // 建立審批請求（在 protected namespace 執行操作時）
  createRequest: (clusterID: number, data: {
    namespace: string;
    resourceKind: string;
    resourceName: string;
    action: string;
    payload?: string;
    expiresInHours?: number;
  }) =>
    axios.post(`/api/v1/clusters/${clusterID}/approvals`, data),

  // 命名空間保護設定
  getProtections: (clusterID: number) =>
    axios.get<{ items: Array<{ namespace: string; requireApproval: boolean; description: string }> }>(
      `/api/v1/clusters/${clusterID}/namespace-protections`
    ),

  getProtectionStatus: (clusterID: number, namespace: string) =>
    axios.get<NamespaceProtection>(
      `/api/v1/clusters/${clusterID}/namespace-protections/${namespace}`
    ),

  setProtection: (clusterID: number, namespace: string, data: NamespaceProtection) =>
    axios.put(`/api/v1/clusters/${clusterID}/namespace-protections/${namespace}`, data),
};
