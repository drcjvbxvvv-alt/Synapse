/**
 * EmptyState — 統一空狀態元件
 *
 * 取代各頁面散落的 `<Empty image={...} description={...}>` 用法。
 * 提供四種內建類型，可自訂 icon、描述文字與行動按鈕。
 *
 * types:
 *   - no-data        : 此叢集目前沒有資源（預設）
 *   - no-permission  : 您沒有此資源的存取權限
 *   - offline        : 無法連線至叢集 API
 *   - not-configured : 功能尚未設定，引導至設定頁
 */
import React from 'react';
import { Empty, Button, Space, Typography } from 'antd';
import {
  LockOutlined,
  DisconnectOutlined,
  SettingOutlined,
} from '@ant-design/icons';

const { Text } = Typography;

export type EmptyStateType = 'no-data' | 'no-permission' | 'offline' | 'not-configured';

interface EmptyStateAction {
  label: string;
  onClick: () => void;
  type?: 'primary' | 'default';
  icon?: React.ReactNode;
}

interface EmptyStateProps {
  /** 內建類型，決定預設圖示與文字 */
  type?: EmptyStateType;
  /** 覆蓋預設主標題 */
  title?: string;
  /** 覆蓋預設說明文字 */
  description?: string;
  /** 自訂圖示（覆蓋 type 對應的預設圖示） */
  icon?: React.ReactNode | string;
  /** 行動按鈕，最多建議 2 個 */
  actions?: EmptyStateAction[];
  /** 外層容器的 style */
  style?: React.CSSProperties;
}

interface TypeConfig {
  icon: React.ReactNode | string;
  title: string;
  description: string;
}

const TYPE_CONFIG: Record<EmptyStateType, TypeConfig> = {
  'no-data': {
    icon: Empty.PRESENTED_IMAGE_DEFAULT,
    title: '暫無資料',
    description: '',
  },
  'no-permission': {
    icon: <LockOutlined style={{ fontSize: 48, color: '#faad14' }} />,
    title: '無存取權限',
    description: '您沒有存取此資源的權限，請聯絡管理員。',
  },
  'offline': {
    icon: <DisconnectOutlined style={{ fontSize: 48, color: '#ff4d4f' }} />,
    title: '叢集無法連線',
    description: '無法連線至叢集 API，請確認叢集狀態與網路連線。',
  },
  'not-configured': {
    icon: <SettingOutlined style={{ fontSize: 48, color: '#1677ff' }} />,
    title: '尚未設定',
    description: '此功能需要先完成設定才能使用。',
  },
};

const EmptyState: React.FC<EmptyStateProps> = React.memo(function EmptyState({
  type = 'no-data',
  title,
  description,
  icon,
  actions,
  style,
}: EmptyStateProps) {
  const config = TYPE_CONFIG[type];
  const displayIcon = icon ?? config.icon;

  // When only description is passed (no explicit title), treat it as the sole message.
  // When title is passed explicitly, show both title and description.
  const displayTitle = title ?? (description !== undefined ? description : config.title);
  const displayDescription = title !== undefined
    ? (description ?? config.description)
    : (description !== undefined ? '' : config.description);

  return (
    <Empty
      style={{ padding: '32px 0', ...style }}
      image={displayIcon}
      imageStyle={typeof displayIcon !== 'string' ? { width: 124, height: 92, margin: '0 auto' } : undefined}
      description={
        <Space direction="vertical" size={4}>
          <Text strong style={{ fontSize: 15, color: '#8a94b0' }}>{displayTitle}</Text>
          {displayDescription && <Text style={{ color: '#8a94b0' }}>{displayDescription}</Text>}
        </Space>
      }
    >
      {actions && actions.length > 0 && (
        <Space>
          {actions.map((action, i) => (
            <Button
              key={i}
              type={action.type ?? (i === 0 ? 'primary' : 'default')}
              icon={action.icon}
              onClick={action.onClick}
            >
              {action.label}
            </Button>
          ))}
        </Space>
      )}
    </Empty>
  );
});

export default EmptyState;
