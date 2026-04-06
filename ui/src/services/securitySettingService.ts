import { request } from '../utils/api';
import type { SecurityConfig, APIToken, CreateAPITokenResponse } from '../types';

export interface CreateAPITokenRequest {
  name: string;
  scopes: string[];
  expires_at?: string;
}

export const securitySettingService = {
  // 取得安全設定（PlatformAdmin）
  getSecurityConfig: (): Promise<SecurityConfig> =>
    request.get<SecurityConfig>('/system/security/config'),

  // 更新安全設定（PlatformAdmin）
  updateSecurityConfig: (config: SecurityConfig): Promise<{ message: string }> =>
    request.put<{ message: string }>('/system/security/config', config),

  // 列出個人 API Token
  listAPITokens: (): Promise<APIToken[]> =>
    request.get<APIToken[]>('/users/me/tokens'),

  // 建立 API Token（回傳一次性明文）
  createAPIToken: (data: CreateAPITokenRequest): Promise<CreateAPITokenResponse> =>
    request.post<CreateAPITokenResponse>('/users/me/tokens', data),

  // 撤銷 API Token
  deleteAPIToken: (id: number): Promise<{ message: string }> =>
    request.delete<{ message: string }>(`/users/me/tokens/${id}`),
};
