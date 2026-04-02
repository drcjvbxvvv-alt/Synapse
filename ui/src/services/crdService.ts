import { request } from '../utils/api';

export interface CRDInfo {
  name: string;      // e.g. "certificates.cert-manager.io"
  group: string;     // e.g. "cert-manager.io"
  version: string;   // e.g. "v1"
  kind: string;      // e.g. "Certificate"
  plural: string;    // e.g. "certificates"
  namespaced: boolean;
}

export interface CRDResourceItem {
  name: string;
  namespace?: string;
  uid: string;
  created: string;
  labels?: Record<string, string>;
  status?: unknown;
  spec?: Record<string, unknown>;
}

export interface ListResponse<T> {
  items: T[];
  total: number;
}

export const crdService = {
  /** 自動發現叢集中所有 CRD */
  listCRDs: (clusterId: string) =>
    request.get<ListResponse<CRDInfo>>(`/clusters/${clusterId}/crds`),

  /** 列出特定 CRD 的所有資源實例 */
  listCRDResources: (
    clusterId: string,
    params: { group: string; version: string; plural: string; namespace?: string }
  ) =>
    request.get<ListResponse<CRDResourceItem>>(`/clusters/${clusterId}/crds/resources`, {
      params,
    }),

  /** 取得單個 CRD 資源實例的完整物件 */
  getCRDResource: (
    clusterId: string,
    namespace: string,
    name: string,
    params: { group: string; version: string; plural: string }
  ) =>
    request.get<Record<string, unknown>>(
      `/clusters/${clusterId}/crds/resources/${namespace}/${name}`,
      { params }
    ),

  /** 刪除單個 CRD 資源實例 */
  deleteCRDResource: (
    clusterId: string,
    namespace: string,
    name: string,
    params: { group: string; version: string; plural: string }
  ) =>
    request.delete(`/clusters/${clusterId}/crds/resources/${namespace}/${name}`, { params }),
};
