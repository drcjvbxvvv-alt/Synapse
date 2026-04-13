import api from '../utils/api';

export interface PDBInfo {
  name: string;
  namespace: string;
  selector: Record<string, string>;
  minAvailable?: string;
  maxUnavailable?: string;
  currentHealthy: number;
  desiredHealthy: number;
  expectedPods: number;
  disruptionsAllowed: number;
  createdAt: string;
}

export interface PDBRequest {
  name: string;
  namespace: string;
  selector: Record<string, string>;
  minAvailable?: string;
  maxUnavailable?: string;
}

const base = (clusterID: string) => `/clusters/${clusterID}/pdbs`;

export const pdbService = {
  list: (clusterID: string, namespace?: string) =>
    api.get<{ items: PDBInfo[]; total: number }>(base(clusterID), { params: { namespace } }),

  listForNamespace: (clusterID: string, namespace: string) =>
    api.get<{ items: PDBInfo[]; total: number }>(`${base(clusterID)}/${namespace}`),

  create: (clusterID: string, data: PDBRequest) =>
    api.post<PDBInfo>(base(clusterID), data),

  update: (clusterID: string, namespace: string, name: string, data: PDBRequest) =>
    api.put<PDBInfo>(`${base(clusterID)}/${namespace}/${name}`, data),

  delete: (clusterID: string, namespace: string, name: string) =>
    api.delete(`${base(clusterID)}/${namespace}/${name}`),
};
