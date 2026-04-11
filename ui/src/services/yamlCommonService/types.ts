import type {
  WorkloadFormData,
  ContainerConfig,
  VolumeConfig,
  TolerationConfig,
} from '../../types/workload';

// ==================== 公共型別 ====================

export interface CommonBuildParts {
  metadata: Record<string, unknown>;
  labels: Record<string, string>;
  podSpec: Record<string, unknown>;
  podTemplateSpec: {
    metadata: { labels: Record<string, string> };
    spec: Record<string, unknown>;
  };
}

export interface CommonParsedFields {
  name: string;
  namespace: string;
  description?: string;
  labels: Array<{ key: string; value: string }>;
  annotations: Array<{ key: string; value: string }>;
  containers: ContainerConfig[];
  initContainers?: ContainerConfig[];
  volumes?: VolumeConfig[];
  imagePullSecrets?: string[];
  scheduling?: WorkloadFormData['scheduling'];
  tolerations?: TolerationConfig[];
  terminationGracePeriodSeconds?: number;
  dnsPolicy?: WorkloadFormData['dnsPolicy'];
  dnsConfig?: WorkloadFormData['dnsConfig'];
  hostNetwork?: boolean;
}
