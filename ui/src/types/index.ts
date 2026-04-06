// 叢集相關型別定義
export interface Cluster {
  id: string;
  name: string;
  apiServer: string;
  version: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  nodeCount: number;
  readyNodes: number;
  cpuUsage: number;
  memoryUsage: number;
  lastHeartbeat: string;
  createdAt: string;
  labels?: Record<string, string>;
}

// 容器子網IP資訊
export interface ContainerSubnetIPs {
  total_ips: number;
  used_ips: number;
  available_ips: number;
}

// 叢集概覽資訊
export interface ClusterOverview {
  clusterID: number;
  nodes: number;
  namespace: number;
  pods: number;
  deployments: number;
  statefulsets: number;
  daemonsets: number;
  jobs: number;
  rollouts: number;
  containerSubnetIPs?: ContainerSubnetIPs;
}

export interface ClusterStats {
  totalClusters: number;
  healthyClusters: number;
  unhealthyClusters: number;
  totalNodes: number;
  readyNodes: number;
  totalPods: number;
  runningPods: number;
}

// 節點相關型別定義
export interface NodeAddress {
  address: string;
  type: string;
}

export interface Node {
  id: string;
  name: string;
  clusterId: string;
  addresses: NodeAddress[];
  status: 'Ready' | 'NotReady' | 'Unknown';
  roles: string[];
  version: string;
  osImage: string;
  kernelVersion: string;
  kubeletVersion: string;
  containerRuntime: string;
  resources: NodeResource;
  cpuUsage: number;
  memoryUsage: number;
  podCount: number;
  maxPods: number;
  conditions: NodeCondition[];
  taints: NodeTaint[];
  labels?: { key: string; value: string }[];
  creationTimestamp: string;
  unschedulable: boolean;
}

export interface NodeResource {
  cpu: string;
  memory: string;
  pods: number;
}

export interface NodeCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime: string;
}

export interface NodeTaint {
  key: string;
  value?: string;
  effect: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute';
}

// Pod相關型別定義
export interface Pod {
  id: string;
  name: string;
  namespace: string;
  clusterId: string;
  nodeName: string;
  status: 'Running' | 'Pending' | 'Succeeded' | 'Failed' | 'Unknown';
  phase: string;
  restartCount: number;
  cpuUsage: number;
  memoryUsage: number;
  containers: Container[];
  labels: Record<string, string>;
  createdAt: string;
  startTime?: string;
}

export interface Container {
  name: string;
  image: string;
  ready: boolean;
  restartCount: number;
  state: ContainerState;
}

export interface ContainerState {
  waiting?: {
    reason: string;
    message?: string;
  };
  running?: {
    startedAt: string;
  };
  terminated?: {
    exitCode: number;
    reason: string;
    message?: string;
    startedAt: string;
    finishedAt: string;
  };
}

// 工作負載相關型別定義
export interface Workload {
  id: string;
  name: string;
  namespace: string;
  clusterId: string;
  kind: 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Job' | 'CronJob';
  replicas: number;
  readyReplicas: number;
  availableReplicas: number;
  images: string[];
  labels: Record<string, string>;
  createdAt: string;
  updatedAt: string;
}

// API 響應型別（後端直接返回資料體，無包裝）
export type ApiResponse<T> = T;

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

// 搜尋相關型別
export interface SearchResult {
  id: string;
  name: string;
  type: 'cluster' | 'node' | 'pod' | 'workload';
  namespace?: string;
  clusterId: string;
  clusterName: string;
  status: string;
  description?: string;
  ip?: string;
  kind?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
  stats: {
    cluster: number;
    node: number;
    pod: number;
    workload: number;
  };
}

// 監控資料型別
export interface MetricData {
  timestamp: number;
  value: number;
}

export interface MetricSeries {
  name: string;
  data: MetricData[];
}

 // K8s 事件型別
export interface K8sEventInvolvedObject {
  kind: string;
  name: string;
  namespace?: string;
  uid?: string;
  apiVersion?: string;
  fieldPath?: string;
}

export interface K8sEvent {
  metadata?: {
    uid?: string;
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
  };
  involvedObject: K8sEventInvolvedObject;
  type: 'Normal' | 'Warning' | string;
  reason: string;
  message: string;
  source?: { component?: string; host?: string };
  firstTimestamp?: string;
  lastTimestamp?: string;
  eventTime?: string;
  count?: number;
}

// Service相關型別定義
export interface ServicePort {
  name: string;
  protocol: string;
  port: number;
  targetPort: string;
  nodePort?: number;
}

export interface LoadBalancerIngress {
  ip?: string;
  hostname?: string;
}

export interface Service {
  name: string;
  namespace: string;
  type: 'ClusterIP' | 'NodePort' | 'LoadBalancer' | 'ExternalName';
  clusterIP: string;
  externalIPs?: string[];
  ports: ServicePort[];
  selector: Record<string, string>;
  sessionAffinity: string;
  loadBalancerIP?: string;
  loadBalancerIngress?: LoadBalancerIngress[];
  externalName?: string;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
}

// Ingress相關型別定義
export interface IngressPathInfo {
  path: string;
  pathType: string;
  serviceName: string;
  servicePort: string;
}

export interface IngressRuleInfo {
  host: string;
  paths: IngressPathInfo[];
}

export interface IngressTLSInfo {
  hosts: string[];
  secretName: string;
}

export interface LoadBalancerStatus {
  ip?: string;
  hostname?: string;
}

export interface Ingress {
  name: string;
  namespace: string;
  ingressClassName?: string;
  rules: IngressRuleInfo[];
  tls?: IngressTLSInfo[];
  loadBalancer?: LoadBalancerStatus[];
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
}

// Endpoints相關型別定義
export interface EndpointAddress {
  ip: string;
  nodeName?: string;
  targetRef?: {
    kind: string;
    name: string;
    namespace: string;
  };
}

export interface EndpointPort {
  name: string;
  port: number;
  protocol: string;
}

export interface EndpointSubset {
  addresses: EndpointAddress[];
  ports: EndpointPort[];
}

export interface Endpoints {
  name: string;
  namespace: string;
  subsets: EndpointSubset[];
}

// 儲存相關型別定義 - PVC
export interface PVC {
  name: string;
  namespace: string;
  status: string;
  volumeName: string;
  storageClassName: string;
  accessModes: string[];
  capacity: string;
  volumeMode: string;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
}

// 儲存相關型別定義 - PV
export interface PVClaimRef {
  namespace: string;
  name: string;
}

export interface PV {
  name: string;
  status: string;
  capacity: string;
  accessModes: string[];
  reclaimPolicy: string;
  storageClassName: string;
  volumeMode: string;
  claimRef?: PVClaimRef;
  persistentVolumeSource: string;
  mountOptions?: string[];
  nodeAffinity?: string;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
}

// 儲存相關型別定義 - StorageClass
export interface StorageClass {
  name: string;
  provisioner: string;
  reclaimPolicy: string;
  volumeBindingMode: string;
  allowVolumeExpansion: boolean;
  parameters?: Record<string, string>;
  mountOptions?: string[];
  isDefault: boolean;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
}

// 使用者相關型別定義
export interface User {
  id: number;
  username: string;
  email: string;
  display_name: string;
  auth_type: 'local' | 'ldap';
  status: 'active' | 'inactive' | 'locked';
  last_login_at?: string;
  last_login_ip?: string;
  created_at: string;
  updated_at: string;
}

// 使用者管理請求型別
export interface CreateUserRequest {
  username: string;
  password: string;
  email?: string;
  display_name?: string;
}

export interface UpdateUserRequest {
  email?: string;
  display_name?: string;
}

// LDAP配置型別
export interface LDAPConfig {
  enabled: boolean;
  server: string;
  port: number;
  use_tls: boolean;
  skip_tls_verify: boolean;
  bind_dn: string;
  bind_password: string;
  base_dn: string;
  user_filter: string;
  username_attr: string;
  email_attr: string;
  display_name_attr: string;
  group_filter: string;
  group_attr: string;
}

// SSH配置型別
export interface SSHConfig {
  enabled: boolean;
  username: string;
  port: number;
  auth_type: 'password' | 'key';
  password?: string;
  private_key?: string;
}

// Grafana 配置型別
export interface GrafanaConfig {
  url: string;
  api_key: string;
}

// 系統安全設定
export interface SecurityConfig {
  session_ttl_minutes: number;
  login_fail_lock_threshold: number;
  lock_duration_minutes: number;
  password_min_length: number;
}

// API Token
export interface APIToken {
  id: number;
  name: string;
  scopes: string[];
  expires_at?: string;
  last_used_at?: string;
  created_at: string;
}

// 建立 API Token 回應（含一次性明文）
export interface CreateAPITokenResponse {
  id: number;
  name: string;
  token: string;
  expires_at?: string;
  created_at: string;
}

// Grafana Dashboard 同步狀態
export interface GrafanaDashboardSyncStatus {
  folder_exists: boolean;
  dashboards: GrafanaDashboardStatusItem[];
  all_synced: boolean;
}

export interface GrafanaDashboardStatusItem {
  uid: string;
  title: string;
  exists: boolean;
}

// Grafana 資料來源同步狀態
export interface GrafanaDataSourceSyncStatus {
  datasources: GrafanaDataSourceStatusItem[];
  all_synced: boolean;
}

export interface GrafanaDataSourceStatusItem {
  cluster_name: string;
  datasource_uid: string;
  prometheus_url: string;
  exists: boolean;
}

// ========== 權限管理型別 ==========

// 權限型別常量
export type PermissionType = 'admin' | 'ops' | 'dev' | 'readonly' | 'custom';

// 權限型別資訊
export interface PermissionTypeInfo {
  type: PermissionType;
  name: string;
  description: string;
  resources: string[];
  actions: string[];
  allowPartialNamespaces: boolean;  // 是否允許選擇部分命名空間
  requireAllNamespaces: boolean;    // 是否必須選擇全部命名空間
}

// 使用者組
export interface UserGroup {
  id: number;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  users?: User[];
}

// 叢集權限配置
export interface ClusterPermission {
  id: number;
  cluster_id: number;
  cluster_name?: string;
  user_id?: number;
  username?: string;
  user_group_id?: number;
  user_group_name?: string;
  permission_type: PermissionType;
  permission_name: string;
  namespaces: string[];
  custom_role_ref?: string;
  created_at: string;
  updated_at: string;
}

// 建立叢集權限請求
export interface CreateClusterPermissionRequest {
  cluster_id: number;
  user_id?: number;
  user_group_id?: number;
  user_ids?: number[];
  user_group_ids?: number[];
  permission_type: PermissionType;
  namespaces?: string[];
  custom_role_ref?: string;
}

// 更新叢集權限請求
export interface UpdateClusterPermissionRequest {
  permission_type?: PermissionType;
  namespaces?: string[];
  custom_role_ref?: string;
}

// 我的權限響應
export interface MyPermissionsResponse {
  cluster_id: number;
  cluster_name: string;
  permission_type: PermissionType;
  permission_name: string;
  namespaces: string[];
  allowed_actions: string[];
  custom_role_ref?: string;
}

// 建立使用者組請求
export interface CreateUserGroupRequest {
  name: string;
  description?: string;
}

// 更新使用者組請求
export interface UpdateUserGroupRequest {
  name: string;
  description?: string;
}
