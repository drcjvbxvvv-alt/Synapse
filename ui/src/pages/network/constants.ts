// 網路拓樸共用常數 — 集中管理，避免各元件重複定義

export const WORKLOAD_KIND_COLOR: Record<string, string> = {
  Deployment:  '#1677ff',
  StatefulSet: '#722ed1',
  DaemonSet:   '#13c2c2',
  Job:         '#fa8c16',
  Pod:         '#8c8c8c',
  ReplicaSet:  '#1677ff',
};

export const HEALTH_COLOR: Record<string, string> = {
  healthy:  '#52c41a',
  degraded: '#fa8c16',
  down:     '#ff4d4f',
  unknown:  '#d9d9d9',
};
