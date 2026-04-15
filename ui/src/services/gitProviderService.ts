import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface GitProvider {
  id: number;
  name: string;
  type: 'github' | 'gitlab' | 'gitea';
  base_url: string;
  webhook_token: string;
  enabled: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface CreateGitProviderRequest {
  name: string;
  type: 'github' | 'gitlab' | 'gitea';
  base_url: string;
  access_token?: string;
  webhook_secret?: string;
}

export interface UpdateGitProviderRequest {
  name?: string;
  base_url?: string;
  access_token?: string;
  webhook_secret?: string;
  enabled?: boolean;
}

export interface CreateGitProviderResponse {
  id: number;
  name: string;
  type: string;
  webhook_token: string;
  webhook_url: string;
}

export interface RegenerateTokenResponse {
  webhook_token: string;
  webhook_url: string;
}

// ─── Service ───────────────────────────────────────────────────────────────

const gitProviderService = {
  list(): Promise<{ items: GitProvider[]; total: number }> {
    return request.get('/system/git-providers');
  },

  get(id: number): Promise<GitProvider> {
    return request.get(`/system/git-providers/${id}`);
  },

  create(data: CreateGitProviderRequest): Promise<CreateGitProviderResponse> {
    return request.post('/system/git-providers', data);
  },

  update(id: number, data: UpdateGitProviderRequest): Promise<void> {
    return request.put(`/system/git-providers/${id}`, data);
  },

  delete(id: number): Promise<void> {
    return request.delete(`/system/git-providers/${id}`);
  },

  regenerateToken(id: number): Promise<RegenerateTokenResponse> {
    return request.post(`/system/git-providers/${id}/regenerate-token`);
  },
};

export default gitProviderService;
