import api from '../utils/api';

export interface NotificationItem {
  id: number;
  ruleName: string;
  clusterId: number;
  clusterName: string;
  namespace: string;
  eventReason: string;
  eventType: string;   // Warning | Normal
  message: string;
  involvedObj: string; // kind/name
  notifyResult: string; // sent | failed | disabled
  isRead: boolean;
  triggeredAt: string;
}

export interface NotificationListResponse {
  items: NotificationItem[];
  total: number;
  unreadCount: number;
}

export const notificationService = {
  list: () =>
    api.get<NotificationListResponse>('/notifications'),

  unreadCount: () =>
    api.get<{ unreadCount: number }>('/notifications/unread-count'),

  markRead: (id: number) =>
    api.put(`/notifications/${id}/read`),

  markAllRead: () =>
    api.put('/notifications/read-all'),
};
