import { request } from '../utils/api';
import type { ApiResponse, PaginatedResponse } from '../types';

export interface NetworkPolicyInfo {
  name: string;
  namespace: string;
  podSelector: Record<string, string>;
  policyTypes: string[];
  ingressRules: number;
  egressRules: number;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
}

export interface NetworkPolicyPeer {
  podSelector?: { matchLabels?: Record<string, string> };
  namespaceSelector?: { matchLabels?: Record<string, string> };
  ipBlock?: { cidr: string; except?: string[] };
}

export interface NetworkPolicyPort {
  protocol?: string;
  port?: string;
  endPort?: number;
}

export interface NetworkPolicyDetail extends NetworkPolicyInfo {
  ingress?: Array<{ ports?: NetworkPolicyPort[]; from?: NetworkPolicyPeer[] }>;
  egress?: Array<{ ports?: NetworkPolicyPort[]; to?: NetworkPolicyPeer[] }>;
}

export type NetworkPolicyListResponse = ApiResponse<PaginatedResponse<NetworkPolicyInfo>>;
export type NetworkPolicyDetailResponse = ApiResponse<NetworkPolicyDetail>;
export type NetworkPolicyYAMLResponse = ApiResponse<{ yaml: string }>;

export class NetworkPolicyService {
  static async list(
    clusterId: string,
    namespace?: string,
    search?: string,
    page = 1,
    pageSize = 20
  ): Promise<NetworkPolicyListResponse> {
    const params = new URLSearchParams({ page: page.toString(), pageSize: pageSize.toString() });
    if (namespace && namespace !== '_all_') params.append('namespace', namespace);
    if (search) params.append('search', search);
    return request.get(`/clusters/${clusterId}/networkpolicies?${params}`);
  }

  static async get(clusterId: string, namespace: string, name: string): Promise<NetworkPolicyDetailResponse> {
    return request.get(`/clusters/${clusterId}/networkpolicies/${namespace}/${name}`);
  }

  static async getYAML(clusterId: string, namespace: string, name: string): Promise<NetworkPolicyYAMLResponse> {
    return request.get(`/clusters/${clusterId}/networkpolicies/${namespace}/${name}/yaml`);
  }

  static async create(clusterId: string, namespace: string, yamlContent: string): Promise<ApiResponse<NetworkPolicyInfo>> {
    return request.post(`/clusters/${clusterId}/networkpolicies`, { namespace, yaml: yamlContent });
  }

  static async update(
    clusterId: string,
    namespace: string,
    name: string,
    yamlContent: string
  ): Promise<ApiResponse<NetworkPolicyInfo>> {
    return request.put(`/clusters/${clusterId}/networkpolicies/${namespace}/${name}`, { yaml: yamlContent });
  }

  static async delete(clusterId: string, namespace: string, name: string): Promise<ApiResponse<null>> {
    return request.delete(`/clusters/${clusterId}/networkpolicies/${namespace}/${name}`);
  }
}
