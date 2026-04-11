// ─── formDataToYAML: converts form data to Kubernetes YAML ────────────────

interface VolumeItem {
  name: string;
  type: string;
  hostPath?: string;
  configMapName?: string;
  secretName?: string;
  mountPath?: string;
  readOnly?: boolean;
  pvcName?: string;
}

// 表單資料轉YAML
export function formDataToYAML(
  workloadType: 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'Job' | 'CronJob',
  formData: Record<string, unknown>
): string {
  // 解析labels和annotations
  const parseKeyValue = (str: string): Record<string, string> => {
    if (!str) return {};
    const result: Record<string, string> = {};
    str.split(',').forEach((item) => {
      const [key, value] = item.split('=');
      if (key && value) {
        result[key.trim()] = value.trim();
      }
    });
    return result;
  };

  // 處理 labels（支援陣列和物件格式）
  let labels: Record<string, string> = {};
  if (Array.isArray(formData.labels)) {
    formData.labels.forEach((item: { key: string; value: string }) => {
      if (item.key && item.value) {
        labels[item.key] = item.value;
      }
    });
  } else if (typeof formData.labels === 'string') {
    labels = parseKeyValue(formData.labels);
  } else if (formData.labels) {
    labels = formData.labels as Record<string, string>;
  }

  // 處理 annotations（支援陣列和物件格式）
  let annotations: Record<string, string> = {};
  if (Array.isArray(formData.annotations)) {
    formData.annotations.forEach((item: { key: string; value: string }) => {
      if (item.key && item.value) {
        annotations[item.key] = item.value;
      }
    });
  } else if (typeof formData.annotations === 'string') {
    annotations = parseKeyValue(formData.annotations);
  } else if (formData.annotations) {
    annotations = formData.annotations as Record<string, string>;
  }

  // 基礎metadata - 確保 name 不為 undefined
  const workloadName = formData.name || `example-${workloadType.toLowerCase()}`;
  const metadata = {
    name: workloadName,
    namespace: formData.namespace || 'default',
    labels: Object.keys(labels).length > 0 ? labels : { app: workloadName },
    ...(Object.keys(annotations).length > 0 && { annotations }),
  };

  // 構建容器 YAML 字串的輔助函式
  const buildContainerYAML = (): string => {
    // 確保 image 不為 undefined
    const containerImage = formData.image || 'nginx:latest';
    const containerName = formData.containerName || 'main';

    let containerYAML = `      - name: ${containerName}
        image: ${containerImage}`;

    if (formData.imagePullPolicy) {
      containerYAML += `\n        imagePullPolicy: ${formData.imagePullPolicy}`;
    }

    if (formData.containerPort) {
      containerYAML += `\n        ports:\n        - containerPort: ${formData.containerPort}`;
    }

    if (formData.env && Array.isArray(formData.env) && formData.env.length > 0) {
      containerYAML += `\n        env:`;
      (formData.env as Array<{ name: string; value: string }>).forEach((e: { name: string; value: string }) => {
        containerYAML += `\n        - name: ${e.name}\n          value: "${e.value}"`;
      });
    }

    if (formData.resources) {
      const resources = formData.resources as { requests?: { cpu?: string; memory?: string }; limits?: { cpu?: string; memory?: string } };
      containerYAML += `\n        resources:`;
      if (resources.requests) {
        containerYAML += `\n          requests:`;
        if (resources.requests.cpu) {
          containerYAML += `\n            cpu: ${resources.requests.cpu}`;
        }
        if (resources.requests.memory) {
          containerYAML += `\n            memory: ${resources.requests.memory}`;
        }
      }
      if (resources.limits) {
        containerYAML += `\n          limits:`;
        if (resources.limits.cpu) {
          containerYAML += `\n            cpu: ${resources.limits.cpu}`;
        }
        if (resources.limits.memory) {
          containerYAML += `\n            memory: ${resources.limits.memory}`;
        }
      }
    }

    // 生命週期
    if (formData.lifecycle) {
      const lifecycle = formData.lifecycle as { postStart?: { exec?: { command: string | string[] } }; preStop?: { exec?: { command: string | string[] } } };
      containerYAML += `\n        lifecycle:`;
      if (lifecycle.postStart?.exec?.command) {
        const cmd = Array.isArray(lifecycle.postStart.exec.command)
          ? lifecycle.postStart.exec.command
          : lifecycle.postStart.exec.command.split(',');
        containerYAML += `\n          postStart:\n            exec:\n              command: [${cmd.map((c: string) => `"${c.trim()}"`).join(', ')}]`;
      }
      if (lifecycle.preStop?.exec?.command) {
        const cmd = Array.isArray(lifecycle.preStop.exec.command)
          ? lifecycle.preStop.exec.command
          : lifecycle.preStop.exec.command.split(',');
        containerYAML += `\n          preStop:\n            exec:\n              command: [${cmd.map((c: string) => `"${c.trim()}"`).join(', ')}]`;
      }
    }

    // 健康檢查
    if (formData.livenessProbe) {
      const livenessProbe = formData.livenessProbe as { httpGet?: { path: string; port: number }; initialDelaySeconds?: number; periodSeconds?: number; failureThreshold?: number };
      containerYAML += `\n        livenessProbe:`;
      if (livenessProbe.httpGet) {
        containerYAML += `\n          httpGet:\n            path: ${livenessProbe.httpGet.path}\n            port: ${livenessProbe.httpGet.port}`;
      }
      if (livenessProbe.initialDelaySeconds !== undefined) {
        containerYAML += `\n          initialDelaySeconds: ${livenessProbe.initialDelaySeconds}`;
      }
      if (livenessProbe.periodSeconds !== undefined) {
        containerYAML += `\n          periodSeconds: ${livenessProbe.periodSeconds}`;
      }
      if (livenessProbe.failureThreshold !== undefined) {
        containerYAML += `\n          failureThreshold: ${livenessProbe.failureThreshold}`;
      }
    }

    if (formData.readinessProbe) {
      const readinessProbe = formData.readinessProbe as { httpGet?: { path: string; port: number }; initialDelaySeconds?: number; periodSeconds?: number; failureThreshold?: number };
      containerYAML += `\n        readinessProbe:`;
      if (readinessProbe.httpGet) {
        containerYAML += `\n          httpGet:\n            path: ${readinessProbe.httpGet.path}\n            port: ${readinessProbe.httpGet.port}`;
      }
      if (readinessProbe.initialDelaySeconds !== undefined) {
        containerYAML += `\n          initialDelaySeconds: ${readinessProbe.initialDelaySeconds}`;
      }
      if (readinessProbe.periodSeconds !== undefined) {
        containerYAML += `\n          periodSeconds: ${readinessProbe.periodSeconds}`;
      }
      if (readinessProbe.failureThreshold !== undefined) {
        containerYAML += `\n          failureThreshold: ${readinessProbe.failureThreshold}`;
      }
    }

    // 安全上下文
    if (formData.securityContext) {
      const securityContext = formData.securityContext as { privileged?: boolean; runAsUser?: number; runAsGroup?: number; runAsNonRoot?: boolean; readOnlyRootFilesystem?: boolean; allowPrivilegeEscalation?: boolean };
      containerYAML += `\n        securityContext:`;
      if (securityContext.privileged !== undefined) {
        containerYAML += `\n          privileged: ${securityContext.privileged}`;
      }
      if (securityContext.runAsUser !== undefined) {
        containerYAML += `\n          runAsUser: ${securityContext.runAsUser}`;
      }
      if (securityContext.runAsGroup !== undefined) {
        containerYAML += `\n          runAsGroup: ${securityContext.runAsGroup}`;
      }
      if (securityContext.runAsNonRoot !== undefined) {
        containerYAML += `\n          runAsNonRoot: ${securityContext.runAsNonRoot}`;
      }
      if (securityContext.readOnlyRootFilesystem !== undefined) {
        containerYAML += `\n          readOnlyRootFilesystem: ${securityContext.readOnlyRootFilesystem}`;
      }
      if (securityContext.allowPrivilegeEscalation !== undefined) {
        containerYAML += `\n          allowPrivilegeEscalation: ${securityContext.allowPrivilegeEscalation}`;
      }
    }

    return containerYAML;
  };

  // 構建 PodSpec YAML 字串的輔助函式
  const buildPodSpecYAML = (): string => {
    let podSpecYAML = buildContainerYAML();

    // 資料卷掛載（新增到容器）
    if (formData.volumes && Array.isArray(formData.volumes) && formData.volumes.length > 0) {
      const volumeMounts = (formData.volumes as VolumeItem[]).map((vol) =>
        `\n        - name: ${vol.name}\n          mountPath: ${vol.mountPath}${vol.readOnly ? '\n          readOnly: true' : ''}`
      ).join('');
      podSpecYAML += `\n        volumeMounts:${volumeMounts}`;
    }

    // 映像拉取金鑰
    if (formData.imagePullSecrets && Array.isArray(formData.imagePullSecrets) && formData.imagePullSecrets.length > 0) {
      podSpecYAML += `\n      imagePullSecrets:`;
      (formData.imagePullSecrets as string[]).forEach((secret: string) => {
        podSpecYAML += `\n      - name: ${secret}`;
      });
    }

    // 節點選擇器
    if (formData.nodeSelectorList && Array.isArray(formData.nodeSelectorList) && formData.nodeSelectorList.length > 0) {
      podSpecYAML += `\n      nodeSelector:`;
      (formData.nodeSelectorList as Array<{ key: string; value: string }>).forEach((item: { key: string; value: string }) => {
        podSpecYAML += `\n        ${item.key}: ${item.value}`;
      });
    }

    // 容忍策略
    if (formData.tolerations && Array.isArray(formData.tolerations) && formData.tolerations.length > 0) {
      podSpecYAML += `\n      tolerations:`;
      (formData.tolerations as Array<{ key: string; operator: string; effect: string; value?: string; tolerationSeconds?: number }>).forEach((tol) => {
        podSpecYAML += `\n      - key: ${tol.key}\n        operator: ${tol.operator}\n        effect: ${tol.effect}`;
        if (tol.value) {
          podSpecYAML += `\n        value: ${tol.value}`;
        }
        if (tol.tolerationSeconds !== undefined) {
          podSpecYAML += `\n        tolerationSeconds: ${tol.tolerationSeconds}`;
        }
      });
    }

    // DNS配置
    if (formData.dnsPolicy) {
      podSpecYAML += `\n      dnsPolicy: ${formData.dnsPolicy}`;
    }
    if (formData.dnsConfig) {
      const dnsConfig = formData.dnsConfig as { nameservers?: string[]; searches?: string[] };
      podSpecYAML += `\n      dnsConfig:`;
      if (dnsConfig.nameservers && Array.isArray(dnsConfig.nameservers) && dnsConfig.nameservers.length > 0) {
        podSpecYAML += `\n        nameservers: [${dnsConfig.nameservers.map((ns: string) => `"${ns}"`).join(', ')}]`;
      }
      if (dnsConfig.searches && Array.isArray(dnsConfig.searches) && dnsConfig.searches.length > 0) {
        podSpecYAML += `\n        searches: [${dnsConfig.searches.map((s: string) => `"${s}"`).join(', ')}]`;
      }
    }

    // 終止寬限期
    if (formData.terminationGracePeriodSeconds !== undefined) {
      podSpecYAML += `\n      terminationGracePeriodSeconds: ${formData.terminationGracePeriodSeconds}`;
    }

    return podSpecYAML;
  };

  const buildVolumesYAML = (indent = '      '): string => {
    if (!formData.volumes || !Array.isArray(formData.volumes) || formData.volumes.length === 0) {
      return '';
    }
    return `\n${indent}volumes:` + (formData.volumes as VolumeItem[]).map((vol) => {
      let volYAML = `\n${indent}- name: ${vol.name}`;
      if (vol.type === 'emptyDir') {
        volYAML += `\n${indent}  emptyDir: {}`;
      } else if (vol.type === 'hostPath' && vol.hostPath) {
        volYAML += `\n${indent}  hostPath:\n${indent}    path: ${vol.hostPath}`;
      } else if (vol.type === 'configMap' && vol.configMapName) {
        volYAML += `\n${indent}  configMap:\n${indent}    name: ${vol.configMapName}`;
      } else if (vol.type === 'secret' && vol.secretName) {
        volYAML += `\n${indent}  secret:\n${indent}    secretName: ${vol.secretName}`;
      } else if (vol.type === 'persistentVolumeClaim' && vol.pvcName) {
        volYAML += `\n${indent}  persistentVolumeClaim:\n${indent}    claimName: ${vol.pvcName}`;
      }
      return volYAML;
    }).join('');
  };

  let yaml = '';

  switch (workloadType) {
    case 'Deployment': {
      let deploymentStrategy = '';
      if (formData.strategy) {
        const strategy = formData.strategy as { type?: string; rollingUpdate?: { maxUnavailable?: string; maxSurge?: string; minReadySeconds?: number; revisionHistoryLimit?: number; progressDeadlineSeconds?: number } };
        if (strategy.type === 'Recreate') {
          deploymentStrategy = `\n  strategy:\n    type: Recreate`;
        } else if (strategy.type === 'RollingUpdate' && strategy.rollingUpdate) {
          deploymentStrategy = `\n  strategy:\n    type: RollingUpdate\n    rollingUpdate:`;
          if (strategy.rollingUpdate.maxUnavailable) {
            deploymentStrategy += `\n      maxUnavailable: ${strategy.rollingUpdate.maxUnavailable}`;
          }
          if (strategy.rollingUpdate.maxSurge) {
            deploymentStrategy += `\n      maxSurge: ${strategy.rollingUpdate.maxSurge}`;
          }
          if (strategy.rollingUpdate.minReadySeconds !== undefined) {
            deploymentStrategy += `\n      minReadySeconds: ${strategy.rollingUpdate.minReadySeconds}`;
          }
          if (strategy.rollingUpdate.revisionHistoryLimit !== undefined) {
            deploymentStrategy += `\n      revisionHistoryLimit: ${strategy.rollingUpdate.revisionHistoryLimit}`;
          }
          if (strategy.rollingUpdate.progressDeadlineSeconds !== undefined) {
            deploymentStrategy += `\n      progressDeadlineSeconds: ${strategy.rollingUpdate.progressDeadlineSeconds}`;
          }
        }
      }

      yaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${metadata.name}
  namespace: ${metadata.namespace}
  labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `    ${k}: ${v}`)
  .join('\n')}
${Object.keys(annotations).length > 0 ? `  annotations:\n${Object.entries(annotations).map(([k, v]) => `    ${k}: ${v}`).join('\n')}` : ''}
spec:
  replicas: ${formData.replicas || 1}${deploymentStrategy}
  selector:
    matchLabels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `      ${k}: ${v}`)
  .join('\n')}
  template:
    metadata:
      labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `        ${k}: ${v}`)
  .join('\n')}
    spec:
      containers:
${buildPodSpecYAML()}${buildVolumesYAML()}`;
      break;
    }

    case 'StatefulSet':
      yaml = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${metadata.name}
  namespace: ${metadata.namespace}
  labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `    ${k}: ${v}`)
  .join('\n')}
spec:
  serviceName: ${formData.serviceName || metadata.name}
  replicas: ${formData.replicas || 1}
  selector:
    matchLabels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `      ${k}: ${v}`)
  .join('\n')}
  template:
    metadata:
      labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `        ${k}: ${v}`)
  .join('\n')}
    spec:
      containers:
${buildPodSpecYAML()}${buildVolumesYAML()}`;
      break;

    case 'DaemonSet':
      yaml = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ${metadata.name}
  namespace: ${metadata.namespace}
  labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `    ${k}: ${v}`)
  .join('\n')}
spec:
  selector:
    matchLabels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `      ${k}: ${v}`)
  .join('\n')}
  template:
    metadata:
      labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `        ${k}: ${v}`)
  .join('\n')}
    spec:
      containers:
${buildPodSpecYAML()}${buildVolumesYAML()}`;
      break;

    case 'Rollout':
      yaml = `apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: ${metadata.name}
  namespace: ${metadata.namespace}
  labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `    ${k}: ${v}`)
  .join('\n')}
spec:
  replicas: ${formData.replicas || 1}
  selector:
    matchLabels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `      ${k}: ${v}`)
  .join('\n')}
  template:
    metadata:
      labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `        ${k}: ${v}`)
  .join('\n')}
    spec:
      containers:
${buildPodSpecYAML()}${buildVolumesYAML()}
  strategy:
    canary:
      steps:
      - setWeight: 20
      - pause: {duration: 10s}
      - setWeight: 50
      - pause: {duration: 10s}`;
      break;

    case 'Job':
      yaml = `apiVersion: batch/v1
kind: Job
metadata:
  name: ${metadata.name}
  namespace: ${metadata.namespace}
  labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `    ${k}: ${v}`)
  .join('\n')}
spec:
${formData.completions ? `  completions: ${formData.completions}` : ''}
${formData.parallelism ? `  parallelism: ${formData.parallelism}` : ''}
${formData.backoffLimit !== undefined ? `  backoffLimit: ${formData.backoffLimit}` : ''}
  template:
    metadata:
      labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `        ${k}: ${v}`)
  .join('\n')}
    spec:
      containers:
${buildPodSpecYAML()}${buildVolumesYAML()}
      restartPolicy: Never`;
      break;

    case 'CronJob':
      yaml = `apiVersion: batch/v1
kind: CronJob
metadata:
  name: ${metadata.name}
  namespace: ${metadata.namespace}
  labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `    ${k}: ${v}`)
  .join('\n')}
spec:
  schedule: "${formData.schedule || '0 0 * * *'}"
${formData.suspend !== undefined ? `  suspend: ${formData.suspend}` : ''}
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
${Object.entries(metadata.labels)
  .map(([k, v]) => `            ${k}: ${v}`)
  .join('\n')}
        spec:
          containers:
${buildPodSpecYAML().replace(/ {6}/g, '          ')}${buildVolumesYAML('          ')}
          restartPolicy: OnFailure`;
      break;

    default:
      throw new Error(`不支援的工作負載型別: ${workloadType}`);
  }

  return yaml;
}
