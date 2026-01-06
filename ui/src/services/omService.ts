import { request } from '../utils/api';

// 健康诊断响应
export interface HealthDiagnosisResponse {
  health_score: number;
  status: 'healthy' | 'warning' | 'critical';
  risk_items: RiskItem[];
  suggestions: string[];
  diagnosis_time: number;
  category_scores: Record<string, number>;
}

// 风险项
export interface RiskItem {
  id: string;
  category: 'node' | 'workload' | 'resource' | 'storage' | 'control_plane';
  severity: 'critical' | 'warning' | 'info';
  title: string;
  description: string;
  resource: string;
  namespace?: string;
  solution: string;
}

// 资源 Top N 请求
export interface ResourceTopRequest {
  type: 'cpu' | 'memory' | 'disk' | 'network';
  level: 'namespace' | 'workload' | 'pod';
  limit?: number;
}

// 资源 Top N 响应
export interface ResourceTopResponse {
  type: string;
  level: string;
  items: ResourceTopItem[];
  query_time: number;
}

// 资源 Top 项
export interface ResourceTopItem {
  rank: number;
  name: string;
  namespace?: string;
  usage: number;
  usage_rate: number;
  request?: number;
  limit?: number;
  unit: string;
}

// 控制面状态响应
export interface ControlPlaneStatusResponse {
  overall: 'healthy' | 'degraded' | 'unhealthy';
  components: ControlPlaneComponent[];
  check_time: number;
}

// 控制面组件
export interface ControlPlaneComponent {
  name: string;
  type: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  message: string;
  last_check_time: number;
  metrics?: ComponentMetrics;
  instances?: ComponentInstance[];
}

// 组件指标
export interface ComponentMetrics {
  request_rate?: number;
  error_rate?: number;
  latency?: number;
  queue_length?: number;
  leader_status?: boolean;
  db_size?: number;
  member_count?: number;
}

// 组件实例
export interface ComponentInstance {
  name: string;
  node: string;
  status: string;
  ip: string;
  start_time: number;
}

export const omService = {
  // 获取集群健康诊断
  getHealthDiagnosis: (clusterId: string) => {
    return request.get<HealthDiagnosisResponse>(`/clusters/${clusterId}/om/health-diagnosis`);
  },

  // 获取资源消耗 Top N
  getResourceTop: (clusterId: string, params: ResourceTopRequest) => {
    return request.get<ResourceTopResponse>(`/clusters/${clusterId}/om/resource-top`, { params });
  },

  // 获取控制面组件状态
  getControlPlaneStatus: (clusterId: string) => {
    return request.get<ControlPlaneStatusResponse>(`/clusters/${clusterId}/om/control-plane-status`);
  },
};
