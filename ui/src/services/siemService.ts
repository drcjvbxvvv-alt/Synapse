import axios from 'axios';

export interface SIEMConfig {
  id?: number;
  enabled: boolean;
  webhookURL: string;
  secretHeader: string;
  secretValue: string;
  batchSize?: number;
}

export const siemService = {
  getConfig: () =>
    axios.get<SIEMConfig>('/api/v1/system/siem/config'),

  updateConfig: (data: SIEMConfig) =>
    axios.put('/api/v1/system/siem/config', data),

  testWebhook: () =>
    axios.post<{ message: string; statusCode: number }>('/api/v1/system/siem/test'),

  exportLogs: (params?: { start?: string; end?: string }) => {
    const query = new URLSearchParams(params as Record<string, string>).toString();
    window.open(`/api/v1/audit/export${query ? '?' + query : ''}`, '_blank');
  },
};
