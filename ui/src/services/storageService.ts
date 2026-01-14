import { request } from '../utils/api';
import type { PVC, PV, StorageClass, ApiResponse, PaginatedResponse } from '../types';

export type PVCListResponse = ApiResponse<PaginatedResponse<PVC>>;
export type PVCDetailResponse = ApiResponse<PVC>;
export type PVCYAMLResponse = ApiResponse<{ yaml: string }>;

export type PVListResponse = ApiResponse<PaginatedResponse<PV>>;
export type PVDetailResponse = ApiResponse<PV>;
export type PVYAMLResponse = ApiResponse<{ yaml: string }>;

export type StorageClassListResponse = ApiResponse<PaginatedResponse<StorageClass>>;
export type StorageClassDetailResponse = ApiResponse<StorageClass>;
export type StorageClassYAMLResponse = ApiResponse<{ yaml: string }>;

export class StorageService {
  // ==================== PVC 相关方法 ====================

  // 获取PVC列表
  static async getPVCs(
    clusterId: string,
    namespace?: string,
    status?: string,
    search?: string,
    page = 1,
    pageSize = 20
  ): Promise<PVCListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });
    
    if (namespace && namespace !== '_all_') {
      params.append('namespace', namespace);
    }
    
    if (status) {
      params.append('status', status);
    }
    
    if (search) {
      params.append('search', search);
    }
    
    return request.get(`/clusters/${clusterId}/pvcs?${params}`);
  }

  // 获取PVC详情
  static async getPVC(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<PVCDetailResponse> {
    return request.get(`/clusters/${clusterId}/pvcs/${namespace}/${name}`);
  }

  // 获取PVC的YAML
  static async getPVCYAML(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<PVCYAMLResponse> {
    return request.get(`/clusters/${clusterId}/pvcs/${namespace}/${name}/yaml`);
  }

  // 删除PVC
  static async deletePVC(
    clusterId: string,
    namespace: string,
    name: string
  ): Promise<ApiResponse<null>> {
    return request.delete(`/clusters/${clusterId}/pvcs/${namespace}/${name}`);
  }

  // 获取PVC所在的命名空间列表
  static async getPVCNamespaces(
    clusterId: string
  ): Promise<ApiResponse<{ name: string; count: number }[]>> {
    return request.get(`/clusters/${clusterId}/pvcs/namespaces`);
  }

  // ==================== PV 相关方法 ====================

  // 获取PV列表
  static async getPVs(
    clusterId: string,
    status?: string,
    search?: string,
    page = 1,
    pageSize = 20
  ): Promise<PVListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });
    
    if (status) {
      params.append('status', status);
    }
    
    if (search) {
      params.append('search', search);
    }
    
    return request.get(`/clusters/${clusterId}/pvs?${params}`);
  }

  // 获取PV详情
  static async getPV(
    clusterId: string,
    name: string
  ): Promise<PVDetailResponse> {
    return request.get(`/clusters/${clusterId}/pvs/${name}`);
  }

  // 获取PV的YAML
  static async getPVYAML(
    clusterId: string,
    name: string
  ): Promise<PVYAMLResponse> {
    return request.get(`/clusters/${clusterId}/pvs/${name}/yaml`);
  }

  // 删除PV
  static async deletePV(
    clusterId: string,
    name: string
  ): Promise<ApiResponse<null>> {
    return request.delete(`/clusters/${clusterId}/pvs/${name}`);
  }

  // ==================== StorageClass 相关方法 ====================

  // 获取StorageClass列表
  static async getStorageClasses(
    clusterId: string,
    search?: string,
    page = 1,
    pageSize = 20
  ): Promise<StorageClassListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });
    
    if (search) {
      params.append('search', search);
    }
    
    return request.get(`/clusters/${clusterId}/storageclasses?${params}`);
  }

  // 获取StorageClass详情
  static async getStorageClass(
    clusterId: string,
    name: string
  ): Promise<StorageClassDetailResponse> {
    return request.get(`/clusters/${clusterId}/storageclasses/${name}`);
  }

  // 获取StorageClass的YAML
  static async getStorageClassYAML(
    clusterId: string,
    name: string
  ): Promise<StorageClassYAMLResponse> {
    return request.get(`/clusters/${clusterId}/storageclasses/${name}/yaml`);
  }

  // 删除StorageClass
  static async deleteStorageClass(
    clusterId: string,
    name: string
  ): Promise<ApiResponse<null>> {
    return request.delete(`/clusters/${clusterId}/storageclasses/${name}`);
  }

  // ==================== 格式化工具函数 ====================

  // 格式化访问模式
  static formatAccessModes(modes: string[]): string {
    if (!modes || modes.length === 0) return '-';
    const shortNames: Record<string, string> = {
      'ReadWriteOnce': 'RWO',
      'ReadOnlyMany': 'ROX',
      'ReadWriteMany': 'RWX',
      'ReadWriteOncePod': 'RWOP',
    };
    return modes.map(mode => shortNames[mode] || mode).join(', ');
  }

  // 获取PVC状态颜色
  static getPVCStatusColor(status: string): string {
    const colorMap: Record<string, string> = {
      'Bound': 'green',
      'Pending': 'orange',
      'Lost': 'red',
    };
    return colorMap[status] || 'default';
  }

  // 获取PV状态颜色
  static getPVStatusColor(status: string): string {
    const colorMap: Record<string, string> = {
      'Available': 'blue',
      'Bound': 'green',
      'Released': 'orange',
      'Failed': 'red',
    };
    return colorMap[status] || 'default';
  }

  // 获取回收策略颜色
  static getReclaimPolicyColor(policy: string): string {
    const colorMap: Record<string, string> = {
      'Retain': 'blue',
      'Recycle': 'orange',
      'Delete': 'red',
    };
    return colorMap[policy] || 'default';
  }

  // 格式化容量显示
  static formatCapacity(capacity: string): string {
    if (!capacity) return '-';
    return capacity;
  }

  // 格式化ClaimRef显示
  static formatClaimRef(claimRef?: { namespace: string; name: string }): string {
    if (!claimRef) return '-';
    return `${claimRef.namespace}/${claimRef.name}`;
  }
}
