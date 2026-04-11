import * as YAML from 'yaml';
import type {
  WorkloadFormData,
  ContainerConfig,
  VolumeConfig,
  TolerationConfig,
} from '../../types/workload';
import type { CommonBuildParts, CommonParsedFields } from './types';
import { parseCommaString, commandArrayToString } from './stringHelpers';
import { buildContainerSpec, buildVolumeSpec, buildSchedulingSpec } from './builders';
import { buildSchedulingFromForm } from './affinityParser';
import { parseAffinityToScheduling } from './affinityParser';

// ==================== 公共構建函式 ====================

export const buildCommonParts = (formData: WorkloadFormData): CommonBuildParts => {
  const labels: Record<string, string> = {};
  if (formData.labels && formData.labels.length > 0) {
    formData.labels.forEach(item => {
      if (item.key && item.value) {
        labels[item.key] = item.value;
      }
    });
  }
  if (Object.keys(labels).length === 0) {
    labels.app = formData.name || 'app';
  }

  const annotations: Record<string, string> = {};
  if (formData.description) {
    annotations['description'] = formData.description;
  }
  if (formData.annotations && formData.annotations.length > 0) {
    formData.annotations.forEach(item => {
      if (item.key && item.value) {
        annotations[item.key] = item.value;
      }
    });
  }

  const metadata: Record<string, unknown> = {
    name: formData.name || 'example',
    namespace: formData.namespace || 'default',
    labels: { ...labels },
    ...(Object.keys(annotations).length > 0 && { annotations }),
  };

  const containers = (formData.containers || []).map(c => buildContainerSpec(c));
  const initContainers = (formData.initContainers || []).map(c => buildContainerSpec(c));
  const volumes = (formData.volumes || []).map(v => buildVolumeSpec(v));

  const scheduling = buildSchedulingFromForm(formData as unknown as Record<string, unknown>);
  const affinity = buildSchedulingSpec(scheduling);

  const podSpec: Record<string, unknown> = {
    containers,
    ...(initContainers.length > 0 && { initContainers }),
    ...(volumes.length > 0 && { volumes }),
    ...(affinity && { affinity }),
    ...(formData.nodeSelector && Object.keys(formData.nodeSelector).length > 0 && { nodeSelector: formData.nodeSelector }),
    ...(formData.tolerations && formData.tolerations.length > 0 && {
      tolerations: formData.tolerations.map(t => ({
        ...(t.key && { key: t.key }),
        operator: t.operator,
        ...(t.value && { value: t.value }),
        ...(t.effect && { effect: t.effect }),
        ...(t.tolerationSeconds !== undefined && { tolerationSeconds: t.tolerationSeconds }),
      })),
    }),
    ...(formData.dnsPolicy && { dnsPolicy: formData.dnsPolicy }),
    ...(formData.dnsConfig && {
      dnsConfig: {
        ...(formData.dnsConfig.nameservers && { nameservers: parseCommaString(formData.dnsConfig.nameservers as unknown as string) }),
        ...(formData.dnsConfig.searches && { searches: parseCommaString(formData.dnsConfig.searches as unknown as string) }),
      },
    }),
    ...(formData.terminationGracePeriodSeconds !== undefined && { terminationGracePeriodSeconds: formData.terminationGracePeriodSeconds }),
    ...(formData.hostNetwork && { hostNetwork: formData.hostNetwork }),
    ...(formData.imagePullSecrets && formData.imagePullSecrets.length > 0 && {
      imagePullSecrets: formData.imagePullSecrets.map(s => ({ name: s })),
    }),
  };

  const podTemplateSpec = {
    metadata: { labels: { ...labels } },
    spec: podSpec,
  };

  return { metadata, labels, podSpec, podTemplateSpec };
};

// ==================== 公共解析函式 ====================

const parseProbe = (probe: Record<string, unknown> | undefined) => {
  if (!probe) return undefined;
  let type = 'httpGet';
  if (probe.exec) type = 'exec';
  else if (probe.tcpSocket) type = 'tcpSocket';

  const result: Record<string, unknown> = {
    enabled: true,
    type,
  };

  Object.keys(probe).forEach(key => {
    if (key !== 'exec') {
      result[key] = probe[key];
    }
  });

  if (probe.exec) {
    result.exec = {
      command: commandArrayToString((probe.exec as Record<string, unknown>).command as string[]),
    };
  }

  return result;
};

export const parseCommonFields = (
  obj: Record<string, unknown>,
  podSpec: Record<string, unknown>,
): CommonParsedFields => {
  const metadata = (obj.metadata || {}) as Record<string, unknown>;

  const containers: ContainerConfig[] = ((podSpec.containers as Record<string, unknown>[]) || []).map((c) => ({
    name: c.name as string || 'main',
    image: c.image as string || '',
    imagePullPolicy: c.imagePullPolicy as 'Always' | 'IfNotPresent' | 'Never' | undefined,
    command: commandArrayToString(c.command as string[]) as unknown as string[],
    args: commandArrayToString(c.args as string[]) as unknown as string[],
    workingDir: c.workingDir as string | undefined,
    ports: c.ports as ContainerConfig['ports'],
    env: c.env as ContainerConfig['env'],
    resources: c.resources as ContainerConfig['resources'],
    volumeMounts: c.volumeMounts as ContainerConfig['volumeMounts'],
    lifecycle: c.lifecycle ? {
      postStart: (c.lifecycle as Record<string, unknown>).postStart ? {
        exec: {
          command: commandArrayToString(
            ((c.lifecycle as Record<string, unknown>).postStart as Record<string, unknown>)?.exec
              ? (((c.lifecycle as Record<string, unknown>).postStart as Record<string, unknown>).exec as Record<string, unknown>).command as string[]
              : undefined
          ) as unknown as string[],
        },
      } : undefined,
      preStop: (c.lifecycle as Record<string, unknown>).preStop ? {
        exec: {
          command: commandArrayToString(
            ((c.lifecycle as Record<string, unknown>).preStop as Record<string, unknown>)?.exec
              ? (((c.lifecycle as Record<string, unknown>).preStop as Record<string, unknown>).exec as Record<string, unknown>).command as string[]
              : undefined
          ) as unknown as string[],
        },
      } : undefined,
    } : undefined,
    livenessProbe: parseProbe(c.livenessProbe as Record<string, unknown>) as ContainerConfig['livenessProbe'],
    readinessProbe: parseProbe(c.readinessProbe as Record<string, unknown>) as ContainerConfig['readinessProbe'],
    startupProbe: parseProbe(c.startupProbe as Record<string, unknown>) as ContainerConfig['startupProbe'],
  }));

  const initContainers: ContainerConfig[] = ((podSpec.initContainers as Record<string, unknown>[]) || []).map((c) => ({
    name: c.name as string || 'init',
    image: c.image as string || '',
    imagePullPolicy: c.imagePullPolicy as 'Always' | 'IfNotPresent' | 'Never' | undefined,
    command: commandArrayToString(c.command as string[]) as unknown as string[],
    args: commandArrayToString(c.args as string[]) as unknown as string[],
    workingDir: c.workingDir as string | undefined,
    env: c.env as ContainerConfig['env'],
    resources: c.resources as ContainerConfig['resources'],
    volumeMounts: c.volumeMounts as ContainerConfig['volumeMounts'],
  }));

  const labels = (metadata.labels as Record<string, string>)
    ? Object.entries(metadata.labels as Record<string, string>).map(([key, value]) => ({ key, value: String(value) }))
    : [];
  const annotations = (metadata.annotations as Record<string, string>)
    ? Object.entries(metadata.annotations as Record<string, string>).map(([key, value]) => ({ key, value: String(value) }))
    : [];

  const volumes: VolumeConfig[] = ((podSpec.volumes as Record<string, unknown>[]) || []).map((v) => {
    let type: VolumeConfig['type'] = 'emptyDir';
    if (v.hostPath) type = 'hostPath';
    else if (v.configMap) type = 'configMap';
    else if (v.secret) type = 'secret';
    else if (v.persistentVolumeClaim) type = 'persistentVolumeClaim';

    return {
      name: v.name as string,
      type,
      emptyDir: v.emptyDir as VolumeConfig['emptyDir'],
      hostPath: v.hostPath as VolumeConfig['hostPath'],
      configMap: v.configMap as VolumeConfig['configMap'],
      secret: v.secret as VolumeConfig['secret'],
      persistentVolumeClaim: v.persistentVolumeClaim as VolumeConfig['persistentVolumeClaim'],
    };
  });

  const tolerations: TolerationConfig[] = ((podSpec.tolerations as Record<string, unknown>[]) || []).map((t) => ({
    key: t.key as string | undefined,
    operator: t.operator as 'Equal' | 'Exists' || 'Equal',
    value: t.value as string | undefined,
    effect: t.effect as TolerationConfig['effect'],
    tolerationSeconds: t.tolerationSeconds as number | undefined,
  }));

  const imagePullSecrets = ((podSpec.imagePullSecrets as Record<string, unknown>[]) || []).map((s) => s.name as string);

  const affinityData = podSpec.affinity as Record<string, unknown> | undefined;
  const schedulingData = parseAffinityToScheduling(affinityData);

  return {
    name: (metadata.name as string) || '',
    namespace: (metadata.namespace as string) || 'default',
    description: (metadata.annotations as Record<string, unknown>)?.description as string | undefined,
    labels,
    annotations,
    containers,
    initContainers: initContainers.length > 0 ? initContainers : undefined,
    volumes: volumes.length > 0 ? volumes : undefined,
    imagePullSecrets: imagePullSecrets.length > 0 ? imagePullSecrets : undefined,
    scheduling: schedulingData as WorkloadFormData['scheduling'],
    tolerations: tolerations.length > 0 ? tolerations : undefined,
    terminationGracePeriodSeconds: podSpec.terminationGracePeriodSeconds as number | undefined,
    dnsPolicy: podSpec.dnsPolicy as WorkloadFormData['dnsPolicy'],
    dnsConfig: podSpec.dnsConfig as WorkloadFormData['dnsConfig'],
    hostNetwork: podSpec.hostNetwork as boolean | undefined,
  };
};

// ==================== YAML 序列化 ====================

export const toYAMLString = (obj: Record<string, unknown>): string => {
  return YAML.stringify(obj, { lineWidth: 0 });
};

export const parseYAMLString = (yamlContent: string): Record<string, unknown> | null => {
  try {
    return YAML.parse(yamlContent) || null;
  } catch {
    return null;
  }
};

export const getPodSpec = (obj: Record<string, unknown>): Record<string, unknown> => {
  const spec = (obj.spec || {}) as Record<string, unknown>;
  if (obj.kind === 'CronJob') {
    return (((spec.jobTemplate as Record<string, unknown>)?.spec as Record<string, unknown>)?.template as Record<string, unknown>)?.spec as Record<string, unknown> || {};
  }
  return (spec.template as Record<string, unknown>)?.spec as Record<string, unknown> || {};
};
