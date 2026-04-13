import api from '../utils/api';

export interface PortForwardSession {
  id: number;
  clusterID: number;
  clusterName: string;
  namespace: string;
  podName: string;
  podPort: number;
  localPort: number;
  username: string;
  status: 'active' | 'stopped';
  stoppedAt?: string;
  createdAt: string;
}

export const portforwardService = {
  start: (clusterID: string, namespace: string, podName: string, podPort: number) =>
    api.post<{ sessionId: number; localPort: number; podPort: number; message: string }>(
      `clusters/${clusterID}/pods/${namespace}/${podName}/portforward`,
      { podPort }
    ),

  stop: (sessionId: number) =>
    api.delete(`portforwards/${sessionId}`),

  list: (status?: string) =>
    api.get<{ items: PortForwardSession[]; total: number }>('portforwards', {
      params: { status },
    }),
};
