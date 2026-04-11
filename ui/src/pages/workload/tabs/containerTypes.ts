// Container tab shared types

export interface ProbeConfig {
  httpGet?: {
    path?: string;
    port: number | string;
    scheme?: string;
    host?: string;
    httpHeaders?: Array<{ name: string; value: string }>;
  };
  tcpSocket?: {
    port: number | string;
    host?: string;
  };
  exec?: {
    command?: string[];
  };
  grpc?: {
    port: number;
    service?: string;
  };
  initialDelaySeconds?: number;
  periodSeconds?: number;
  timeoutSeconds?: number;
  successThreshold?: number;
  failureThreshold?: number;
  terminationGracePeriodSeconds?: number;
}

export interface LifecycleHandler {
  exec?: {
    command?: string[];
  };
  httpGet?: {
    path?: string;
    port: number | string;
    scheme?: string;
    host?: string;
    httpHeaders?: Array<{ name: string; value: string }>;
  };
  tcpSocket?: {
    port: number | string;
    host?: string;
  };
}

export interface ContainerInfo {
  name: string;
  image: string;
  imagePullPolicy: string;
  command?: string[];
  args?: string[];
  workingDir?: string;
  ports?: Array<{
    name?: string;
    containerPort: number;
    protocol: string;
  }>;
  env?: Array<{
    name: string;
    value?: string;
    valueFrom?: {
      configMapKeyRef?: { name: string; key: string; optional?: boolean };
      secretKeyRef?: { name: string; key: string; optional?: boolean };
      fieldRef?: { fieldPath: string; apiVersion?: string };
      resourceFieldRef?: { containerName?: string; resource: string; divisor?: string };
    };
  }>;
  envFrom?: Array<{
    configMapRef?: { name: string; optional?: boolean };
    secretRef?: { name: string; optional?: boolean };
    prefix?: string;
  }>;
  resources?: {
    limits?: {
      cpu?: string;
      memory?: string;
      [key: string]: string | undefined;
    };
    requests?: {
      cpu?: string;
      memory?: string;
      [key: string]: string | undefined;
    };
  };
  volumeMounts?: Array<{
    name: string;
    mountPath: string;
    readOnly?: boolean;
    subPath?: string;
    subPathExpr?: string;
  }>;
  lifecycle?: {
    postStart?: LifecycleHandler;
    preStop?: LifecycleHandler;
  };
  livenessProbe?: ProbeConfig;
  readinessProbe?: ProbeConfig;
  startupProbe?: ProbeConfig;
  securityContext?: {
    privileged?: boolean;
    runAsUser?: number;
    runAsGroup?: number;
    runAsNonRoot?: boolean;
    readOnlyRootFilesystem?: boolean;
    allowPrivilegeEscalation?: boolean;
    capabilities?: {
      add?: string[];
      drop?: string[];
    };
  };
  stdin?: boolean;
  stdinOnce?: boolean;
  tty?: boolean;
  terminationMessagePath?: string;
  terminationMessagePolicy?: string;
}

export interface VolumeConfig {
  name: string;
  configMap?: {
    name: string;
    defaultMode?: number;
    optional?: boolean;
    items?: Array<{ key: string; path: string; mode?: number }>;
  };
  secret?: {
    secretName: string;
    defaultMode?: number;
    optional?: boolean;
    items?: Array<{ key: string; path: string; mode?: number }>;
  };
  emptyDir?: {
    medium?: string;
    sizeLimit?: string;
  };
  hostPath?: {
    path: string;
    type?: string;
  };
  persistentVolumeClaim?: {
    claimName: string;
    readOnly?: boolean;
  };
  nfs?: {
    server: string;
    path: string;
    readOnly?: boolean;
  };
  downwardAPI?: {
    items?: Array<{
      path: string;
      fieldRef?: { fieldPath: string };
      resourceFieldRef?: { containerName?: string; resource: string };
    }>;
    defaultMode?: number;
  };
  projected?: {
    sources?: Array<{
      configMap?: { name: string; items?: Array<{ key: string; path: string }> };
      secret?: { name: string; items?: Array<{ key: string; path: string }> };
      downwardAPI?: { items?: Array<{ path: string; fieldRef?: { fieldPath: string } }> };
      serviceAccountToken?: { path: string; expirationSeconds?: number; audience?: string };
    }>;
    defaultMode?: number;
  };
  csi?: {
    driver: string;
    readOnly?: boolean;
    volumeAttributes?: Record<string, string>;
  };
}

export interface DeploymentSpec {
  replicas?: number;
  selector?: {
    matchLabels?: Record<string, string>;
    matchExpressions?: Array<{ key: string; operator: string; values?: string[] }>;
  };
  template?: {
    metadata?: {
      labels?: Record<string, string>;
      annotations?: Record<string, string>;
    };
    spec?: {
      containers?: ContainerInfo[];
      initContainers?: ContainerInfo[];
      volumes?: VolumeConfig[];
      serviceAccountName?: string;
      nodeSelector?: Record<string, string>;
      tolerations?: Array<{
        key?: string;
        operator?: string;
        value?: string;
        effect?: string;
        tolerationSeconds?: number;
      }>;
      affinity?: {
        nodeAffinity?: Record<string, unknown>;
        podAffinity?: Record<string, unknown>;
        podAntiAffinity?: Record<string, unknown>;
      };
      dnsPolicy?: string;
      restartPolicy?: string;
      terminationGracePeriodSeconds?: number;
      hostNetwork?: boolean;
      hostPID?: boolean;
      hostIPC?: boolean;
    };
  };
}

export interface ContainerTabProps {
  clusterId: string;
  namespace: string;
  deploymentName?: string;
  rolloutName?: string;
  statefulSetName?: string;
  daemonSetName?: string;
  jobName?: string;
  cronJobName?: string;
}
