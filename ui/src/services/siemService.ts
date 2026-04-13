import api from '../utils/api';

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
    api.get<SIEMConfig>('system/siem/config'),

  updateConfig: (data: SIEMConfig) =>
    api.put('system/siem/config', data),

  testWebhook: () =>
    api.post<{ message: string; statusCode: number }>('system/siem/test'),

  exportLogs: (params?: { start?: string; end?: string }) => {
    const query = new URLSearchParams(params as Record<string, string>).toString();
    window.open(`audit/export${query ? '?' + query : ''}`, '_blank');
  },
};
