import axios from 'axios';

const base = (clusterId: string) => `/clusters/${clusterId}`;

// ─── KEDA ──────────────────────────────────────────────────────────────────

export interface KEDAStatus {
  installed: boolean;
}

export interface ScaledObjectTrigger {
  type: string;
  metadata?: Record<string, string>;
}

export interface ScaledObjectInfo {
  name: string;
  namespace: string;
  targetName: string;
  targetKind: string;
  minReplicas: number;
  maxReplicas: number;
  currentReplicas: number;
  desiredReplicas: number;
  triggers: ScaledObjectTrigger[];
  createdAt: string;
}

export interface ScaledJobInfo {
  name: string;
  namespace: string;
  triggers: ScaledObjectTrigger[];
  ready: boolean;
  createdAt: string;
}

// ─── Karpenter ─────────────────────────────────────────────────────────────

export interface KarpenterStatus {
  installed: boolean;
}

export interface NodePoolInfo {
  name: string;
  limits?: Record<string, string>;
  consolidationPolicy?: string;
  resources?: Record<string, string>;
  createdAt: string;
}

export interface NodeClaimInfo {
  name: string;
  nodePool: string;
  nodeName: string;
  instanceType?: unknown;
  conditions?: unknown[];
  createdAt: string;
}

// ─── CAS ───────────────────────────────────────────────────────────────────

export interface CASStatus {
  installed: boolean;
  status: string;
  nodeGroupCount: number;
}

// ─── API calls ─────────────────────────────────────────────────────────────

export const autoscalingService = {
  // KEDA
  checkKEDA: (clusterId: string) =>
    axios.get<KEDAStatus>(`${base(clusterId)}/keda/status`),

  listScaledObjects: (clusterId: string, namespace?: string) =>
    axios.get<{ items: ScaledObjectInfo[]; total: number }>(
      `${base(clusterId)}/keda/scaled-objects`,
      { params: namespace ? { namespace } : {} },
    ),

  listScaledJobs: (clusterId: string, namespace?: string) =>
    axios.get<{ items: ScaledJobInfo[]; total: number }>(
      `${base(clusterId)}/keda/scaled-jobs`,
      { params: namespace ? { namespace } : {} },
    ),

  // Karpenter
  checkKarpenter: (clusterId: string) =>
    axios.get<KarpenterStatus>(`${base(clusterId)}/karpenter/status`),

  listNodePools: (clusterId: string) =>
    axios.get<{ items: NodePoolInfo[]; total: number }>(
      `${base(clusterId)}/karpenter/node-pools`,
    ),

  listNodeClaims: (clusterId: string) =>
    axios.get<{ items: NodeClaimInfo[]; total: number }>(
      `${base(clusterId)}/karpenter/node-claims`,
    ),

  // CAS
  getCASStatus: (clusterId: string) =>
    axios.get<CASStatus>(`${base(clusterId)}/cas/status`),
};
