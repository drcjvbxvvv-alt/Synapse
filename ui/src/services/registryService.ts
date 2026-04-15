import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface Registry {
  id: number;
  name: string;
  type: 'harbor' | 'dockerhub' | 'acr' | 'ecr' | 'gcr';
  url: string;
  username: string;
  insecure_tls: boolean;
  default_project: string;
  enabled: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface CreateRegistryRequest {
  name: string;
  type: 'harbor' | 'dockerhub' | 'acr' | 'ecr' | 'gcr';
  url: string;
  username?: string;
  password?: string;
  insecure_tls?: boolean;
  ca_bundle?: string;
  default_project?: string;
}

export interface UpdateRegistryRequest {
  name?: string;
  url?: string;
  username?: string;
  password?: string;
  insecure_tls?: boolean;
  ca_bundle?: string;
  default_project?: string;
  enabled?: boolean;
}

export interface TestConnectionResponse {
  connected: boolean;
  error?: string;
}

export interface RegistryRepository {
  name: string;
  tag_count?: number;
  pull_count?: number;
}

export interface RegistryTag {
  name: string;
  digest?: string;
  size?: number;
  created_at?: string;
}

// ─── Service ───────────────────────────────────────────────────────────────

const registryService = {
  list(): Promise<{ items: Registry[]; total: number }> {
    return request.get('/system/registries');
  },

  get(id: number): Promise<Registry> {
    return request.get(`/system/registries/${id}`);
  },

  create(data: CreateRegistryRequest): Promise<Registry> {
    return request.post('/system/registries', data);
  },

  update(id: number, data: UpdateRegistryRequest): Promise<void> {
    return request.put(`/system/registries/${id}`, data);
  },

  delete(id: number): Promise<void> {
    return request.delete(`/system/registries/${id}`);
  },

  testConnection(id: number): Promise<TestConnectionResponse> {
    return request.post(`/system/registries/${id}/test-connection`);
  },

  listRepositories(id: number, project?: string): Promise<{ items: RegistryRepository[]; total: number }> {
    const params = project ? `?project=${encodeURIComponent(project)}` : '';
    return request.get(`/system/registries/${id}/repositories${params}`);
  },

  listTags(id: number, repository: string): Promise<{ items: RegistryTag[]; total: number }> {
    return request.get(`/system/registries/${id}/tags?repository=${encodeURIComponent(repository)}`);
  },
};

export default registryService;
