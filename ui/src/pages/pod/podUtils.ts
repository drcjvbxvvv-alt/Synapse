import type { PodInfo } from '../../services/podService';

export interface SearchCondition {
  field: 'name' | 'namespace' | 'status' | 'podIP' | 'nodeName' | 'cpuRequest' | 'cpuLimit' | 'memoryRequest' | 'memoryLimit';
  value: string;
}

// 解析CPU值（轉換為毫核）
export const parseCpuValue = (value: string): number => {
  if (!value) return 0;
  if (value.endsWith('m')) {
    return parseInt(value.slice(0, -1), 10);
  }
  return parseFloat(value) * 1000;
};

// 格式化CPU值
export const formatCpuValue = (milliCores: number): string => {
  if (milliCores >= 1000) {
    return `${(milliCores / 1000).toFixed(1)}`;
  }
  return `${milliCores}m`;
};

// 解析記憶體值（轉換為位元組）
export const parseMemoryValue = (value: string): number => {
  if (!value) return 0;
  const units: Record<string, number> = {
    'Ki': 1024,
    'Mi': 1024 * 1024,
    'Gi': 1024 * 1024 * 1024,
    'Ti': 1024 * 1024 * 1024 * 1024,
    'K': 1000,
    'M': 1000 * 1000,
    'G': 1000 * 1000 * 1000,
    'T': 1000 * 1000 * 1000 * 1000,
  };

  for (const [unit, multiplier] of Object.entries(units)) {
    if (value.endsWith(unit)) {
      return parseFloat(value.slice(0, -unit.length)) * multiplier;
    }
  }
  return parseFloat(value);
};

// 格式化記憶體值
export const formatMemoryValue = (bytes: number): string => {
  if (bytes >= 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}Gi`;
  }
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(0)}Mi`;
  }
  if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(0)}Ki`;
  }
  return `${bytes}`;
};

// 獲取Pod的CPU和Memory資源
export const getPodResources = (pod: PodInfo) => {
  let cpuRequest = 0;
  let cpuLimit = 0;
  let memoryRequest = 0;
  let memoryLimit = 0;

  pod.containers?.forEach(container => {
    if (container.resources?.requests?.cpu) {
      cpuRequest += parseCpuValue(container.resources.requests.cpu);
    }
    if (container.resources?.limits?.cpu) {
      cpuLimit += parseCpuValue(container.resources.limits.cpu);
    }
    if (container.resources?.requests?.memory) {
      memoryRequest += parseMemoryValue(container.resources.requests.memory);
    }
    if (container.resources?.limits?.memory) {
      memoryLimit += parseMemoryValue(container.resources.limits.memory);
    }
  });

  return {
    cpuRequest: cpuRequest > 0 ? formatCpuValue(cpuRequest) : '-',
    cpuLimit: cpuLimit > 0 ? formatCpuValue(cpuLimit) : '-',
    memoryRequest: memoryRequest > 0 ? formatMemoryValue(memoryRequest) : '-',
    memoryLimit: memoryLimit > 0 ? formatMemoryValue(memoryLimit) : '-',
  };
};
