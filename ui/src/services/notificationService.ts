import axios from 'axios';

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
    axios.get<NotificationListResponse>('/notifications'),

  unreadCount: () =>
    axios.get<{ unreadCount: number }>('/notifications/unread-count'),

  markRead: (id: number) =>
    axios.put(`/notifications/${id}/read`),

  markAllRead: () =>
    axios.put('/notifications/read-all'),
};
