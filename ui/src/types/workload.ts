// 容器探針配置
export interface ProbeConfig {
  // HTTP檢查
  httpGet?: {
    path: string;
    port: number;
    scheme?: 'HTTP' | 'HTTPS';
    httpHeaders?: Array<{ name: string; value: string }>;
  };
  // 命令檢查
  exec?: {
    command: string[];
  };
  // TCP檢查
  tcpSocket?: {
    port: number;
  };
  // 通用參數
  initialDelaySeconds?: number;
  periodSeconds?: number;
  timeoutSeconds?: number;
  successThreshold?: number;
  failureThreshold?: number;
}

// 容器生命週期配置
export interface LifecycleConfig {
  postStart?: {
    exec?: { command: string[] };
    httpGet?: {
      path: string;
      port: number;
      host?: string;
      scheme?: string;
    };
  };
  preStop?: {
    exec?: { command: string[] };
    httpGet?: {
      path: string;
      port: number;
      host?: string;
      scheme?: string;
    };
  };
}

// 容器資源配置（支援 cpu、memory、ephemeral-storage、nvidia.com/gpu）
export interface ResourceConfig {
  limits?: {
    cpu?: string;
    memory?: string;
    'ephemeral-storage'?: string;  // 臨時儲存限制
    'nvidia.com/gpu'?: string;
  };
  requests?: {
    cpu?: string;
    memory?: string;
    'ephemeral-storage'?: string;  // 臨時儲存請求
  };
}

// 資料卷掛載配置
export interface VolumeMount {
  name: string;
  mountPath: string;
  subPath?: string;
  readOnly?: boolean;
}

// 資料卷配置
export interface VolumeConfig {
  name: string;
  type: 'emptyDir' | 'hostPath' | 'configMap' | 'secret' | 'persistentVolumeClaim';
  // EmptyDir
  emptyDir?: {
    medium?: '' | 'Memory';
    sizeLimit?: string;
  };
  // HostPath
  hostPath?: {
    path: string;
    type?: 'DirectoryOrCreate' | 'Directory' | 'FileOrCreate' | 'File' | 'Socket' | 'CharDevice' | 'BlockDevice';
  };
  // ConfigMap
  configMap?: {
    name: string;
    items?: Array<{ key: string; path: string }>;
    defaultMode?: number;
  };
  // Secret
  secret?: {
    secretName: string;
    items?: Array<{ key: string; path: string }>;
    defaultMode?: number;
  };
  // PVC
  persistentVolumeClaim?: {
    claimName: string;
    readOnly?: boolean;
  };
}

// 容器配置
export interface ContainerConfig {
  name: string;
  image: string;
  imagePullPolicy?: 'Always' | 'IfNotPresent' | 'Never';
  // 啟動命令
  command?: string[];
  args?: string[];
  // 工作目錄
  workingDir?: string;
  // 連接埠配置
  ports?: Array<{
    name?: string;
    containerPort: number;
    protocol?: 'TCP' | 'UDP' | 'SCTP';
  }>;
  // 環境變數
  env?: Array<{
    name: string;
    value?: string;
    valueFrom?: {
      configMapKeyRef?: { name: string; key: string };
      secretKeyRef?: { name: string; key: string };
      fieldRef?: { fieldPath: string };
      resourceFieldRef?: { containerName?: string; resource: string };
    };
  }>;
  // 資源配置
  resources?: ResourceConfig;
  // 資料卷掛載
  volumeMounts?: VolumeMount[];
  // 生命週期
  lifecycle?: LifecycleConfig;
  // 健康檢查
  livenessProbe?: ProbeConfig;
  readinessProbe?: ProbeConfig;
  startupProbe?: ProbeConfig;
  // 標準輸入
  stdin?: boolean;
  stdinOnce?: boolean;
  tty?: boolean;
}

// 節點親和性 - 必須滿足
export interface NodeSelectorTerm {
  matchExpressions?: Array<{
    key: string;
    operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt';
    values?: string[];
  }>;
  matchFields?: Array<{
    key: string;
    operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt';
    values?: string[];
  }>;
}

// 節點親和性 - 儘量滿足
export interface PreferredSchedulingTerm {
  weight: number;
  preference: NodeSelectorTerm;
}

// 節點親和性配置
export interface NodeAffinityConfig {
  // 必須滿足 (requiredDuringSchedulingIgnoredDuringExecution)
  required?: {
    nodeSelectorTerms: NodeSelectorTerm[];
  };
  // 儘量滿足 (preferredDuringSchedulingIgnoredDuringExecution)
  preferred?: PreferredSchedulingTerm[];
}

// Pod親和性條件
export interface PodAffinityTerm {
  labelSelector?: {
    matchLabels?: Record<string, string>;
    matchExpressions?: Array<{
      key: string;
      operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist';
      values?: string[];
    }>;
  };
  namespaces?: string[];
  topologyKey: string;
  namespaceSelector?: {
    matchLabels?: Record<string, string>;
    matchExpressions?: Array<{
      key: string;
      operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist';
      values?: string[];
    }>;
  };
}

// Pod親和性 - 儘量滿足
export interface WeightedPodAffinityTerm {
  weight: number;
  podAffinityTerm: PodAffinityTerm;
}

// Pod親和性配置
export interface PodAffinityConfig {
  // 必須滿足
  required?: PodAffinityTerm[];
  // 儘量滿足
  preferred?: WeightedPodAffinityTerm[];
}

// 排程策略配置
export interface SchedulingConfig {
  // 節點親和
  nodeAffinity?: NodeAffinityConfig;
  // Pod親和
  podAffinity?: PodAffinityConfig;
  // Pod反親和
  podAntiAffinity?: PodAffinityConfig;
}

// 容忍配置
export interface TolerationConfig {
  key?: string;
  operator: 'Equal' | 'Exists';
  value?: string;
  effect?: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute' | '';
  tolerationSeconds?: number;
}

// DNS配置
export interface DNSConfig {
  nameservers?: string[];
  searches?: string[];
  options?: Array<{ name: string; value?: string }>;
}

// 升級策略配置
export interface UpdateStrategyConfig {
  type: 'RollingUpdate' | 'Recreate';
  rollingUpdate?: {
    maxUnavailable?: string | number;
    maxSurge?: string | number;
  };
}

// Argo Rollout 金絲雀釋出步驟
export interface CanaryStep {
  // 設定流量權重
  setWeight?: number;
  // 暫停 - 可以是無限期暫停或指定時長
  pause?: {
    duration?: string;  // 例如: "10m", "1h"
  };
  // 設定金絲雀副本數比例
  setCanaryScale?: {
    replicas?: number;
    weight?: number;
    matchTrafficWeight?: boolean;
  };
  // 分析執行
  analysis?: {
    templates?: Array<{
      templateName: string;
    }>;
    args?: Array<{
      name: string;
      value: string;
    }>;
  };
}

// Argo Rollout 金絲雀策略配置
export interface CanaryStrategyConfig {
  // 釋出步驟
  steps?: CanaryStep[];
  // 最大超量
  maxSurge?: string | number;
  // 最大不可用
  maxUnavailable?: string | number;
  // 金絲雀服務名稱 (用於流量路由)
  canaryService?: string;
  // 穩定版本服務名稱
  stableService?: string;
  // 流量路由配置
  trafficRouting?: {
    // Nginx Ingress
    nginx?: {
      stableIngress: string;
      annotationPrefix?: string;
      additionalIngressAnnotations?: Record<string, string>;
    };
    // Istio
    istio?: {
      virtualService?: {
        name: string;
        routes?: string[];
      };
      destinationRule?: {
        name: string;
        canarySubsetName?: string;
        stableSubsetName?: string;
      };
    };
    // ALB Ingress
    alb?: {
      ingress: string;
      servicePort: number;
      annotationPrefix?: string;
    };
  };
  // 分析配置
  analysis?: {
    templates?: Array<{
      templateName: string;
    }>;
    startingStep?: number;
    args?: Array<{
      name: string;
      value?: string;
      valueFrom?: {
        podTemplateHashValue?: 'Latest' | 'Stable';
        fieldRef?: {
          fieldPath: string;
        };
      };
    }>;
  };
  // 反親和配置
  antiAffinity?: {
    requiredDuringSchedulingIgnoredDuringExecution?: Record<string, unknown>;
    preferredDuringSchedulingIgnoredDuringExecution?: {
      weight: number;
    };
  };
  // 金絲雀後設資料
  canaryMetadata?: {
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  // 穩定版本後設資料
  stableMetadata?: {
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
}

// Argo Rollout 藍綠髮布策略配置
export interface BlueGreenStrategyConfig {
  // 活躍服務名稱 (生產流量)
  activeService: string;
  // 預覽服務名稱 (測試流量)
  previewService?: string;
  // 自動晉升啟用
  autoPromotionEnabled?: boolean;
  // 自動晉升延遲時間(秒)
  autoPromotionSeconds?: number;
  // 縮容延遲時間(秒)
  scaleDownDelaySeconds?: number;
  // 縮容延遲修訂版本限制
  scaleDownDelayRevisionLimit?: number;
  // 預覽副本數
  previewReplicaCount?: number;
  // 預覽後設資料
  previewMetadata?: {
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  // 活躍後設資料
  activeMetadata?: {
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  // 反親和配置
  antiAffinity?: {
    requiredDuringSchedulingIgnoredDuringExecution?: Record<string, unknown>;
    preferredDuringSchedulingIgnoredDuringExecution?: {
      weight: number;
    };
  };
  // 預晉升分析
  prePromotionAnalysis?: {
    templates?: Array<{
      templateName: string;
    }>;
    args?: Array<{
      name: string;
      value: string;
    }>;
  };
  // 後晉升分析
  postPromotionAnalysis?: {
    templates?: Array<{
      templateName: string;
    }>;
    args?: Array<{
      name: string;
      value: string;
    }>;
  };
}

// Argo Rollout 策略配置
export interface RolloutStrategyConfig {
  // 策略型別
  type: 'Canary' | 'BlueGreen';
  // 金絲雀策略
  canary?: CanaryStrategyConfig;
  // 藍綠策略
  blueGreen?: BlueGreenStrategyConfig;
}

// 完整的工作負載表單資料
export interface WorkloadFormData {
  // 基本資訊
  name: string;
  namespace: string;
  description?: string;
  replicas?: number;
  labels?: Array<{ key: string; value: string }>;
  annotations?: Array<{ key: string; value: string }>;
  
  // 容器配置 - 支援多容器
  containers: ContainerConfig[];
  // Init容器
  initContainers?: ContainerConfig[];
  
  // 資料卷
  volumes?: VolumeConfig[];
  
  // 映像拉取憑證
  imagePullSecrets?: string[];
  
  // 排程策略
  scheduling?: SchedulingConfig;
  // 節點選擇器 (簡化版)
  nodeSelector?: Record<string, string>;
  // 容忍策略
  tolerations?: TolerationConfig[];
  
  // 升級策略
  strategy?: UpdateStrategyConfig;
  minReadySeconds?: number;
  revisionHistoryLimit?: number;
  progressDeadlineSeconds?: number;
  
  // 終止配置
  terminationGracePeriodSeconds?: number;
  
  // DNS配置
  dnsPolicy?: 'ClusterFirst' | 'ClusterFirstWithHostNet' | 'Default' | 'None';
  dnsConfig?: DNSConfig;
  
  // 主機網路
  hostNetwork?: boolean;
  hostPID?: boolean;
  hostIPC?: boolean;
  
  // StatefulSet 特有
  serviceName?: string;
  podManagementPolicy?: 'OrderedReady' | 'Parallel';
  
  // CronJob 特有
  schedule?: string;
  suspend?: boolean;
  concurrencyPolicy?: 'Allow' | 'Forbid' | 'Replace';
  successfulJobsHistoryLimit?: number;
  failedJobsHistoryLimit?: number;
  
  // Job 特有
  completions?: number;
  parallelism?: number;
  backoffLimit?: number;
  activeDeadlineSeconds?: number;
  ttlSecondsAfterFinished?: number;
  
  // Argo Rollout 特有
  rolloutStrategy?: RolloutStrategyConfig;
}

// 表單中排程策略的簡化格式
export interface SchedulingFormData {
  // 節點親和 - 必須滿足
  nodeAffinityRequired?: Array<{
    key: string;
    operator: string;
    values: string;
  }>;
  // 節點親和 - 儘量滿足
  nodeAffinityPreferred?: Array<{
    weight: number;
    key: string;
    operator: string;
    values: string;
  }>;
  // Pod親和 - 必須滿足
  podAffinityRequired?: Array<{
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }>;
  // Pod親和 - 儘量滿足
  podAffinityPreferred?: Array<{
    weight: number;
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }>;
  // Pod反親和 - 必須滿足
  podAntiAffinityRequired?: Array<{
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }>;
  // Pod反親和 - 儘量滿足
  podAntiAffinityPreferred?: Array<{
    weight: number;
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }>;
}

