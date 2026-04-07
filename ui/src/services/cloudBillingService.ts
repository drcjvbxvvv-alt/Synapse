import { request } from '../utils/api';

export interface CloudBillingConfig {
  id: number;
  cluster_id: number;
  provider: 'disabled' | 'aws' | 'gcp';
  // AWS
  aws_access_key_id: string;
  aws_secret_set: boolean;
  aws_region: string;
  aws_linked_account_id: string;
  // GCP
  gcp_project_id: string;
  gcp_billing_account_id: string;
  gcp_service_account_set: boolean;
  // status
  last_synced_at: string | null;
  last_error: string;
}

export interface UpdateBillingConfigReq {
  provider: string;
  aws_access_key_id?: string;
  aws_secret_access_key?: string;
  aws_region?: string;
  aws_linked_account_id?: string;
  gcp_project_id?: string;
  gcp_billing_account_id?: string;
  gcp_service_account_json?: string;
}

export interface CloudBillingRecord {
  id: number;
  cluster_id: number;
  month: string;
  provider: string;
  service: string;
  amount: number;
  currency: string;
}

export interface CloudBillingOverview {
  month: string;
  provider: string;
  total_amount: number;
  currency: string;
  cpu_unit_cost: number;    // USD/core-hour
  memory_unit_cost: number; // USD/GiB-hour
  services: CloudBillingRecord[];
  last_synced_at: string | null;
  sync_error?: string;
}

export const CloudBillingService = {
  getConfig: (clusterId: string): Promise<CloudBillingConfig> =>
    request.get(`/clusters/${clusterId}/billing/config`),

  updateConfig: (clusterId: string, req: UpdateBillingConfigReq): Promise<{ message: string; provider: string }> =>
    request.put(`/clusters/${clusterId}/billing/config`, req),

  sync: (clusterId: string, month?: string): Promise<{ message: string }> => {
    const q = month ? `?month=${month}` : '';
    return request.post(`/clusters/${clusterId}/billing/sync${q}`, {});
  },

  getOverview: (clusterId: string, month?: string): Promise<CloudBillingOverview> => {
    const q = month ? `?month=${month}` : '';
    return request.get(`/clusters/${clusterId}/billing/overview${q}`);
  },
};
