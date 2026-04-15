import { request } from '../utils/api';

// Base URL for SSE streams (EventSource cannot use axios interceptors)
const API_BASE = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface Pipeline {
  id: number;
  name: string;
  description: string;
  cluster_id: number;
  namespace: string;
  current_version_id: number | null;
  concurrency_group: string;
  concurrency_policy: 'cancel_previous' | 'queue' | 'reject';
  max_concurrent_runs: number;
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
  status: 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | 'waiting_approval';
  trigger_type: 'manual' | 'webhook' | 'cron' | 'rerun';
  triggered_by_user: number;
  concurrency_group: string;
  rerun_from_id: number | null;
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
  namespace: string;
  concurrency_group?: string;
  concurrency_policy?: 'cancel_previous' | 'queue' | 'reject';
  max_concurrent_runs?: number;
}

export interface UpdatePipelineRequest {
  description?: string;
  concurrency_group?: string;
  concurrency_policy?: 'cancel_previous' | 'queue' | 'reject';
  max_concurrent_runs?: number;
}

export interface CreateVersionRequest {
  steps_json: string;
  triggers_json?: string;
  env_json?: string;
  runtime_json?: string;
  workspace_json?: string;
}

export interface TriggerRunRequest {
  trigger_type?: 'manual';
}

// ─── Service ───────────────────────────────────────────────────────────────

const pipelineService = {
  // Pipelines
  list: (clusterId: number, namespace?: string) =>
    request.get<{ items: Pipeline[]; total: number }>(
      `/clusters/${clusterId}/pipelines${namespace ? `?namespace=${namespace}` : ''}`
    ),

  get: (clusterId: number, pipelineId: number) =>
    request.get<Pipeline>(`/clusters/${clusterId}/pipelines/${pipelineId}`),

  create: (clusterId: number, data: CreatePipelineRequest) =>
    request.post<Pipeline>(`/clusters/${clusterId}/pipelines`, data),

  update: (clusterId: number, pipelineId: number, data: UpdatePipelineRequest) =>
    request.put<Pipeline>(`/clusters/${clusterId}/pipelines/${pipelineId}`, data),

  delete: (clusterId: number, pipelineId: number) =>
    request.delete<void>(`/clusters/${clusterId}/pipelines/${pipelineId}`),

  // Versions
  listVersions: (clusterId: number, pipelineId: number) =>
    request.get<{ items: PipelineVersion[]; total: number }>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/versions`
    ),

  createVersion: (clusterId: number, pipelineId: number, data: CreateVersionRequest) =>
    request.post<PipelineVersion>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/versions`,
      data
    ),

  getVersion: (clusterId: number, pipelineId: number, version: number) =>
    request.get<PipelineVersion>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/versions/${version}`
    ),

  // Runs
  listRuns: (clusterId: number, pipelineId: number) =>
    request.get<{ items: PipelineRun[]; total: number }>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/runs`
    ),

  triggerRun: (clusterId: number, pipelineId: number, data?: TriggerRunRequest) =>
    request.post<PipelineRun>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/runs`,
      data ?? { trigger_type: 'manual' }
    ),

  // GetRun returns both the run and its step runs (see pipeline_run_handler.go)
  getRun: (clusterId: number, pipelineId: number, runId: number) =>
    request.get<{ run: PipelineRun; steps: StepRun[] }>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/runs/${runId}`
    ),

  cancelRun: (clusterId: number, pipelineId: number, runId: number) =>
    request.post<void>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/runs/${runId}/cancel`,
      {}
    ),

  rerun: (clusterId: number, pipelineId: number, runId: number, fromFailed = false) =>
    request.post<PipelineRun>(
      `/clusters/${clusterId}/pipelines/${pipelineId}/runs/${runId}/rerun`,
      { from_failed: fromFailed }
    ),

  /**
   * Returns the SSE URL for streaming a step run's logs.
   * Use with useSSELog hook — EventSource cannot go through axios.
   */
  getStepLogUrl: (
    clusterId: number,
    pipelineId: number,
    runId: number,
    stepRunId: number,
  ): string =>
    `${API_BASE}/clusters/${clusterId}/pipelines/${pipelineId}/runs/${runId}/steps/${stepRunId}/logs?follow=true`,
};

export default pipelineService;
