import { request } from '../utils/api';

// 健康診斷響應
export interface HealthDiagnosisResponse {
  health_score: number;
  status: 'healthy' | 'warning' | 'critical';
  risk_items: RiskItem[];
  suggestions: string[];
  diagnosis_time: number;
  category_scores: Record<string, number>;
}

// 風險項
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

// 資源 Top N 請求
export interface ResourceTopRequest {
  type: 'cpu' | 'memory' | 'disk' | 'network';
  level: 'namespace' | 'workload' | 'pod';
  limit?: number;
}

// 資源 Top N 響應
export interface ResourceTopResponse {
  type: string;
  level: string;
  items: ResourceTopItem[];
  query_time: number;
}

// 資源 Top 項
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

// 控制面狀態響應
export interface ControlPlaneStatusResponse {
  overall: 'healthy' | 'degraded' | 'unhealthy';
  components: ControlPlaneComponent[];
  check_time: number;
}

// 控制面元件
export interface ControlPlaneComponent {
  name: string;
  type: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  message: string;
  last_check_time: number;
  metrics?: ComponentMetrics;
  instances?: ComponentInstance[];
}

// 元件指標
export interface ComponentMetrics {
  request_rate?: number;
  error_rate?: number;
  latency?: number;
  queue_length?: number;
  leader_status?: boolean;
  db_size?: number;
  member_count?: number;
}

// 元件例項
export interface ComponentInstance {
  name: string;
  node: string;
  status: string;
  ip: string;
  start_time: number;
}

export const omService = {
  // 獲取叢集健康診斷
  getHealthDiagnosis: (clusterId: string) => {
    return request.get<HealthDiagnosisResponse>(`/clusters/${clusterId}/om/health-diagnosis`);
  },

  // 獲取資源消耗 Top N
  getResourceTop: (clusterId: string, params: ResourceTopRequest) => {
    return request.get<ResourceTopResponse>(`/clusters/${clusterId}/om/resource-top`, { params });
  },

  // 獲取控制面元件狀態
  getControlPlaneStatus: (clusterId: string) => {
    return request.get<ControlPlaneStatusResponse>(`/clusters/${clusterId}/om/control-plane-status`);
  },
};
