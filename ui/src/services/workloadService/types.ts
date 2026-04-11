// ─── WorkloadService types ─────────────────────────────────────────────────

export interface WorkloadInfo {
  id: string;
  name: string;
  namespace: string;
  type: string;
  status: string;
  ready?: string;
  upToDate?: number;
  available?: number;
  age?: string;
  images: string[];
  selector: Record<string, string>;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  createdAt: string;
  creationTimestamp?: string;
  replicas?: number;
  readyReplicas?: number;
  updatedReplicas?: number;
  availableReplicas?: number;
  strategy?: string;
  cpuLimit?: string;
  cpuRequest?: string;
  memoryLimit?: string;
  memoryRequest?: string;
  conditions?: Array<{
    type: string;
    status: string;
    lastUpdateTime: string;
    lastTransitionTime: string;
    reason: string;
    message: string;
  }>;
}

export interface WorkloadListResponse {
  items: WorkloadInfo[];
  total: number;
  page: number;
  pageSize: number;
}

export interface WorkloadDetailResponse {
  workload: WorkloadInfo;
  raw: Record<string, unknown>;
  yaml?: string;
  pods?: Array<Record<string, unknown>>;
}

export interface ScaleWorkloadRequest {
  replicas: number;
}

export interface YAMLApplyRequest {
  yaml: string;
  dryRun?: boolean;
}
