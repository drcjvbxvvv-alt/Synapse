import { request } from '../utils/api';

export interface MeshStatus {
  installed: boolean;
  version?: string;
  reason?: string;
}

export interface MeshNode {
  id: string;
  name: string;
  namespace: string;
  rps: number;
  errorRate: number;
  p99ms: number;
}

export interface MeshEdge {
  id: string;
  source: string;
  target: string;
  rps: number;
}

export interface MeshTopology {
  nodes: MeshNode[];
  edges: MeshEdge[];
}

export interface VirtualServiceSummary {
  name: string;
  namespace: string;
  createdAt: string;
  spec?: Record<string, unknown>;
}

export interface DestinationRuleSummary {
  name: string;
  namespace: string;
  createdAt: string;
  spec?: Record<string, unknown>;
}

export class MeshService {
  static async getStatus(clusterId: string): Promise<{ data: MeshStatus }> {
    return request.get(`/clusters/${clusterId}/service-mesh/status`);
  }

  static async getTopology(clusterId: string, namespace?: string): Promise<{ data: MeshTopology }> {
    const params = namespace ? `?namespace=${namespace}` : '';
    return request.get(`/clusters/${clusterId}/service-mesh/topology${params}`);
  }

  static async listVirtualServices(
    clusterId: string,
    namespace?: string
  ): Promise<{ data: { items: VirtualServiceSummary[]; total: number; installed: boolean } }> {
    const params = namespace ? `?namespace=${namespace}` : '';
    return request.get(`/clusters/${clusterId}/service-mesh/virtual-services${params}`);
  }

  static async getVirtualService(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<{ data: Record<string, unknown> }> {
    return request.get(`/clusters/${clusterId}/service-mesh/virtual-services/${namespace}/${name}`);
  }

  static async createVirtualService(
    clusterId: string,
    namespace: string,
    body: Record<string, unknown>
  ): Promise<{ data: Record<string, unknown> }> {
    return request.post(`/clusters/${clusterId}/service-mesh/virtual-services?namespace=${namespace}`, body);
  }

  static async updateVirtualService(
    clusterId: string,
    namespace: string,
    name: string,
    body: Record<string, unknown>
  ): Promise<{ data: Record<string, unknown> }> {
    return request.put(
      `/clusters/${clusterId}/service-mesh/virtual-services/${namespace}/${name}`,
      body
    );
  }

  static async deleteVirtualService(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<{ data: null }> {
    return request.delete(
      `/clusters/${clusterId}/service-mesh/virtual-services/${namespace}/${name}`
    );
  }

  static async listDestinationRules(
    clusterId: string,
    namespace?: string
  ): Promise<{ data: { items: DestinationRuleSummary[]; total: number; installed: boolean } }> {
    const params = namespace ? `?namespace=${namespace}` : '';
    return request.get(`/clusters/${clusterId}/service-mesh/destination-rules${params}`);
  }

  static async getDestinationRule(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<{ data: Record<string, unknown> }> {
    return request.get(`/clusters/${clusterId}/service-mesh/destination-rules/${namespace}/${name}`);
  }

  static async createDestinationRule(
    clusterId: string,
    namespace: string,
    body: Record<string, unknown>
  ): Promise<{ data: Record<string, unknown> }> {
    return request.post(
      `/clusters/${clusterId}/service-mesh/destination-rules?namespace=${namespace}`,
      body
    );
  }

  static async updateDestinationRule(
    clusterId: string,
    namespace: string,
    name: string,
    body: Record<string, unknown>
  ): Promise<{ data: Record<string, unknown> }> {
    return request.put(
      `/clusters/${clusterId}/service-mesh/destination-rules/${namespace}/${name}`,
      body
    );
  }

  static async deleteDestinationRule(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<{ data: null }> {
    return request.delete(
      `/clusters/${clusterId}/service-mesh/destination-rules/${namespace}/${name}`
    );
  }
}
