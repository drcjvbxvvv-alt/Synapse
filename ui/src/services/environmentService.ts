import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface Environment {
  id: number;
  name: string;
  pipeline_id: number;
  cluster_id: number;
  namespace: string;
  order_index: number;
  auto_promote: boolean;
  approval_required: boolean;
  approver_ids: string;       // JSON array string e.g. "[1,2,3]"
  smoke_test_step_name: string;
  notify_channel_ids: string; // JSON array string
  created_at: string;
  updated_at: string;
}

export interface CreateEnvironmentRequest {
  name: string;
  cluster_id: number;
  namespace: string;
  order_index: number;
  auto_promote?: boolean;
  approval_required?: boolean;
  approver_ids?: number[];
  smoke_test_step_name?: string;
  notify_channel_ids?: number[];
}

export interface UpdateEnvironmentRequest {
  name?: string;
  cluster_id?: number;
  namespace?: string;
  order_index?: number;
  auto_promote?: boolean;
  approval_required?: boolean;
  approver_ids?: number[];
  smoke_test_step_name?: string;
  notify_channel_ids?: number[];
}

// ─── Service ───────────────────────────────────────────────────────────────

const environmentService = {
  list(clusterId: number, pipelineId: number): Promise<{ items: Environment[]; total: number }> {
    return request.get(`/clusters/${clusterId}/pipelines/${pipelineId}/environments`);
  },

  create(clusterId: number, pipelineId: number, data: CreateEnvironmentRequest): Promise<Environment> {
    return request.post(`/clusters/${clusterId}/pipelines/${pipelineId}/environments`, data);
  },

  update(clusterId: number, pipelineId: number, envId: number, data: UpdateEnvironmentRequest): Promise<void> {
    return request.put(`/clusters/${clusterId}/pipelines/${pipelineId}/environments/${envId}`, data);
  },

  delete(clusterId: number, pipelineId: number, envId: number): Promise<void> {
    return request.delete(`/clusters/${clusterId}/pipelines/${pipelineId}/environments/${envId}`);
  },
};

export default environmentService;
