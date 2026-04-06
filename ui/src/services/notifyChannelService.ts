import request from '../utils/request';

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
    return request.get('/system/notify-channels').then(res => res.data);
  },

  create(data: CreateNotifyChannelRequest): Promise<NotifyChannel> {
    return request.post('/system/notify-channels', data).then(res => res.data);
  },

  update(id: number, data: Partial<CreateNotifyChannelRequest>): Promise<NotifyChannel> {
    return request.put(`/system/notify-channels/${id}`, data).then(res => res.data);
  },

  delete(id: number): Promise<void> {
    return request.delete(`/system/notify-channels/${id}`).then(() => undefined);
  },

  test(id: number): Promise<{ message: string }> {
    return request.post(`/system/notify-channels/${id}/test`).then(res => res.data);
  },
};

export default notifyChannelService;
