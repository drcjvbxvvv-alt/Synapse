import { request } from '../utils/api';

export type ChaosKind =
  | 'PodChaos'
  | 'NetworkChaos'
  | 'StressChaos'
  | 'HTTPChaos'
  | 'IOChaos';

export interface ChaosExperiment {
  uid: string;
  name: string;
  namespace: string;
  kind: ChaosKind;
  phase: string;
  duration: string;
  created_at: string;
}

export interface ChaosSchedule {
  uid?: string;
  name: string;
  namespace: string;
  cron_expr: string;
  type: string;
  suspended: boolean;
  last_run_time?: string;
}

export interface ChaosStatus {
  installed: boolean;
  version: string;
}

export interface TargetSelector {
  namespace?: string;
  label_selectors?: Record<string, string>;
  pod_phase_selectors?: string[];
}

export interface PodChaosSpec {
  action: 'pod-kill' | 'pod-failure' | 'container-kill';
  mode: 'one' | 'all' | 'fixed' | 'fixed-percent' | 'random-max-percent';
  value?: string;
  duration?: string;
  selector: TargetSelector;
  container_names?: string[];
}

export interface NetworkDelaySpec {
  latency: string;
  jitter?: string;
  correlation?: string;
}

export interface NetworkLossSpec {
  loss: string;
  correlation?: string;
}

export interface NetworkChaosSpec {
  action: 'delay' | 'loss' | 'duplicate' | 'corrupt' | 'bandwidth' | 'partition';
  mode: string;
  value?: string;
  duration?: string;
  selector: TargetSelector;
  delay?: NetworkDelaySpec;
  loss?: NetworkLossSpec;
}

export interface StressCPUSpec {
  workers: number;
  load?: number;
}

export interface StressMemSpec {
  workers: number;
  size?: string;
}

export interface StressChaosSpec {
  mode: string;
  value?: string;
  duration?: string;
  selector: TargetSelector;
  stressors?: {
    cpu?: StressCPUSpec;
    memory?: StressMemSpec;
  };
}

export interface CreateChaosRequest {
  kind: ChaosKind;
  name: string;
  namespace: string;
  pod_chaos?: PodChaosSpec;
  network_chaos?: NetworkChaosSpec;
  stress_chaos?: StressChaosSpec;
}

export interface CreateScheduleRequest {
  name: string;
  namespace: string;
  cron_expr: string;
  kind: ChaosKind;
  duration?: string;
  target: TargetSelector;
  pod_chaos?: PodChaosSpec;
  network_chaos?: NetworkChaosSpec;
  stress_chaos?: StressChaosSpec;
}

interface ListResponse<T> {
  items: T[];
  total: number;
}

export const chaosService = {
  getStatus: (clusterID: string | number) =>
    request.get<ChaosStatus>(`/clusters/${clusterID}/chaos/status`),

  listExperiments: (clusterID: string | number, namespace?: string) =>
    request.get<ListResponse<ChaosExperiment>>(
      `/clusters/${clusterID}/chaos/experiments`,
      { params: namespace ? { namespace } : undefined },
    ),

  getExperiment: (clusterID: string | number, namespace: string, kind: string, name: string) =>
    request.get<object>(
      `/clusters/${clusterID}/chaos/experiments/${namespace}/${kind}/${name}`,
    ),

  createExperiment: (clusterID: string | number, payload: CreateChaosRequest) =>
    request.post<ChaosExperiment>(
      `/clusters/${clusterID}/chaos/experiments`,
      payload,
    ),

  deleteExperiment: (clusterID: string | number, namespace: string, kind: string, name: string) =>
    request.delete<{ message: string }>(
      `/clusters/${clusterID}/chaos/experiments/${namespace}/${kind}/${name}`,
    ),

  listSchedules: (clusterID: string | number, namespace?: string) =>
    request.get<ListResponse<ChaosSchedule>>(
      `/clusters/${clusterID}/chaos/schedules`,
      { params: namespace ? { namespace } : undefined },
    ),

  createSchedule: (clusterID: string | number, payload: CreateScheduleRequest) =>
    request.post<ChaosSchedule>(
      `/clusters/${clusterID}/chaos/schedules`,
      payload,
    ),

  deleteSchedule: (clusterID: string | number, namespace: string, name: string) =>
    request.delete<{ message: string }>(
      `/clusters/${clusterID}/chaos/schedules/${namespace}/${name}`,
    ),
};
