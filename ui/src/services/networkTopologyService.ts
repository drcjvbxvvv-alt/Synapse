import { request } from '../utils/api';

export interface NetworkNode {
  id: string;
  kind: 'Workload' | 'Service' | 'Ingress';
  name: string;
  namespace: string;
  workloadKind?: string;    // Deployment | StatefulSet | DaemonSet | Job | Pod
  labels?: Record<string, string>;
  readyCount: number;
  totalCount: number;
  clusterIP?: string;
  serviceType?: string;
  ingressClass?: string;    // Phase C: nginx | traefik | istio …
  hosts?: string[];          // Phase C: Ingress hostnames
  hasHPA?: boolean;          // §7.3: HPA attached
  hpaMin?: number;           // §7.3: minReplicas
  hpaMax?: number;           // §7.3: maxReplicas
}

export interface NetworkEdge {
  source: string;
  target: string;
  kind?: string;         // "" (static) | "ingress" | "istio-flow"
  health: 'healthy' | 'degraded' | 'down' | 'unknown';
  ports?: string;
  // Phase B: Istio enrichment
  requestRate?: number;  // req/s
  errorRate?: number;    // 0.0–1.0
  latencyP99ms?: number; // ms
  // Phase E: NetworkPolicy overlay
  policyStatus?: 'policy-allow' | 'policy-deny' | 'policy-restricted';
  policyName?: string;
  // Phase F: Cilium Hubble real flow data
  hubbleFlowRate?: number;   // forwarded flows/s
  hubbleDropRate?: number;   // 0.0-1.0
  hubbleDropReason?: string; // e.g. "POLICY_DENIED"
}

export interface ClusterNetworkTopology {
  nodes: NetworkNode[];
  edges: NetworkEdge[];
}

export interface TopologyIntegrationStatus {
  cilium: boolean;
  ciliumVersion?: string;
  hubbleMetrics: boolean; // Prometheus 確實有 Hubble 指標（hubble-relay 存在不等於有指標）
  istio: boolean;
  istioVersion?: string;
}

export const networkTopologyService = {
  getTopology: (
    clusterId: string,
    namespaces?: string[],
    enrich?: boolean,
    policy?: boolean,
    hubble?: boolean,
  ): Promise<ClusterNetworkTopology> =>
    request.get(`/clusters/${clusterId}/network/topology`, {
      params: {
        ...(namespaces?.length ? { namespaces: namespaces.join(',') } : {}),
        ...(enrich  ? { enrich:  'true' } : {}),
        ...(policy  ? { policy:  'true' } : {}),
        ...(hubble  ? { hubble:  'true' } : {}),
      },
    }),

  getIntegrations: (clusterId: string): Promise<TopologyIntegrationStatus> =>
    request.get(`/clusters/${clusterId}/network/integrations`),
};
