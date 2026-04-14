import React, { useState, useEffect, useCallback } from 'react';
import {
  Badge, Button, Popover, List, Typography, Space, Tag, Tooltip, Spin, Empty,
} from 'antd';
import {
  BellOutlined, CheckOutlined, WarningOutlined, InfoCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  notificationService, type NotificationItem,
} from '../services/notificationService';
import { useVisibilityInterval } from '../hooks/useVisibilityInterval';

const { Text } = Typography;

const POLL_INTERVAL = 30_000; // 30s

function eventTypeColor(type: string) {
  return type === 'Warning' ? 'warning' : 'processing';
}

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60_000);
  if (m < 1) return '剛剛';
  if (m < 60) return `${m} 分鐘前`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} 小時前`;
  return `${Math.floor(h / 24)} 天前`;
}

// ─── Popover content ────────────────────────────────────────────────────────

interface ContentProps {
  items: NotificationItem[];
  loading: boolean;
  onMarkRead: (id: number) => void;
  onMarkAllRead: () => void;
}

const PopoverContent: React.FC<ContentProps> = ({ items, loading, onMarkRead, onMarkAllRead }) => {
  const { t } = useTranslation('common');
  const unread = items.filter(n => !n.isRead).length;

  return (
    <div style={{ width: 380 }}>
      {/* Header */}
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        padding: '0 0 10px', borderBottom: '1px solid #f0f0f0', marginBottom: 4,
      }}>
        <Text strong style={{ fontSize: 14 }}>
          {t('notification.title')}
          {unread > 0 && (
            <Badge count={unread} size="small" style={{ marginLeft: 6 }} />
          )}
        </Text>
        {unread > 0 && (
          <Button
            type="link"
            size="small"
            icon={<CheckOutlined />}
            onClick={onMarkAllRead}
            style={{ padding: 0, fontSize: 12 }}
          >
            {t('notification.markAllRead')}
          </Button>
        )}
      </div>

      {/* List */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
      ) : items.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={t('notification.empty')}
          style={{ padding: '24px 0' }}
        />
      ) : (
        <List
          dataSource={items}
          style={{ maxHeight: 420, overflowY: 'auto' }}
          renderItem={item => (
            <List.Item
              style={{
                padding: '10px 8px',
                background: item.isRead ? 'transparent' : '#f0f7ff',
                borderRadius: 6,
                marginBottom: 2,
                cursor: 'pointer',
                alignItems: 'flex-start',
              }}
              onClick={() => !item.isRead && onMarkRead(item.id)}
            >
              <Space align="start" style={{ width: '100%' }} size={10}>
                {/* unread dot */}
                <div style={{
                  width: 8, height: 8, borderRadius: '50%', marginTop: 6, flexShrink: 0,
                  background: item.isRead ? 'transparent' : '#1677ff',
                }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 4 }}>
                    <Space size={4} wrap>
                      <Tag
                        icon={item.eventType === 'Warning' ? <WarningOutlined /> : <InfoCircleOutlined />}
                        color={eventTypeColor(item.eventType)}
                        style={{ margin: 0, fontSize: 11 }}
                      >
                        {item.eventReason || item.eventType}
                      </Tag>
                      {item.clusterName && (
                        <Tag color="geekblue" style={{ margin: 0, fontSize: 11 }}>{item.clusterName}</Tag>
                      )}
                    </Space>
                    <Text type="secondary" style={{ fontSize: 11, flexShrink: 0 }}>
                      {timeAgo(item.triggeredAt)}
                    </Text>
                  </div>
                  <Tooltip title={item.message}>
                    <Text
                      style={{
                        display: 'block', marginTop: 4, fontSize: 12,
                        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                      }}
                    >
                      {item.message || item.ruleName}
                    </Text>
                  </Tooltip>
                  {item.namespace && (
                    <Text type="secondary" style={{ fontSize: 11 }}>
                      {item.namespace}{item.involvedObj ? ` · ${item.involvedObj}` : ''}
                    </Text>
                  )}
                </div>
              </Space>
            </List.Item>
          )}
        />
      )}
    </div>
  );
};

// ─── Main component ──────────────────────────────────────────────────────────

const NotificationPopover: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);

  // Lightweight unread count poll
  const fetchUnread = useCallback(async () => {
    try {
      const res = await notificationService.unreadCount();
      setUnreadCount((res.data as { unreadCount: number }).unreadCount ?? 0);
    } catch { /* silent */ }
  }, []);

  // Full list fetch when popover opens
  const fetchList = useCallback(async () => {
    setLoading(true);
    try {
      const res = await notificationService.list();
      const data = res.data as { items: NotificationItem[]; unreadCount: number };
      setItems(data.items ?? []);
      setUnreadCount(data.unreadCount ?? 0);
    } catch { /* silent */ } finally {
      setLoading(false);
    }
  }, []);

  // Initial load + polling — pauses when tab is hidden
  useEffect(() => { fetchUnread(); }, [fetchUnread]);
  useVisibilityInterval(fetchUnread, POLL_INTERVAL);

  const handleOpenChange = (v: boolean) => {
    setOpen(v);
    if (v) fetchList();
  };

  const handleMarkRead = async (id: number) => {
    setItems(prev => prev.map(n => n.id === id ? { ...n, isRead: true } : n));
    setUnreadCount(prev => Math.max(0, prev - 1));
    await notificationService.markRead(id);
  };

  const handleMarkAllRead = async () => {
    setItems(prev => prev.map(n => ({ ...n, isRead: true })));
    setUnreadCount(0);
    await notificationService.markAllRead();
  };

  return (
    <Popover
      open={open}
      onOpenChange={handleOpenChange}
      trigger="click"
      placement="bottomRight"
      arrow={false}
      styles={{ body: { padding: '14px 14px 10px' } }}
      content={
        <PopoverContent
          items={items}
          loading={loading}
          onMarkRead={handleMarkRead}
          onMarkAllRead={handleMarkAllRead}
        />
      }
    >
      <Badge count={unreadCount} size="small" offset={[-8, 10]}>
        <Button
          type="text"
          icon={<BellOutlined />}
          size="large"
          style={{ color: '#ffffff' }}
        />
      </Badge>
    </Popover>
  );
};

export default NotificationPopover;
