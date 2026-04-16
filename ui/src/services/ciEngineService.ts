import { request } from '../utils/api';

// ─── Config types ──────────────────────────────────────────────────────────

export type CIEngineType = 'native' | 'gitlab' | 'jenkins' | 'tekton' | 'argo' | 'github';

export interface CIEngineConfig {
  id: number;
  name: string;
  engine_type: CIEngineType;
  enabled: boolean;
  endpoint: string;
  auth_type: string;
  username: string;
  cluster_id?: number;
  extra_json?: string;
  insecure_skip_verify: boolean;
  last_checked_at?: string;
  last_healthy: boolean;
  last_version?: string;
  last_error?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface CIEngineStatus {
  type: CIEngineType;
  available: boolean;
  version: string;
  message?: string;
  config_id?: number;
  config_name?: string;
}

export interface CreateCIEngineRequest {
  name: string;
  engine_type: CIEngineType;
  enabled?: boolean;
  endpoint?: string;
  auth_type?: string;
  username?: string;
  token?: string;
  password?: string;
  webhook_secret?: string;
  cluster_id?: number;
  extra_json?: string;
  insecure_skip_verify?: boolean;
  ca_bundle?: string;
}

export type UpdateCIEngineRequest = Partial<CreateCIEngineRequest>;

// ─── Run operation types ───────────────────────────────────────────────────

export type RunPhase = 'pending' | 'running' | 'success' | 'failed' | 'cancelled' | 'unknown';

export const RUN_PHASE_TERMINAL: ReadonlySet<RunPhase> = new Set([
  'success', 'failed', 'cancelled',
]);

export interface TriggerRunRequest {
  ref?: string;
  variables?: Record<string, string>;
  pipeline_id?: number;
}

export interface TriggerRunResult {
  run_id: string;
  external_id?: string;
  url?: string;
  queued_at: string;
}

export interface StepStatus {
  name: string;
  phase: RunPhase;
  raw?: string;
  started_at?: string;
  finished_at?: string;
}

export interface RunStatus {
  run_id: string;
  external_id?: string;
  phase: RunPhase;
  raw?: string;
  message?: string;
  started_at?: string;
  finished_at?: string;
  steps?: StepStatus[];
}

export interface Artifact {
  name: string;
  kind: string;
  url?: string;
  size_bytes?: number;
  digest?: string;
  created_at: string;
}

// ─── Service ───────────────────────────────────────────────────────────────

const ciEngineService = {
  // ── Config CRUD ──────────────────────────────────────────────────────────

  list(): Promise<{ items: CIEngineConfig[]; total: number }> {
    return request.get('/ci-engines');
  },

  get(id: number): Promise<CIEngineConfig> {
    return request.get(`/ci-engines/${id}`);
  },

  listStatus(): Promise<{ items: CIEngineStatus[]; total: number }> {
    return request.get('/ci-engines/status');
  },

  create(req: CreateCIEngineRequest): Promise<CIEngineConfig> {
    return request.post('/ci-engines', req);
  },

  update(id: number, req: UpdateCIEngineRequest): Promise<CIEngineConfig> {
    return request.put(`/ci-engines/${id}`, req);
  },

  delete(id: number): Promise<void> {
    return request.delete(`/ci-engines/${id}`);
  },

  // ── Run operations ────────────────────────────────────────────────────────

  /** POST /ci-engines/:id/runs — trigger a new run */
  triggerRun(id: number, req: TriggerRunRequest): Promise<TriggerRunResult> {
    return request.post(`/ci-engines/${id}/runs`, req);
  },

  /** GET /ci-engines/:id/runs/:runId — current run status */
  getRun(id: number, runId: string): Promise<RunStatus> {
    return request.get(`/ci-engines/${id}/runs/${encodeURIComponent(runId)}`);
  },

  /** DELETE /ci-engines/:id/runs/:runId — cancel a run */
  cancelRun(id: number, runId: string): Promise<void> {
    return request.delete(`/ci-engines/${id}/runs/${encodeURIComponent(runId)}`);
  },

  /** GET /ci-engines/:id/runs/:runId/logs?step= — fetch log snapshot (text/plain) */
  fetchLogs(id: number, runId: string, step?: string): Promise<string> {
    return request.get<string>(
      `/ci-engines/${id}/runs/${encodeURIComponent(runId)}/logs`,
      { params: step ? { step } : undefined, responseType: 'text' },
    );
  },

  /** GET /ci-engines/:id/runs/:runId/artifacts */
  getArtifacts(id: number, runId: string): Promise<{ items: Artifact[]; total: number }> {
    return request.get(`/ci-engines/${id}/runs/${encodeURIComponent(runId)}/artifacts`);
  },
};

export default ciEngineService;
