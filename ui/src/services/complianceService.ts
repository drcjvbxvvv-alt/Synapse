import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export type Framework = 'SOC2' | 'ISO27001' | 'CIS_K8S';

export interface ComplianceReport {
  id: number;
  cluster_id: number;
  framework: Framework;
  version: string;
  status: 'pending' | 'generating' | 'completed' | 'failed';
  score: number;
  pass_count: number;
  fail_count: number;
  warn_count: number;
  result_json?: string;
  error?: string;
  generated_by: number;
  generated_at: string | null;
  created_at: string;
}

export interface ControlResult {
  control_id: string;
  title: string;
  category: string;
  status: 'pass' | 'fail' | 'warn' | 'na';
  description: string;
  evidence?: string;
}

export interface ViolationEvent {
  id: number;
  cluster_id: number;
  source: 'gatekeeper' | 'trivy' | 'bench' | 'audit';
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info';
  title: string;
  description: string;
  resource_type: string;
  resource_ref: string;
  resolved_at: string | null;
  resolved_by: string;
  created_at: string;
}

export interface ViolationStats {
  total_open: number;
  total_resolved: number;
  by_source: Record<string, number>;
  by_severity: Record<string, number>;
}

export interface ComplianceEvidence {
  id: number;
  report_id: number | null;
  cluster_id: number;
  framework: string;
  control_id: string;
  control_title: string;
  evidence_type: string;
  data_json?: string;
  captured_at: string;
  created_at: string;
}

// ─── API ───────────────────────────────────────────────────────────────────

export const complianceService = {
  // Reports
  generateReport: (clusterId: string, framework: Framework) =>
    request.post(`/clusters/${clusterId}/compliance/reports`, { framework }),

  listReports: (clusterId: string, framework?: string) =>
    request.get(`/clusters/${clusterId}/compliance/reports`, { params: { framework } }),

  getReport: (clusterId: string, reportId: number) =>
    request.get(`/clusters/${clusterId}/compliance/reports/${reportId}`),

  exportReport: (clusterId: string, reportId: number) =>
    request.get(`/clusters/${clusterId}/compliance/reports/${reportId}/export`),

  deleteReport: (clusterId: string, reportId: number) =>
    request.delete(`/clusters/${clusterId}/compliance/reports/${reportId}`),

  // Violations
  listViolations: (clusterId: string, params?: {
    source?: string;
    severity?: string;
    resolved?: string;
    page?: number;
    pageSize?: number;
  }) =>
    request.get(`/clusters/${clusterId}/compliance/violations`, { params }),

  getViolationStats: (clusterId: string) =>
    request.get(`/clusters/${clusterId}/compliance/violations/stats`),

  resolveViolation: (clusterId: string, violationId: number) =>
    request.put(`/clusters/${clusterId}/compliance/violations/${violationId}/resolve`),

  // Evidence
  captureEvidence: (clusterId: string, data: {
    framework: string;
    control_id: string;
    control_title?: string;
    evidence_type: string;
    data_json: string;
  }) =>
    request.post(`/clusters/${clusterId}/compliance/evidence`, data),

  listEvidence: (clusterId: string, framework?: string, controlId?: string) =>
    request.get(`/clusters/${clusterId}/compliance/evidence`, {
      params: { framework, control_id: controlId },
    }),

  getEvidence: (clusterId: string, evidenceId: number) =>
    request.get(`/clusters/${clusterId}/compliance/evidence/${evidenceId}`),
};
