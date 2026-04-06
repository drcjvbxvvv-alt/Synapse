import { request } from '../utils/api';

export interface NotifyChannel {
  id: number;
  name: string;
  type: 'webhook' | 'dingtalk' | 'slack' | 'teams';
  webhookUrl: string;
  dingTalkSecret?: string;
  description?: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateNotifyChannelRequest {
  name: string;
  type: string;
  webhookUrl: string;
  dingTalkSecret?: string;
  description?: string;
  enabled: boolean;
}

const notifyChannelService = {
  list(): Promise<NotifyChannel[]> {
    return request.get<NotifyChannel[]>('/system/notify-channels');
  },

  create(data: CreateNotifyChannelRequest): Promise<NotifyChannel> {
    return request.post<NotifyChannel>('/system/notify-channels', data);
  },

  update(id: number, data: Partial<CreateNotifyChannelRequest>): Promise<NotifyChannel> {
    return request.put<NotifyChannel>(`/system/notify-channels/${id}`, data);
  },

  delete(id: number): Promise<void> {
    return request.delete<void>(`/system/notify-channels/${id}`);
  },

  test(id: number): Promise<{ message: string }> {
    return request.post<{ message: string }>(`/system/notify-channels/${id}/test`);
  },
};

export default notifyChannelService;
