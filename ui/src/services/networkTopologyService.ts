import { request } from '../utils/api';

export interface NetworkNode {
  id: string;
  kind: 'Workload' | 'Service';
  name: string;
  namespace: string;
  workloadKind?: string; // Deployment | StatefulSet | DaemonSet | Job | Pod
  labels?: Record<string, string>;
  readyCount: number;
  totalCount: number;
  clusterIP?: string;
  serviceType?: string;
}

export interface NetworkEdge {
  source: string;
  target: string;
  health: 'healthy' | 'degraded' | 'down' | 'unknown';
  ports?: string;
}

export interface ClusterNetworkTopology {
  nodes: NetworkNode[];
  edges: NetworkEdge[];
}

export const networkTopologyService = {
  getTopology: (
    clusterId: string,
    namespaces?: string[],
  ): Promise<ClusterNetworkTopology> =>
    request.get(`/clusters/${clusterId}/network/topology`, {
      params: namespaces?.length ? { namespaces: namespaces.join(',') } : undefined,
    }),
};
