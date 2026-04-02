import axios from 'axios';

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
    axios.post<{ sessionId: number; localPort: number; podPort: number; message: string }>(
      `/api/v1/clusters/${clusterID}/pods/${namespace}/${podName}/portforward`,
      { podPort }
    ),

  stop: (sessionId: number) =>
    axios.delete(`/api/v1/portforwards/${sessionId}`),

  list: (status?: string) =>
    axios.get<{ items: PortForwardSession[]; total: number }>('/api/v1/portforwards', {
      params: { status },
    }),
};
