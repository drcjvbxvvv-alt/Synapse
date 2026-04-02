import axios from 'axios';

export interface VPARecommendation {
  containerName: string;
  lowerBound?: { cpu?: string; memory?: string };
  target?: { cpu?: string; memory?: string };
  upperBound?: { cpu?: string; memory?: string };
  uncappedTarget?: { cpu?: string; memory?: string };
}

export interface VPAInfo {
  name: string;
  namespace: string;
  targetKind: string;
  targetName: string;
  updateMode: string;
  recommendations: VPARecommendation[];
  createdAt: string;
}

export interface VPARequest {
  name: string;
  namespace: string;
  targetKind: string;
  targetName: string;
  updateMode?: string;
  minCPU?: string;
  maxCPU?: string;
  minMemory?: string;
  maxMemory?: string;
}

const base = (clusterID: string) => `/api/v1/clusters/${clusterID}/vpa`;

export const vpaService = {
  checkCRD: (clusterID: string) =>
    axios.get<{ installed: boolean }>(`${base(clusterID)}/crd-check`),

  list: (clusterID: string, namespace?: string) =>
    axios.get<{ items: VPAInfo[]; total: number; installed: boolean }>(base(clusterID), {
      params: { namespace },
    }),

  getWorkloadVPA: (clusterID: string, namespace: string, name: string, kind: string) =>
    axios.get<{ vpa: VPAInfo | null; installed: boolean }>(
      `${base(clusterID)}/${namespace}/${name}/workload`,
      { params: { kind } }
    ),

  create: (clusterID: string, data: VPARequest) =>
    axios.post<VPAInfo>(base(clusterID), data),

  update: (clusterID: string, namespace: string, name: string, data: VPARequest) =>
    axios.put<VPAInfo>(`${base(clusterID)}/${namespace}/${name}`, data),

  delete: (clusterID: string, namespace: string, name: string) =>
    axios.delete(`${base(clusterID)}/${namespace}/${name}`),
};
