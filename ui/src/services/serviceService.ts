import { request } from '../utils/api';
import type { Service, Endpoints, ApiResponse, PaginatedResponse } from '../types';

export type ServiceListResponse = ApiResponse<PaginatedResponse<Service>>;

export type ServiceDetailResponse = ApiResponse<Service>;

export type ServiceYAMLResponse = ApiResponse<{ yaml: string }>;

export type EndpointsResponse = ApiResponse<Endpoints>;

export class ServiceService {
  // 獲取Service列表
  static async getServices(
    clusterId: string,
    namespace?: string,
    type?: string,
    search?: string,
    page = 1,
    pageSize = 20
  ): Promise<ServiceListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });
    
    if (namespace && namespace !== '_all_') {
      params.append('namespace', namespace);
    }
    
    if (type) {
      params.append('type', type);
    }
    
    if (search) {
      params.append('search', search);
    }
    
    return request.get(`/clusters/${clusterId}/services?${params}`);
  }

  // 獲取Service詳情
  static async getService(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<ServiceDetailResponse> {
    return request.get(`/clusters/${clusterId}/services/${namespace}/${name}`);
  }

  // 獲取Service的YAML
  static async getServiceYAML(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<ServiceYAMLResponse> {
    return request.get(`/clusters/${clusterId}/services/${namespace}/${name}/yaml`);
  }

  // 獲取Service的Endpoints
  static async getServiceEndpoints(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<EndpointsResponse> {
    return request.get(`/clusters/${clusterId}/services/${namespace}/${name}/endpoints`);
  }

  // 刪除Service
  static async deleteService(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<ApiResponse<null>> {
    return request.delete(`/clusters/${clusterId}/services/${namespace}/${name}`);
  }

  // 建立Service
  static async createService(
    clusterId: string,
    data: {
      namespace: string;
      yaml?: string;
      formData?: Record<string, unknown>;
    }
  ): Promise<ApiResponse<Service>> {
    return request.post(`/clusters/${clusterId}/services`, data);
  }

  // 更新Service
  static async updateService(
    clusterId: string,
    namespace: string,
    name: string,
    data: {
      namespace: string;
      yaml?: string;
      formData?: Record<string, unknown>;
    }
  ): Promise<ApiResponse<Service>> {
    return request.put(`/clusters/${clusterId}/services/${namespace}/${name}`, data);
  }

  // 獲取Service型別顏色
  static getTypeColor(type: string): string {
    switch (type) {
      case 'ClusterIP':
        return 'blue';
      case 'NodePort':
        return 'green';
      case 'LoadBalancer':
        return 'purple';
      case 'ExternalName':
        return 'orange';
      default:
        return 'default';
    }
  }

  // 獲取Service型別標籤
  static getTypeTag(type: string): string {
    switch (type) {
      case 'ClusterIP':
        return '叢集內訪問';
      case 'NodePort':
        return '節點訪問';
      case 'LoadBalancer':
        return '負載均衡';
      case 'ExternalName':
        return '外部名稱';
      default:
        return type;
    }
  }

  // 格式化連接埠資訊
  static formatPorts(service: Service): string {
    if (!service.ports || service.ports.length === 0) {
      return '-';
    }

    return service.ports.map(port => {
      let portStr = `${port.port}`;
      if (port.name) {
        portStr = `${port.name}:${portStr}`;
      }
      if (port.nodePort) {
        portStr += `:${port.nodePort}`;
      }
      if (port.protocol && port.protocol !== 'TCP') {
        portStr += `/${port.protocol}`;
      }
      return portStr;
    }).join(', ');
  }

  // 格式化訪問地址
  static formatAccessAddress(service: Service): string[] {
    const addresses: string[] = [];

    // ClusterIP
    if (service.clusterIP && service.clusterIP !== 'None') {
      addresses.push(service.clusterIP);
    }

    // ExternalIPs
    if (service.externalIPs && service.externalIPs.length > 0) {
      addresses.push(...service.externalIPs);
    }

    // LoadBalancer
    if (service.loadBalancerIngress && service.loadBalancerIngress.length > 0) {
      service.loadBalancerIngress.forEach(lb => {
        if (lb.ip) {
          addresses.push(lb.ip);
        }
        if (lb.hostname) {
          addresses.push(lb.hostname);
        }
      });
    }

    // LoadBalancerIP
    if (service.loadBalancerIP) {
      addresses.push(service.loadBalancerIP);
    }

    // ExternalName
    if (service.externalName) {
      addresses.push(service.externalName);
    }

    return addresses.length > 0 ? addresses : ['-'];
  }

  // 格式化選擇器
  static formatSelector(selector: Record<string, string>): string {
    if (!selector || Object.keys(selector).length === 0) {
      return '-';
    }
    return Object.entries(selector)
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
  }

  // 獲取年齡顯示
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

  // 獲取Service命名空間列表（帶計數）
  static async getServiceNamespaces(clusterId: string): Promise<{ name: string; count: number }[]> {
    return request.get<{ name: string; count: number }[]>(
      `/clusters/${clusterId}/services/namespaces`
    );
  }
}

