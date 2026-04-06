import { request } from '../utils/api';

export interface ConfigMapListItem {
  name: string;
  namespace: string;
  labels: Record<string, string>;
  dataCount: number;
  creationTimestamp: string;
  age: string;
}

export interface ConfigMapDetail {
  name: string;
  namespace: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  data: Record<string, string>;
  binaryData?: Record<string, Uint8Array>;
  creationTimestamp: string;
  age: string;
  resourceVersion: string;
}

export interface SecretListItem {
  name: string;
  namespace: string;
  type: string;
  labels: Record<string, string>;
  dataCount: number;
  creationTimestamp: string;
  age: string;
}

export interface SecretDetail {
  name: string;
  namespace: string;
  type: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  data: Record<string, string>;
  creationTimestamp: string;
  age: string;
  resourceVersion: string;
}

export interface NamespaceItem {
  name: string;
  count: number;
}

export interface ConfigVersion {
  id: number;
  clusterId: number;
  resourceType: string;
  namespace: string;
  name: string;
  version: number;
  contentJSON: string;
  changedBy: string;
  changedAt: string;
}

export interface ListResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

// 命名空間物件介面
interface NamespaceObject {
  name: string;
  status: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  creationTimestamp: string;
}

// 獲取命名空間列表（通用）
export const getNamespaces = async (clusterId: number): Promise<string[]> => {
  try {
    // 直接使用叢集的命名空間介面
    const response = await request.get<NamespaceObject[]>(
      `/clusters/${clusterId}/namespaces`
    );
    // 提取命名空間名稱陣列
    return response.map((ns) => ns.name);
  } catch (error) {
    console.error('獲取命名空間列表失敗:', error);
    // 返回預設命名空間
    return ['default', 'kube-system', 'kube-public', 'kube-node-lease'];
  }
};

// ConfigMap API
export const configMapService = {
  // 獲取ConfigMap列表
  async getConfigMaps(
    clusterId: number,
    params: {
      namespace?: string;
      name?: string;
      page?: number;
      pageSize?: number;
    }
  ): Promise<ListResponse<ConfigMapListItem>> {
    const queryParams = new URLSearchParams();
    if (params.namespace) queryParams.append('namespace', params.namespace);
    if (params.name) queryParams.append('name', params.name);
    if (params.page) queryParams.append('page', params.page.toString());
    if (params.pageSize) queryParams.append('pageSize', params.pageSize.toString());

    return request.get<ListResponse<ConfigMapListItem>>(
      `/clusters/${clusterId}/configmaps?${queryParams}`
    );
  },

  // 獲取ConfigMap詳情
  async getConfigMap(
    clusterId: number,
    namespace: string,
    name: string
  ): Promise<ConfigMapDetail> {
    return request.get<ConfigMapDetail>(
      `/clusters/${clusterId}/configmaps/${namespace}/${name}`
    );
  },

  // 獲取ConfigMap命名空間列表
  async getConfigMapNamespaces(clusterId: number): Promise<NamespaceItem[]> {
    return request.get<NamespaceItem[]>(
      `/clusters/${clusterId}/configmaps/namespaces`
    );
  },

  // 建立ConfigMap
  async createConfigMap(
    clusterId: number,
    data: {
      name: string;
      namespace: string;
      labels?: Record<string, string>;
      annotations?: Record<string, string>;
      data?: Record<string, string>;
    }
  ): Promise<{ name: string; namespace: string }> {
    return request.post<{ name: string; namespace: string }>(
      `/clusters/${clusterId}/configmaps`,
      data
    );
  },

  // 更新ConfigMap
  async updateConfigMap(
    clusterId: number,
    namespace: string,
    name: string,
    data: {
      labels?: Record<string, string>;
      annotations?: Record<string, string>;
      data?: Record<string, string>;
    }
  ): Promise<{ name: string; namespace: string; resourceVersion: string }> {
    return request.put<{ name: string; namespace: string; resourceVersion: string }>(
      `/clusters/${clusterId}/configmaps/${namespace}/${name}`,
      data
    );
  },

  // 刪除ConfigMap
  async deleteConfigMap(
    clusterId: number,
    namespace: string,
    name: string
  ): Promise<void> {
    await request.delete(
      `/clusters/${clusterId}/configmaps/${namespace}/${name}`
    );
  },

  // 獲取版本歷史
  async getVersions(clusterId: number, namespace: string, name: string): Promise<ConfigVersion[]> {
    return request.get<ConfigVersion[]>(
      `/clusters/${clusterId}/configmaps/${namespace}/${name}/versions`
    );
  },

  // 回滾到指定版本
  async rollback(clusterId: number, namespace: string, name: string, version: number): Promise<void> {
    await request.post(
      `/clusters/${clusterId}/configmaps/${namespace}/${name}/versions/${version}/rollback`,
      {}
    );
  },
};

// Secret API
export const secretService = {
  // 獲取Secret列表
  async getSecrets(
    clusterId: number,
    params: {
      namespace?: string;
      name?: string;
      type?: string;  // 支援按型別過濾 (如 kubernetes.io/dockerconfigjson)
      page?: number;
      pageSize?: number;
    }
  ): Promise<ListResponse<SecretListItem>> {
    const queryParams = new URLSearchParams();
    if (params.namespace) queryParams.append('namespace', params.namespace);
    if (params.name) queryParams.append('name', params.name);
    if (params.type) queryParams.append('type', params.type);
    if (params.page) queryParams.append('page', params.page.toString());
    if (params.pageSize) queryParams.append('pageSize', params.pageSize.toString());

    return request.get<ListResponse<SecretListItem>>(
      `/clusters/${clusterId}/secrets?${queryParams}`
    );
  },

  // 獲取Secret詳情
  async getSecret(
    clusterId: number,
    namespace: string,
    name: string
  ): Promise<SecretDetail> {
    return request.get<SecretDetail>(
      `/clusters/${clusterId}/secrets/${namespace}/${name}`
    );
  },

  // 獲取Secret命名空間列表
  async getSecretNamespaces(clusterId: number): Promise<NamespaceItem[]> {
    return request.get<NamespaceItem[]>(
      `/clusters/${clusterId}/secrets/namespaces`
    );
  },

  // 建立Secret
  async createSecret(
    clusterId: number,
    data: {
      name: string;
      namespace: string;
      type?: string;
      labels?: Record<string, string>;
      annotations?: Record<string, string>;
      data?: Record<string, string>;
    }
  ): Promise<{ name: string; namespace: string }> {
    return request.post<{ name: string; namespace: string }>(
      `/clusters/${clusterId}/secrets`,
      data
    );
  },

  // 更新Secret
  async updateSecret(
    clusterId: number,
    namespace: string,
    name: string,
    data: {
      labels?: Record<string, string>;
      annotations?: Record<string, string>;
      data?: Record<string, string>;
    }
  ): Promise<{ name: string; namespace: string; resourceVersion: string }> {
    return request.put<{ name: string; namespace: string; resourceVersion: string }>(
      `/clusters/${clusterId}/secrets/${namespace}/${name}`,
      data
    );
  },

  // 刪除Secret
  async deleteSecret(
    clusterId: number,
    namespace: string,
    name: string
  ): Promise<void> {
    await request.delete(
      `/clusters/${clusterId}/secrets/${namespace}/${name}`
    );
  },

  // 獲取版本歷史（僅記錄key列表，不含value）
  async getVersions(clusterId: number, namespace: string, name: string): Promise<ConfigVersion[]> {
    return request.get<ConfigVersion[]>(
      `/clusters/${clusterId}/secrets/${namespace}/${name}/versions`
    );
  },
};

