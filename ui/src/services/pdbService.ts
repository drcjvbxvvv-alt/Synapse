import axios from 'axios';

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

const base = (clusterID: string) => `/api/v1/clusters/${clusterID}/pdbs`;

export const pdbService = {
  list: (clusterID: string, namespace?: string) =>
    axios.get<{ items: PDBInfo[]; total: number }>(base(clusterID), { params: { namespace } }),

  listForNamespace: (clusterID: string, namespace: string) =>
    axios.get<{ items: PDBInfo[]; total: number }>(`${base(clusterID)}/${namespace}`),

  create: (clusterID: string, data: PDBRequest) =>
    axios.post<PDBInfo>(base(clusterID), data),

  update: (clusterID: string, namespace: string, name: string, data: PDBRequest) =>
    axios.put<PDBInfo>(`${base(clusterID)}/${namespace}/${name}`, data),

  delete: (clusterID: string, namespace: string, name: string) =>
    axios.delete(`${base(clusterID)}/${namespace}/${name}`),
};
