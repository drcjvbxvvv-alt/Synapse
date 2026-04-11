import { request } from '../../utils/api';
import type { ApiResponse } from '../../types';
import type { WorkloadInfo, WorkloadListResponse, WorkloadDetailResponse } from './types';
import { formDataToYAML } from './formDataToYAML';

export class WorkloadService {
  // 檢查叢集是否安裝了 Argo Rollouts CRD
  static async checkRolloutCRD(
    clusterId: string
  ): Promise<{ enabled: boolean }> {
    return request.get(`/clusters/${clusterId}/rollouts/crd-check`);
  }

  // 獲取工作負載列表
  static async getWorkloads(
    clusterId: string,
    namespace?: string,
    workloadType?: string,
    page = 1,
    pageSize = 20,
    search?: string
  ): Promise<WorkloadListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });

    if (namespace) {
      params.append('namespace', namespace);
    }

    if (search) {
      params.append('search', search);
    }

    // 根據workloadType路由到不同的後端API端點
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += 'deployments';
        break;
      case 'Rollout':
        endpoint += 'rollouts';
        break;
      case 'StatefulSet':
        endpoint += 'statefulsets';
        break;
      case 'DaemonSet':
        endpoint += 'daemonsets';
        params.append('type', 'DaemonSet'); // 臨時保留
        break;
      case 'Job':
        endpoint += 'jobs';
        params.append('type', 'Job'); // 臨時保留
        break;
      case 'CronJob':
        endpoint += 'cronjobs';
        params.append('type', 'CronJob'); // 臨時保留
        break;
      default:
        endpoint += 'workloads';
        if (workloadType) {
          params.append('type', workloadType);
        }
    }

    return request.get(`${endpoint}?${params}`);
  }

  // 獲取工作負載命名空間列表
  static async getWorkloadNamespaces(
    clusterId: string,
    workloadType?: string
  ): Promise<Array<{ name: string; count: number }>> {
    // 根據workloadType路由到不同的後端API端點
    let endpoint = `/clusters/${clusterId}/`;
    const params = new URLSearchParams();

    switch (workloadType) {
      case 'Deployment':
        endpoint += 'deployments/namespaces';
        break;
      case 'Rollout':
        endpoint += 'rollouts/namespaces';
        break;
      case 'StatefulSet':
        endpoint += 'statefulsets/namespaces';
        break;
      case 'DaemonSet':
        endpoint += 'daemonsets/namespaces';
        params.append('type', 'DaemonSet');
        break;
      case 'Job':
        endpoint += 'jobs/namespaces';
        params.append('type', 'Job');
        break;
      case 'CronJob':
        endpoint += 'cronjobs/namespaces';
        params.append('type', 'CronJob');
        break;
      default:
        endpoint += 'workloads/namespaces';
        if (workloadType) {
          params.append('type', workloadType);
        }
    }

    return request.get(`${endpoint}?${params}`);
  }

  // 獲取工作負載詳情
  static async getWorkloadDetail(
    clusterId: string,
    workloadType: string,
    namespace: string,
    name: string
  ): Promise<WorkloadDetailResponse> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${name}`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${name}`;
        break;
      case 'StatefulSet':
        endpoint += `statefulsets/${namespace}/${name}`;
        break;
      case 'DaemonSet':
        endpoint += `daemonsets/${namespace}/${name}?type=${workloadType}`;
        break;
      case 'Job':
        endpoint += `jobs/${namespace}/${name}?type=${workloadType}`;
        break;
      case 'CronJob':
        endpoint += `cronjobs/${namespace}/${name}?type=${workloadType}`;
        break;
      default:
        endpoint += `workloads/${namespace}/${name}?type=${workloadType}`;
    }
    return request.get(endpoint);
  }

  // 擴縮容工作負載
  static async scaleWorkload(
    clusterId: string,
    namespace: string,
    name: string,
    type: string,
    replicas: number
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (type) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${name}/scale`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${name}/scale`;
        break;
      case 'StatefulSet':
        endpoint += `statefulsets/${namespace}/${name}/scale`;
        break;
      default:
        endpoint += `workloads/${namespace}/${name}/scale?type=${type}`;
    }
    return request.post(endpoint, { replicas });
  }

  // 刪除工作負載
  static async deleteWorkload(
    clusterId: string,
    namespace: string,
    name: string,
    type: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (type) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${name}`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${name}`;
        break;
      case 'StatefulSet':
        endpoint += `statefulsets/${namespace}/${name}`;
        break;
      case 'DaemonSet':
        endpoint += `daemonsets/${namespace}/${name}`;
        break;
      case 'Job':
        endpoint += `jobs/${namespace}/${name}`;
        break;
      case 'CronJob':
        endpoint += `cronjobs/${namespace}/${name}`;
        break;
      default:
        endpoint += `workloads/${namespace}/${name}?type=${type}`;
    }
    return request.delete(endpoint);
  }

  // Argo Rollouts 操控
  static async promoteRollout(clusterId: string, namespace: string, name: string): Promise<ApiResponse<unknown>> {
    return request.post(`/clusters/${clusterId}/rollouts/${namespace}/${name}/promote`, {});
  }

  static async promoteFullRollout(clusterId: string, namespace: string, name: string): Promise<ApiResponse<unknown>> {
    return request.post(`/clusters/${clusterId}/rollouts/${namespace}/${name}/promote-full`, {});
  }

  static async abortRollout(clusterId: string, namespace: string, name: string): Promise<ApiResponse<unknown>> {
    return request.post(`/clusters/${clusterId}/rollouts/${namespace}/${name}/abort`, {});
  }

  static async getRolloutAnalysisRuns(clusterId: string, namespace: string, name: string): Promise<ApiResponse<unknown>> {
    return request.get(`/clusters/${clusterId}/rollouts/${namespace}/${name}/analysis-runs`);
  }

  // 重新部署工作負載（重啟）
  static async restartWorkload(
    clusterId: string,
    namespace: string,
    name: string,
    type: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (type) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${name}/restart`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${name}/restart`;
        break;
      case 'StatefulSet':
        endpoint += `statefulsets/${namespace}/${name}/restart`;
        break;
      case 'DaemonSet':
        endpoint += `daemonsets/${namespace}/${name}/restart`;
        break;
      default:
        endpoint += `workloads/${namespace}/${name}/restart?type=${type}`;
    }
    return request.post(endpoint);
  }

  // 應用YAML
  static async applyYAML(
    clusterId: string,
    yaml: string,
    dryRun = false
  ): Promise<ApiResponse<unknown>> {
    // 解析YAML中的kind來確定使用哪個endpoint
    try {
      const kindMatch = yaml.match(/kind:\s*(\w+)/);
      if (kindMatch) {
        const kind = kindMatch[1];
        let endpoint = `/clusters/${clusterId}/`;
        switch (kind) {
          case 'Deployment':
            endpoint += 'deployments/yaml/apply';
            break;
          case 'Rollout':
            endpoint += 'rollouts/yaml/apply';
            break;
          case 'StatefulSet':
            endpoint += 'statefulsets/yaml/apply';
            break;
          case 'DaemonSet':
            endpoint += 'daemonsets/yaml/apply';
            break;
          case 'Job':
            endpoint += 'jobs/yaml/apply';
            break;
          case 'CronJob':
            endpoint += 'cronjobs/yaml/apply';
            break;
          default:
            endpoint += 'workloads/yaml/apply';
        }
        return request.post(endpoint, { yaml, dryRun });
      }
    } catch {
      // fallback to default
    }
    return request.post(`/clusters/${clusterId}/workloads/yaml/apply`, {
      yaml,
      dryRun,
    });
  }

  // 獲取工作負載型別列表
  static getWorkloadTypes(): Array<{ value: string; label: string; icon: string }> {
    return [
      { value: 'deployment', label: 'Deployment', icon: '🚀' },
      { value: 'argo-rollout', label: 'Argo Rollout', icon: '🌀' },
      { value: 'statefulset', label: 'StatefulSet', icon: '💾' },
      { value: 'daemonset', label: 'DaemonSet', icon: '👥' },
      { value: 'job', label: 'Job', icon: '⚡' },
      { value: 'cronjob', label: 'CronJob', icon: '⏰' },
    ];
  }

  // 獲取工作負載狀態顏色
  static getStatusColor(workload: WorkloadInfo): string {
    const { type, status, replicas, readyReplicas } = workload;

    if (type === 'job' || type === 'cronjob') {
      return status === 'Completed' ? 'success' : 'processing';
    }

    // 如果有副本數資訊，使用副本數判斷
    if (typeof replicas === 'number' && typeof readyReplicas === 'number') {
      if (readyReplicas === 0) return 'error';
      if (readyReplicas < replicas) return 'warning';
      return 'success';
    }

    // 根據狀態欄位判斷
    if (status === 'Ready') return 'success';
    if (status === 'NotReady') return 'error';
    return 'processing';
  }

  // 格式化工作負載狀態
  static formatStatus(workload: WorkloadInfo): { status: string; color: string } {
    const { type, status, replicas, readyReplicas } = workload;
    const color = this.getStatusColor(workload);

    let statusText = status || '未知';

    if (type === 'job') {
      statusText = status === 'Completed' ? '已完成' : '執行中';
    } else if (type === 'cronjob') {
      statusText = '已排程';
    } else if (typeof replicas === 'number' && typeof readyReplicas === 'number') {
      statusText = `${readyReplicas}/${replicas}`;
    }

    return { status: statusText, color };
  }

  // 表單資料轉YAML — delegates to standalone function
  static formDataToYAML(
    workloadType: 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'Job' | 'CronJob',
    formData: Record<string, unknown>
  ): string {
    return formDataToYAML(workloadType, formData);
  }

  // 獲取Deployment關聯的Pods
  static async getWorkloadPods(
    clusterId: string,
    namespace: string,
    workloadType: string,
    workloadName: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${workloadName}/pods`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${workloadName}/pods`;
        break;
      default:
        endpoint += `workloads/${workloadType}/${namespace}/${workloadName}/pods`;
    }
    return request.get(endpoint);
  }

  // 獲取Deployment關聯的Services
  static async getWorkloadServices(
    clusterId: string,
    namespace: string,
    workloadType: string,
    workloadName: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${workloadName}/services`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${workloadName}/services`;
        break;
      default:
        endpoint += `workloads/${workloadType}/${namespace}/${workloadName}/services`;
    }
    return request.get(endpoint);
  }

  // 獲取Deployment關聯的Ingresses
  static async getWorkloadIngresses(
    clusterId: string,
    namespace: string,
    workloadType: string,
    workloadName: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${workloadName}/ingresses`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${workloadName}/ingresses`;
        break;
      default:
        endpoint += `workloads/${workloadType}/${namespace}/${workloadName}/ingresses`;
    }
    return request.get(endpoint);
  }

  // 獲取Deployment的HPA
  static async getWorkloadHPA(
    clusterId: string,
    namespace: string,
    workloadType: string,
    workloadName: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${workloadName}/hpa`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${workloadName}/hpa`;
        break;
      default:
        endpoint += `workloads/${workloadType}/${namespace}/${workloadName}/hpa`;
    }
    return request.get(endpoint);
  }

  // HPA CRUD
  static async createHPA(clusterId: string, data: {
    name: string; namespace: string; targetKind: string; targetName: string;
    minReplicas: number; maxReplicas: number;
    cpuTargetUtilization?: number; memTargetUtilization?: number;
  }): Promise<ApiResponse<unknown>> {
    return request.post(`/clusters/${clusterId}/hpa`, data);
  }

  static async updateHPA(clusterId: string, namespace: string, name: string, data: {
    name: string; namespace: string; targetKind: string; targetName: string;
    minReplicas: number; maxReplicas: number;
    cpuTargetUtilization?: number; memTargetUtilization?: number;
  }): Promise<ApiResponse<unknown>> {
    return request.put(`/clusters/${clusterId}/hpa/${namespace}/${name}`, data);
  }

  static async deleteHPA(clusterId: string, namespace: string, name: string): Promise<ApiResponse<unknown>> {
    return request.delete(`/clusters/${clusterId}/hpa/${namespace}/${name}`);
  }

  // 獲取Deployment的ReplicaSets
  static async getWorkloadReplicaSets(
    clusterId: string,
    namespace: string,
    workloadType: string,
    workloadName: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${workloadName}/replicasets`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${workloadName}/replicasets`;
        break;
      default:
        endpoint += `workloads/${workloadType}/${namespace}/${workloadName}/replicasets`;
    }
    return request.get(endpoint);
  }

  // 獲取Deployment的Events
  static async getWorkloadEvents(
    clusterId: string,
    namespace: string,
    workloadType: string,
    workloadName: string
  ): Promise<ApiResponse<unknown>> {
    let endpoint = `/clusters/${clusterId}/`;
    switch (workloadType) {
      case 'Deployment':
        endpoint += `deployments/${namespace}/${workloadName}/events`;
        break;
      case 'Rollout':
        endpoint += `rollouts/${namespace}/${workloadName}/events`;
        break;
      default:
        endpoint += `workloads/${workloadType}/${namespace}/${workloadName}/events`;
    }
    return request.get(endpoint);
  }
}
