import { request } from '../utils/api';

// Base URL for SSE streams (EventSource cannot use axios interceptors)
const API_BASE = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface Pipeline {
  id: number;
  name: string;
  description: string;
  project_id?: number | null;
  current_version_id: number | null;
  concurrency_group: string;
  concurrency_policy: 'cancel_previous' | 'queue' | 'reject';
  max_concurrent_runs: number;
  approval_enabled?: boolean;
  scan_enabled?: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface PipelineVersion {
  id: number;
  pipeline_id: number;
  version: number;
  steps_json: string;
  triggers_json: string;
  env_json: string;
  runtime_json: string;
  workspace_json: string;
  hash_sha256: string;
  created_by: number;
  created_at: string;
}

export interface PipelineRun {
  id: number;
  pipeline_id: number;
  snapshot_id: number;
  cluster_id: number;
  namespace: string;
  status: 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | 'cancelling' | 'waiting_approval';
  trigger_type: 'manual' | 'webhook' | 'cron' | 'rerun' | 'rollback';
  triggered_by_user: number;
  concurrency_group: string;
  rerun_from_id: number | null;
  rollback_of_run_id?: number | null;
  rerun_from_step: string;
  error: string;
  queued_at: string;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface StepRun {
  id: number;
  pipeline_run_id: number;
  step_name: string;
  step_type: string;
  step_index: number;
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped' | 'waiting_approval';
  image: string;
  job_name: string;
  exit_code: number | null;
  error: string;
  retry_count: number;
  max_retries: number;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
}

export interface CreatePipelineRequest {
  name: string;
  description?: string;
  project_id?: number;
  concurrency_group?: string;
  concurrency_policy?: 'cancel_previous' | 'queue' | 'reject';
  max_concurrent_runs?: number;
}

export interface UpdatePipelineRequest {
  description?: string;
  project_id?: number;
  concurrency_group?: string;
  concurrency_policy?: 'cancel_previous' | 'queue' | 'reject';
  max_concurrent_runs?: number;
  approval_enabled?: boolean;
  scan_enabled?: boolean;
}

export interface CreateVersionRequest {
  steps_json: string;
  triggers_json?: string;
  env_json?: string;
  runtime_json?: string;
  workspace_json?: string;
}

export interface TriggerRunRequest {
  cluster_id: number;
  namespace: string;
  version_id?: number;
}

// ─── Service ───────────────────────────────────────────────────────────────

const pipelineService = {
  // Pipelines
  list: (namespace?: string) =>
    request.get<{ items: Pipeline[]; total: number }>(
      `/pipelines${namespace ? `?namespace=${namespace}` : ''}`
    ),

  get: (pipelineId: number) =>
    request.get<Pipeline>(`/pipelines/${pipelineId}`),

  create: (data: CreatePipelineRequest) =>
    request.post<Pipeline>(`/pipelines`, data),

  update: (pipelineId: number, data: UpdatePipelineRequest) =>
    request.put<Pipeline>(`/pipelines/${pipelineId}`, data),

  delete: (pipelineId: number) =>
    request.delete<void>(`/pipelines/${pipelineId}`),

  // Versions
  listVersions: (pipelineId: number) =>
    request.get<{ items: PipelineVersion[]; total: number }>(
      `/pipelines/${pipelineId}/versions`
    ),

  createVersion: (pipelineId: number, data: CreateVersionRequest) =>
    request.post<PipelineVersion>(
      `/pipelines/${pipelineId}/versions`,
      data
    ),

  getVersion: (pipelineId: number, version: number) =>
    request.get<PipelineVersion>(
      `/pipelines/${pipelineId}/versions/${version}`
    ),

  // Runs
  listRuns: (pipelineId: number) =>
    request.get<{ items: PipelineRun[]; total: number }>(
      `/pipelines/${pipelineId}/runs`
    ),

  triggerRun: (pipelineId: number, req: TriggerRunRequest) =>
    request.post<{ run_id: number; status: string }>(
      `/pipelines/${pipelineId}/runs`,
      req
    ),

  // GetRun returns both the run and its step runs (see pipeline_run_handler.go)
  getRun: (pipelineId: number, runId: number) =>
    request.get<{ run: PipelineRun; steps: StepRun[] }>(
      `/pipelines/${pipelineId}/runs/${runId}`
    ),

  cancelRun: (pipelineId: number, runId: number) =>
    request.post<void>(
      `/pipelines/${pipelineId}/runs/${runId}/cancel`,
      {}
    ),

  rerun: (pipelineId: number, runId: number, fromFailed = false) =>
    request.post<{ run_id: number; status: string }>(
      `/pipelines/${pipelineId}/runs/${runId}/rerun`,
      { from_failed: fromFailed }
    ),

  rollbackRun: (
    pipelineId: number,
    runId: number,
    req?: { cluster_id?: number; namespace?: string },
  ) =>
    request.post<{ run_id: number; rollback_of_run: number; snapshot_id: number; status: string; message: string }>(
      `/pipelines/${pipelineId}/runs/${runId}/rollback`,
      req ?? {},
    ),

  approveStep: (pipelineId: number, runId: number, stepRunId: number) =>
    request.post<void>(
      `/pipelines/${pipelineId}/runs/${runId}/steps/${stepRunId}/approve`,
      {}
    ),

  rejectStep: (pipelineId: number, runId: number, stepRunId: number, reason?: string) =>
    request.post<void>(
      `/pipelines/${pipelineId}/runs/${runId}/steps/${stepRunId}/reject`,
      { reason }
    ),

  /**
   * Returns the SSE URL for streaming a step run's logs.
   * Use with useSSELog hook — EventSource cannot go through axios.
   */
  getStepLogUrl: (
    pipelineId: number,
    runId: number,
    stepRunId: number,
  ): string =>
    `${API_BASE}/pipelines/${pipelineId}/runs/${runId}/steps/${stepRunId}/logs?follow=true`,
};

export default pipelineService;

// ─── Allowed Images ───────────────────────────────────────────────────────────

export const pipelineAllowedImagesService = {
  get: () =>
    request.get<{ patterns: string[] }>('/system/pipeline/allowed-images'),
  update: (patterns: string[]) =>
    request.put<{ patterns: string[] }>('/system/pipeline/allowed-images', { patterns }),
};
