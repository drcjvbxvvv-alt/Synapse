import type {
  ContainerConfig,
  ProbeConfig,
  VolumeConfig,
  SchedulingConfig,
} from '../../types/workload';
import { parseCommandString } from './stringHelpers';

// ==================== 構建函式 ====================

export const buildProbeConfig = (probe: ProbeConfig & { enabled?: boolean; type?: string }): Record<string, unknown> | undefined => {
  if (!probe || !probe.enabled) return undefined;

  const config: Record<string, unknown> = {};

  if (probe.type === 'httpGet' && probe.httpGet) {
    config.httpGet = {
      path: probe.httpGet.path || '/',
      port: probe.httpGet.port || 80,
      ...(probe.httpGet.scheme && { scheme: probe.httpGet.scheme }),
    };
  } else if (probe.type === 'exec' && probe.exec?.command) {
    config.exec = {
      command: parseCommandString(probe.exec.command as unknown as string),
    };
  } else if (probe.type === 'tcpSocket' && probe.tcpSocket) {
    config.tcpSocket = {
      port: probe.tcpSocket.port,
    };
  }

  if (probe.initialDelaySeconds !== undefined) config.initialDelaySeconds = probe.initialDelaySeconds;
  if (probe.periodSeconds !== undefined) config.periodSeconds = probe.periodSeconds;
  if (probe.timeoutSeconds !== undefined) config.timeoutSeconds = probe.timeoutSeconds;
  if (probe.successThreshold !== undefined) config.successThreshold = probe.successThreshold;
  if (probe.failureThreshold !== undefined) config.failureThreshold = probe.failureThreshold;

  return Object.keys(config).length > 0 ? config : undefined;
};

export const buildContainerSpec = (container: ContainerConfig): Record<string, unknown> => {
  const spec: Record<string, unknown> = {
    name: container.name || 'main',
    image: container.image || 'nginx:latest',
  };

  if (container.imagePullPolicy) {
    spec.imagePullPolicy = container.imagePullPolicy;
  }

  if (container.command) {
    const cmd = parseCommandString(container.command as unknown as string);
    if (cmd.length > 0) spec.command = cmd;
  }
  if (container.args) {
    const args = parseCommandString(container.args as unknown as string);
    if (args.length > 0) spec.args = args;
  }
  if (container.workingDir) {
    spec.workingDir = container.workingDir;
  }

  if (container.ports && container.ports.length > 0) {
    spec.ports = container.ports.map(p => ({
      ...(p.name && { name: p.name }),
      containerPort: p.containerPort,
      ...(p.protocol && p.protocol !== 'TCP' && { protocol: p.protocol }),
    }));
  }

  if (container.env && container.env.length > 0) {
    spec.env = container.env.map(e => {
      if (e.valueFrom) {
        return { name: e.name, valueFrom: e.valueFrom };
      }
      return { name: e.name, value: e.value || '' };
    });
  }

  if (container.resources) {
    const resources: Record<string, Record<string, string>> = {};
    if (container.resources.requests) {
      resources.requests = {};
      if (container.resources.requests.cpu) resources.requests.cpu = container.resources.requests.cpu;
      if (container.resources.requests.memory) resources.requests.memory = container.resources.requests.memory;
      if (container.resources.requests['ephemeral-storage']) {
        resources.requests['ephemeral-storage'] = container.resources.requests['ephemeral-storage'];
      }
    }
    if (container.resources.limits) {
      resources.limits = {};
      if (container.resources.limits.cpu) resources.limits.cpu = container.resources.limits.cpu;
      if (container.resources.limits.memory) resources.limits.memory = container.resources.limits.memory;
      if (container.resources.limits['ephemeral-storage']) {
        resources.limits['ephemeral-storage'] = container.resources.limits['ephemeral-storage'];
      }
      if (container.resources.limits['nvidia.com/gpu']) resources.limits['nvidia.com/gpu'] = container.resources.limits['nvidia.com/gpu'];
    }
    if (Object.keys(resources).length > 0) spec.resources = resources;
  }

  if (container.volumeMounts && container.volumeMounts.length > 0) {
    spec.volumeMounts = container.volumeMounts.map(vm => ({
      name: vm.name,
      mountPath: vm.mountPath,
      ...(vm.subPath && { subPath: vm.subPath }),
      ...(vm.readOnly && { readOnly: vm.readOnly }),
    }));
  }

  if (container.lifecycle) {
    const lifecycle: Record<string, unknown> = {};
    if (container.lifecycle.postStart?.exec?.command) {
      const cmd = parseCommandString(container.lifecycle.postStart.exec.command as unknown as string);
      if (cmd.length > 0) {
        lifecycle.postStart = { exec: { command: cmd } };
      }
    }
    if (container.lifecycle.preStop?.exec?.command) {
      const cmd = parseCommandString(container.lifecycle.preStop.exec.command as unknown as string);
      if (cmd.length > 0) {
        lifecycle.preStop = { exec: { command: cmd } };
      }
    }
    if (Object.keys(lifecycle).length > 0) spec.lifecycle = lifecycle;
  }

  const startupProbe = buildProbeConfig(container.startupProbe as ProbeConfig & { enabled?: boolean; type?: string });
  if (startupProbe) spec.startupProbe = startupProbe;

  const livenessProbe = buildProbeConfig(container.livenessProbe as ProbeConfig & { enabled?: boolean; type?: string });
  if (livenessProbe) spec.livenessProbe = livenessProbe;

  const readinessProbe = buildProbeConfig(container.readinessProbe as ProbeConfig & { enabled?: boolean; type?: string });
  if (readinessProbe) spec.readinessProbe = readinessProbe;

  return spec;
};

export const buildVolumeSpec = (volume: VolumeConfig): Record<string, unknown> => {
  const spec: Record<string, unknown> = {
    name: volume.name,
  };

  switch (volume.type) {
    case 'emptyDir':
      spec.emptyDir = volume.emptyDir || {};
      break;
    case 'hostPath':
      spec.hostPath = {
        path: volume.hostPath?.path || '',
        ...(volume.hostPath?.type && { type: volume.hostPath.type }),
      };
      break;
    case 'configMap':
      spec.configMap = {
        name: volume.configMap?.name || '',
        ...(volume.configMap?.items && { items: volume.configMap.items }),
        ...(volume.configMap?.defaultMode && { defaultMode: volume.configMap.defaultMode }),
      };
      break;
    case 'secret':
      spec.secret = {
        secretName: volume.secret?.secretName || '',
        ...(volume.secret?.items && { items: volume.secret.items }),
        ...(volume.secret?.defaultMode && { defaultMode: volume.secret.defaultMode }),
      };
      break;
    case 'persistentVolumeClaim':
      spec.persistentVolumeClaim = {
        claimName: volume.persistentVolumeClaim?.claimName || '',
        ...(volume.persistentVolumeClaim?.readOnly && { readOnly: volume.persistentVolumeClaim.readOnly }),
      };
      break;
  }

  return spec;
};

export const buildSchedulingSpec = (scheduling: SchedulingConfig | undefined): Record<string, unknown> | undefined => {
  if (!scheduling) return undefined;

  const affinity: Record<string, unknown> = {};

  if (scheduling.nodeAffinity) {
    const nodeAffinity: Record<string, unknown> = {};

    if (scheduling.nodeAffinity.required && scheduling.nodeAffinity.required.nodeSelectorTerms?.length > 0) {
      nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution = {
        nodeSelectorTerms: scheduling.nodeAffinity.required.nodeSelectorTerms,
      };
    }

    if (scheduling.nodeAffinity.preferred && scheduling.nodeAffinity.preferred.length > 0) {
      nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution = scheduling.nodeAffinity.preferred;
    }

    if (Object.keys(nodeAffinity).length > 0) affinity.nodeAffinity = nodeAffinity;
  }

  if (scheduling.podAffinity) {
    const podAffinity: Record<string, unknown> = {};

    if (scheduling.podAffinity.required && scheduling.podAffinity.required.length > 0) {
      podAffinity.requiredDuringSchedulingIgnoredDuringExecution = scheduling.podAffinity.required;
    }
    if (scheduling.podAffinity.preferred && scheduling.podAffinity.preferred.length > 0) {
      podAffinity.preferredDuringSchedulingIgnoredDuringExecution = scheduling.podAffinity.preferred;
    }

    if (Object.keys(podAffinity).length > 0) affinity.podAffinity = podAffinity;
  }

  if (scheduling.podAntiAffinity) {
    const podAntiAffinity: Record<string, unknown> = {};

    if (scheduling.podAntiAffinity.required && scheduling.podAntiAffinity.required.length > 0) {
      podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution = scheduling.podAntiAffinity.required;
    }
    if (scheduling.podAntiAffinity.preferred && scheduling.podAntiAffinity.preferred.length > 0) {
      podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution = scheduling.podAntiAffinity.preferred;
    }

    if (Object.keys(podAntiAffinity).length > 0) affinity.podAntiAffinity = podAntiAffinity;
  }

  return Object.keys(affinity).length > 0 ? affinity : undefined;
};
