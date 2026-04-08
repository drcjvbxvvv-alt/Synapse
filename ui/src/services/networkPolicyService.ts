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

// ---- Topology ----
export interface TopologyNode {
  id: string;
  type: 'podgroup' | 'namespace' | 'ipblock' | 'external';
  label: string;
  namespace?: string;
  selector?: Record<string, string>;
  policyCount?: number;
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
  label?: string;
  direction: 'ingress' | 'egress';
  policy: string;
  namespace: string;
}

export interface TopologyResponse {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

// ---- Conflicts ----
export interface ConflictItem {
  policyA: string;
  policyB: string;
  namespace: string;
  reason: string;
  selectorA: Record<string, string>;
  selectorB: Record<string, string>;
}

// ---- Wizard ----
export interface WizardPort {
  protocol: string;
  port: string;
}

export interface WizardIngressRule {
  ports?: WizardPort[];
  fromType: 'pod' | 'namespace' | 'ipblock' | 'all';
  selector?: Record<string, string>;
  cidr?: string;
}

export interface WizardEgressRule {
  ports?: WizardPort[];
  toType: 'pod' | 'namespace' | 'ipblock' | 'all';
  selector?: Record<string, string>;
  cidr?: string;
}

export interface WizardValidateRequest {
  step: number;
  namespace: string;
  name: string;
  selector: Record<string, string>;
  policyTypes: string[];
  ingress?: WizardIngressRule[];
  egress?: WizardEgressRule[];
}

export interface WizardValidateResponse {
  valid: boolean;
  message?: string;
  yaml?: string;
}

export interface SimulateRequest {
  namespace: string;
  fromPodLabels: Record<string, string>;
  toPodLabels: Record<string, string>;
  port: number;
  protocol: string;
}

export interface SimulateResult {
  allowed: boolean;
  reason: string;
  matchedPolicies: string[];
}

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

  static async create(clusterId: string, namespace: string, yamlContent: string, dryRun = false): Promise<ApiResponse<NetworkPolicyInfo>> {
    return request.post(`/clusters/${clusterId}/networkpolicies`, { namespace, yaml: yamlContent, dryRun });
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

  static async getTopology(clusterId: string, namespace?: string): Promise<ApiResponse<TopologyResponse>> {
    const params = namespace && namespace !== '_all_' ? `?namespace=${namespace}` : '';
    return request.get(`/clusters/${clusterId}/networkpolicies/topology${params}`);
  }

  static async getConflicts(clusterId: string, namespace?: string): Promise<ApiResponse<{ conflicts: ConflictItem[]; total: number }>> {
    const params = namespace && namespace !== '_all_' ? `?namespace=${namespace}` : '';
    return request.get(`/clusters/${clusterId}/networkpolicies/conflicts${params}`);
  }

  static async wizardValidate(clusterId: string, req: WizardValidateRequest): Promise<ApiResponse<WizardValidateResponse>> {
    return request.post(`/clusters/${clusterId}/networkpolicies/wizard-validate`, req);
  }

  static async simulate(clusterId: string, req: SimulateRequest): Promise<ApiResponse<SimulateResult>> {
    return request.post(`/clusters/${clusterId}/networkpolicies/simulate`, req);
  }
}
