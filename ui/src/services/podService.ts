import { request } from '../utils/api';
import { buildWebSocketUrl } from '../utils/wsUrl';
import type { ApiResponse } from '../types';

export interface ContainerInfo {
  name: string;
  image: string;
  ready: boolean;
  restartCount: number;
  state: {
    state: string;
    reason?: string;
    message?: string;
    startedAt?: string;
  };
  resources: {
    requests: Record<string, string>;
    limits: Record<string, string>;
  };
  ports: Array<{
    name?: string;
    containerPort: number;
    protocol: string;
  }>;
}

export interface PodCondition {
  type: string;
  status: string;
  lastProbeTime?: string;
  lastTransitionTime: string;
  reason?: string;
  message?: string;
}

export interface PodInfo {
  name: string;
  namespace: string;
  status: string;
  phase: string;
  nodeName: string;
  podIP: string;
  hostIP: string;
  restartCount: number;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  ownerReferences: Array<{
    kind: string;
    name: string;
    uid: string;
    controller?: boolean;
  }>;
  containers: ContainerInfo[];
  initContainers: ContainerInfo[];
  conditions: PodCondition[];
  qosClass: string;
  serviceAccount: string;
  priority?: number;
  priorityClassName?: string;
}

export interface PodListResponse {
  items: PodInfo[];
  total: number;
  page: number;
  pageSize: number;
}

export interface PodDetailResponse {
  pod: PodInfo;
  raw: Record<string, unknown>;
}

export interface PodLogsResponse {
  logs: string;
}

export class PodService {
  // 獲取Pod列表
  static async getPods(
    clusterId: string,
    namespace?: string,
    nodeName?: string,
    labelSelector?: string,
    fieldSelector?: string,
    search?: string,
    page = 1,
    pageSize = 20
  ): Promise<PodListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });
    
    if (namespace) {
      params.append('namespace', namespace);
    }
    
    if (nodeName) {
      params.append('nodeName', nodeName);
    }
    
    if (labelSelector) {
      params.append('labelSelector', labelSelector);
    }
    
    if (fieldSelector) {
      params.append('fieldSelector', fieldSelector);
    }
    
    if (search) {
      params.append('search', search);
    }
    
    return request.get(`/clusters/${clusterId}/pods?${params}`);
  }

  // 獲取Pod詳情
  static async getPodDetail(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<PodDetailResponse> {
    return request.get(`/clusters/${clusterId}/pods/${namespace}/${name}`);
  }

  // 刪除Pod
  static async deletePod(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<ApiResponse<unknown>> {
    return request.delete(`/clusters/${clusterId}/pods/${namespace}/${name}`);
  }

  // 批次刪除Pod
  static async batchDeletePods(
    clusterId: string,
    pods: Array<{ namespace: string; name: string }>
  ): Promise<Array<{ namespace: string; name: string; success: boolean; error?: string }>> {
    const results = await Promise.allSettled(
      pods.map(pod => this.deletePod(clusterId, pod.namespace, pod.name))
    );
    
    return results.map((result, index) => ({
      namespace: pods[index].namespace,
      name: pods[index].name,
      success: result.status === 'fulfilled',
      error: result.status === 'rejected' ? String(result.reason) : undefined,
    }));
  }

  // 獲取Pod日誌
  static async getPodLogs(
    clusterId: string,
    namespace: string,
    name: string,
    container?: string,
    follow = false,
    previous = false,
    tailLines?: number,
    sinceSeconds?: number
  ): Promise<PodLogsResponse> {
    const params = new URLSearchParams();
    
    if (container) {
      params.append('container', container);
    }
    
    if (follow) {
      params.append('follow', 'true');
    }
    
    if (previous) {
      params.append('previous', 'true');
    }
    
    if (tailLines) {
      params.append('tailLines', tailLines.toString());
    }
    
    if (sinceSeconds) {
      params.append('sinceSeconds', sinceSeconds.toString());
    }
    
    return request.get(`/clusters/${clusterId}/pods/${namespace}/${name}/logs?${params}`);
  }

  // 獲取Pod狀態顏色
  static getStatusColor(pod: PodInfo): string {
    const { status, phase } = pod;
    
    if (status.includes('Terminating')) return 'orange';
    if (status === 'Running') return 'green';
    if (status === 'Completed') return 'blue';
    if (status === 'Failed') return 'red';
    if (status === 'Pending') return 'orange';
    if (status.includes('Error') || status.includes('BackOff')) return 'red';
    if (status.includes('NotReady')) return 'orange';
    
    // 根據phase判斷
    switch (phase) {
      case 'Running':
        return 'green';
      case 'Succeeded':
        return 'blue';
      case 'Failed':
        return 'red';
      case 'Pending':
        return 'orange';
      default:
        return 'default';
    }
  }

  // 格式化Pod狀態
  static formatStatus(pod: PodInfo): { status: string; color: string } {
    const color = this.getStatusColor(pod);
    let statusText = pod.status;
    
    // 簡化狀態顯示
    if (statusText.includes('NotReady')) {
      const match = statusText.match(/\((\d+\/\d+)\)/);
      if (match) {
        statusText = `未就緒 ${match[1]}`;
      } else {
        statusText = '未就緒';
      }
    }
    
    return { status: statusText, color };
  }

  // 獲取Pod年齡
  static getAge(createdAt: string): string {
    const now = new Date();
    const created = new Date(createdAt);
    const diffMs = now.getTime() - created.getTime();
    
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    const diffHours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
    const diffMinutes = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
    
    if (diffDays > 0) {
      return `${diffDays}天`;
    } else if (diffHours > 0) {
      return `${diffHours}小時`;
    } else if (diffMinutes > 0) {
      return `${diffMinutes}分鐘`;
    } else {
      return '剛剛';
    }
  }

  // 獲取容器狀態顏色
  static getContainerStatusColor(container: ContainerInfo): string {
    if (!container.ready) return 'red';
    if (container.state.state === 'Running') return 'green';
    if (container.state.state === 'Waiting') return 'orange';
    if (container.state.state === 'Terminated') return 'red';
    return 'default';
  }

  // 格式化容器狀態
  static formatContainerStatus(container: ContainerInfo): string {
    if (container.state.state === 'Running') return '執行中';
    if (container.state.state === 'Waiting') {
      return container.state.reason || '等待中';
    }
    if (container.state.state === 'Terminated') {
      return container.state.reason || '已終止';
    }
    return container.state.state;
  }

  // 獲取Pod命名空間列表
  static async getPodNamespaces(clusterId: string): Promise<string[]> {
    return request.get(`/clusters/${clusterId}/pods/namespaces`);
  }

  // 獲取Pod節點列表
  static async getPodNodes(clusterId: string): Promise<string[]> {
    return request.get(`/clusters/${clusterId}/pods/nodes`);
  }

  // 建立WebSocket連線獲取實時日誌流
  static createLogStream(
    clusterId: string,
    namespace: string,
    name: string,
    options: {
      container?: string;
      previous?: boolean;
      tailLines?: number;
      sinceSeconds?: number;
    } = {}
  ): WebSocket {
    const params = new URLSearchParams();
    if (options.container) {
      params.append('container', options.container);
    }
    if (options.previous) {
      params.append('previous', 'true');
    }
    if (options.tailLines) {
      params.append('tailLines', options.tailLines.toString());
    }
    if (options.sinceSeconds) {
      params.append('sinceSeconds', options.sinceSeconds.toString());
    }

    const url = buildWebSocketUrl(
      `/ws/clusters/${clusterId}/pods/${namespace}/${name}/logs?${params}`
    );
    
    return new WebSocket(url);
  }
}