import { request } from '../utils/api';

export interface NamespaceData {
  name: string;
  status: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  creationTimestamp: string;
  resourceQuota?: {
    hard: Record<string, string>;
    used: Record<string, string>;
  };
}

export interface NamespaceDetailData extends NamespaceData {
  resourceCount: {
    pods: number;
    services: number;
    configMaps: number;
    secrets: number;
  };
}

export interface CreateNamespaceRequest {
  name: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export interface NamespaceListResponse {
  items: NamespaceData[];
  meta: {
    hasAllAccess: boolean;
    allowedNamespaces: string[];
  };
}

/**
 * 獲取命名空間列表
 */
export const getNamespaces = async (clusterId: number): Promise<NamespaceData[]> => {
  const res = await request.get<NamespaceListResponse>(`/clusters/${clusterId}/namespaces`);
  return res.items || [];
};

/**
 * 獲取命名空間詳情
 */
export const getNamespaceDetail = async (
  clusterId: number,
  namespace: string
): Promise<NamespaceDetailData> => {
  return request.get<NamespaceDetailData>(`/clusters/${clusterId}/namespaces/${namespace}`);
};

/**
 * 建立命名空間
 */
export const createNamespace = async (
  clusterId: number,
  data: CreateNamespaceRequest
): Promise<NamespaceData> => {
  return request.post<NamespaceData>(`/clusters/${clusterId}/namespaces`, data);
};

/**
 * 刪除命名空間
 */
export const deleteNamespace = async (
  clusterId: number,
  namespace: string
): Promise<void> => {
  await request.delete<void>(`/clusters/${clusterId}/namespaces/${namespace}`);
};

// ─── ResourceQuota API ────────────────────────────────────────────────────────
export const listResourceQuotas = (clusterId: string, namespace: string) =>
  request.get<{ items: any[]; total: number }>(`/clusters/${clusterId}/namespaces/${namespace}/quotas`);

export const createResourceQuota = (clusterId: string, namespace: string, data: { name: string; hard: Record<string, string> }) =>
  request.post(`/clusters/${clusterId}/namespaces/${namespace}/quotas`, data);

export const updateResourceQuota = (clusterId: string, namespace: string, name: string, data: { hard: Record<string, string> }) =>
  request.put(`/clusters/${clusterId}/namespaces/${namespace}/quotas/${name}`, data);

export const deleteResourceQuota = (clusterId: string, namespace: string, name: string) =>
  request.delete(`/clusters/${clusterId}/namespaces/${namespace}/quotas/${name}`);

// ─── LimitRange API ───────────────────────────────────────────────────────────
export const listLimitRanges = (clusterId: string, namespace: string) =>
  request.get<{ items: any[]; total: number }>(`/clusters/${clusterId}/namespaces/${namespace}/limitranges`);

export const createLimitRange = (clusterId: string, namespace: string, data: { name: string; limits: any[] }) =>
  request.post(`/clusters/${clusterId}/namespaces/${namespace}/limitranges`, data);

export const updateLimitRange = (clusterId: string, namespace: string, name: string, data: { limits: any[] }) =>
  request.put(`/clusters/${clusterId}/namespaces/${namespace}/limitranges/${name}`, data);

export const deleteLimitRange = (clusterId: string, namespace: string, name: string) =>
  request.delete(`/clusters/${clusterId}/namespaces/${namespace}/limitranges/${name}`);

/**
 * 命名空間服務物件 - 相容舊的呼叫方式
 */
export const namespaceService = {
  getNamespaces: async (clusterId: string) => {
    const res = await request.get<NamespaceListResponse>(`/clusters/${clusterId}/namespaces`);
    return res.items || [];
  },
  getNamespaceDetail,
  createNamespace,
  deleteNamespace,
};

