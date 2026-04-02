import { request } from '@/utils/api';

export interface ScanResult {
  id: number;
  cluster_id: number;
  namespace: string;
  pod_name: string;
  container_name: string;
  image: string;
  status: 'pending' | 'scanning' | 'completed' | 'failed';
  critical: number;
  high: number;
  medium: number;
  low: number;
  unknown: number;
  error?: string;
  scanned_at?: string;
}

export interface ScanDetail extends ScanResult {
  result_json?: string;
}

export interface TrivyVulnerability {
  VulnerabilityID: string;
  PkgName: string;
  InstalledVersion: string;
  FixedVersion: string;
  Severity: string;
  Title: string;
  Description: string;
}

export interface TrivyTargetResult {
  Target: string;
  Vulnerabilities: TrivyVulnerability[];
}

export interface TrivyReport {
  Results: TrivyTargetResult[];
}

export interface BenchResult {
  id: number;
  cluster_id: number;
  status: 'pending' | 'running' | 'completed' | 'failed';
  pass: number;
  fail: number;
  warn: number;
  info: number;
  score: number;
  error?: string;
  job_name: string;
  run_at?: string;
  created_at: string;
}

export interface BenchDetail extends BenchResult {
  result_json?: string;
}

export interface BenchTestResult {
  test_number: string;
  text: string;
  status: string;
  actual_value: string;
  remediation: string;
}

export interface BenchGroup {
  id: string;
  text: string;
  tests: BenchTestResult[];
}

export interface BenchSection {
  id: string;
  text: string;
  node_type: string;
  tests: BenchGroup[];
}

export interface GatekeeperViolation {
  constraint_kind: string;
  constraint_name: string;
  resource: string;
  namespace: string;
  message: string;
}

export interface ConstraintSummary {
  kind: string;
  name: string;
  violation_count: number;
  violations: GatekeeperViolation[];
}

export interface GatekeeperSummary {
  total_violations: number;
  constraints: ConstraintSummary[];
}

const securityService = {
  // Image scanning
  triggerScan: (clusterId: number, image: string, namespace?: string, podName?: string, containerName?: string) =>
    request.post<ScanResult>(`/clusters/${clusterId}/security/scans`, {
      image,
      namespace: namespace ?? '',
      pod_name: podName ?? '',
      container_name: containerName ?? '',
    }),

  getScanResults: (clusterId: number, namespace?: string) =>
    request.get<ScanResult[]>(`/clusters/${clusterId}/security/scans`, {
      params: namespace ? { namespace } : {},
    }),

  getScanDetail: (clusterId: number, scanId: number) =>
    request.get<ScanDetail>(`/clusters/${clusterId}/security/scans/${scanId}`),

  // CIS Benchmark
  triggerBenchmark: (clusterId: number) =>
    request.post<BenchResult>(`/clusters/${clusterId}/security/bench`, {}),

  getBenchResults: (clusterId: number) =>
    request.get<BenchResult[]>(`/clusters/${clusterId}/security/bench`),

  getBenchDetail: (clusterId: number, benchId: number) =>
    request.get<BenchDetail>(`/clusters/${clusterId}/security/bench/${benchId}`),

  // Gatekeeper
  getGatekeeperViolations: (clusterId: number) =>
    request.get<GatekeeperSummary>(`/clusters/${clusterId}/security/gatekeeper`),
};

export default securityService;
