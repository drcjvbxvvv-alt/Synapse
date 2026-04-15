import { request } from '../utils/api';

export type SecretScope = 'global' | 'pipeline';

export interface PipelineSecret {
  id: number;
  scope: SecretScope;
  scope_ref: number | null;
  name: string;
  description: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface CreateSecretRequest {
  scope: SecretScope;
  scope_ref?: number;
  name: string;
  value: string;
  description?: string;
}

export interface UpdateSecretRequest {
  value?: string;        // 留空 = 保留現有值
  description?: string;
}

const pipelineSecretService = {
  list: (scope?: SecretScope, scopeRef?: number) => {
    const params = new URLSearchParams();
    if (scope) params.set('scope', scope);
    if (scopeRef !== undefined) params.set('scope_ref', String(scopeRef));
    return request.get<PipelineSecret[]>(`/pipeline-secrets?${params.toString()}`);
  },

  create: (req: CreateSecretRequest) =>
    request.post<PipelineSecret>('/pipeline-secrets', req),

  update: (id: number, req: UpdateSecretRequest) =>
    request.put<PipelineSecret>(`/pipeline-secrets/${id}`, req),

  delete: (id: number) =>
    request.delete<void>(`/pipeline-secrets/${id}`),
};

export default pipelineSecretService;
