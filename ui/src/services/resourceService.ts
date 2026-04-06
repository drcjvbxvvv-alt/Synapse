import { request } from '../utils/api';

// 資源型別定義
export type ResourceKind = 
  | 'ConfigMap'
  | 'Secret'
  | 'Service'
  | 'Ingress'
  | 'PersistentVolumeClaim'
  | 'PersistentVolume'
  | 'StorageClass';

// YAML 應用請求
export interface YAMLApplyRequest {
  yaml: string;
  dryRun?: boolean;
}

// YAML 應用響應
export interface YAMLApplyResponse {
  name: string;
  namespace?: string;
  kind: string;
  resourceVersion?: string;
  isCreated: boolean;
}

// YAML 獲取響應
export interface YAMLGetResponse {
  yaml: string;
}

// 資源端點對映
const resourceEndpoints: Record<ResourceKind, string> = {
  ConfigMap: 'configmaps',
  Secret: 'secrets',
  Service: 'services',
  Ingress: 'ingresses',
  PersistentVolumeClaim: 'pvcs',
  PersistentVolume: 'pvs',
  StorageClass: 'storageclasses',
};

// 是否需要命名空間
const namespaceRequired: Record<ResourceKind, boolean> = {
  ConfigMap: true,
  Secret: true,
  Service: true,
  Ingress: true,
  PersistentVolumeClaim: true,
  PersistentVolume: false,
  StorageClass: false,
};

/**
 * 通用資源服務
 * 提供所有資源型別的 YAML 應用和獲取功能
 */
export class ResourceService {
  /**
   * 應用 YAML 配置
   * @param clusterId 叢集 ID
   * @param kind 資源型別
   * @param yaml YAML 內容
   * @param dryRun 是否為預檢模式
   */
  static async applyYAML(
    clusterId: string,
    kind: ResourceKind,
    yaml: string,
    dryRun = false
  ): Promise<YAMLApplyResponse> {
    const endpoint = resourceEndpoints[kind];
    return request.post(`/clusters/${clusterId}/${endpoint}/yaml/apply`, {
      yaml,
      dryRun,
    });
  }

  /**
   * 獲取資源的 YAML（乾淨版本，用於編輯）
   * @param clusterId 叢集 ID
   * @param kind 資源型別
   * @param namespace 命名空間（叢集級資源可不傳）
   * @param name 資源名稱
   */
  static async getYAML(
    clusterId: string,
    kind: ResourceKind,
    namespace: string | null,
    name: string
  ): Promise<YAMLGetResponse> {
    const endpoint = resourceEndpoints[kind];
    const needsNamespace = namespaceRequired[kind];
    
    let url: string;
    if (needsNamespace && namespace) {
      url = `/clusters/${clusterId}/${endpoint}/${namespace}/${name}/yaml/clean`;
    } else {
      url = `/clusters/${clusterId}/${endpoint}/${name}/yaml/clean`;
    }
    
    return request.get(url);
  }

  /**
   * 從 YAML 中解析資源型別
   */
  static parseKindFromYAML(yaml: string): ResourceKind | null {
    const match = yaml.match(/kind:\s*(\w+)/);
    if (match) {
      const kind = match[1];
      // 對映到標準型別名
      const kindMap: Record<string, ResourceKind> = {
        ConfigMap: 'ConfigMap',
        Secret: 'Secret',
        Service: 'Service',
        Ingress: 'Ingress',
        PersistentVolumeClaim: 'PersistentVolumeClaim',
        PersistentVolume: 'PersistentVolume',
        StorageClass: 'StorageClass',
      };
      return kindMap[kind] || null;
    }
    return null;
  }

  /**
   * 根據資源型別獲取顯示名稱
   */
  static getKindDisplayName(kind: ResourceKind): string {
    const displayNames: Record<ResourceKind, string> = {
      ConfigMap: 'ConfigMap',
      Secret: 'Secret',
      Service: 'Service',
      Ingress: 'Ingress',
      PersistentVolumeClaim: 'PVC',
      PersistentVolume: 'PV',
      StorageClass: 'StorageClass',
    };
    return displayNames[kind];
  }

  /**
   * 檢查資源是否需要命名空間
   */
  static isNamespaced(kind: ResourceKind): boolean {
    return namespaceRequired[kind];
  }

  /**
   * 獲取預設 YAML 模板
   */
  static getDefaultYAML(kind: ResourceKind, namespace = 'default'): string {
    const templates: Record<ResourceKind, string> = {
      ConfigMap: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: ${namespace}
data:
  key1: value1
  key2: value2
`,
      Secret: `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: ${namespace}
type: Opaque
stringData:
  username: admin
  password: password123
`,
      Service: `apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: ${namespace}
spec:
  type: ClusterIP
  selector:
    app: my-app
  ports:
    - name: http
      port: 80
      targetPort: 8080
      protocol: TCP
`,
      Ingress: `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: ${namespace}
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80
`,
      PersistentVolumeClaim: `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  namespace: ${namespace}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard
`,
      PersistentVolume: `apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: standard
  hostPath:
    path: /data/my-pv
`,
      StorageClass: `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-storageclass
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
`,
    };
    return templates[kind];
  }
}

export default ResourceService;

