/**
 * ErrorState — 行內錯誤狀態元件
 *
 * 用於 Card / 區塊內的資料載入失敗狀態。
 * 與 ErrorPage（全頁錯誤）有所區別：此元件為行內使用，體積較小。
 */
import React from 'react';
import { Button, Space } from 'antd';
import {
  WifiOutlined,
  LockOutlined,
  DisconnectOutlined,
  WarningOutlined,
  ReloadOutlined,
} from '@ant-design/icons';

export type ErrorStateType = 'network' | 'permission' | 'offline' | 'unknown';

interface ErrorStateProps {
  type?: ErrorStateType;
  title?: string;
  description?: string;
  onRetry?: () => void;
  extra?: React.ReactNode;
  compact?: boolean;
}

interface TypeConfig {
  icon: React.ReactNode;
  title: string;
  description: string;
  iconBg: string;
  iconColor: string;
}

const TYPE_CONFIG: Record<ErrorStateType, TypeConfig> = {
  network: {
    icon: <WifiOutlined style={{ fontSize: 22 }} />,
    title: '資料載入失敗',
    description: '請求失敗，請確認網路狀態後重試。',
    iconBg: '#fef2f2',
    iconColor: '#ef4444',
  },
  permission: {
    icon: <LockOutlined style={{ fontSize: 22 }} />,
    title: '無存取權限',
    description: '您沒有存取此資源的權限，請聯絡管理員。',
    iconBg: '#fffbeb',
    iconColor: '#f59e0b',
  },
  offline: {
    icon: <DisconnectOutlined style={{ fontSize: 22 }} />,
    title: '叢集無法連線',
    description: '無法連線至叢集 API，請確認叢集狀態。',
    iconBg: '#fef2f2',
    iconColor: '#ef4444',
  },
  unknown: {
    icon: <WarningOutlined style={{ fontSize: 22 }} />,
    title: '發生錯誤',
    description: '操作發生未預期的錯誤，請重試或聯絡管理員。',
    iconBg: '#fffbeb',
    iconColor: '#f59e0b',
  },
};

const ErrorState: React.FC<ErrorStateProps> = ({
  type = 'network',
  title,
  description,
  onRetry,
  extra,
  compact = false,
}) => {
  const config = TYPE_CONFIG[type];

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      padding: compact ? '24px 16px' : '48px 24px',
      textAlign: 'center',
    }}>
      {/* 圖示圓圈 */}
      <div style={{
        width: 52,
        height: 52,
        borderRadius: '50%',
        background: config.iconBg,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        marginBottom: 16,
        color: config.iconColor,
        border: `1px solid ${config.iconColor}20`,
      }}>
        {config.icon}
      </div>

      {/* 標題 */}
      <div style={{
        fontSize: compact ? 14 : 16,
        fontWeight: 600,
        color: '#111827',
        marginBottom: 8,
      }}>
        {title ?? config.title}
      </div>

      {/* 說明 */}
      <div style={{
        fontSize: 13,
        color: '#6b7280',
        lineHeight: 1.6,
        maxWidth: 320,
        marginBottom: (onRetry || extra) ? 20 : 0,
      }}>
        {description ?? config.description}
      </div>

      {/* 操作按鈕 */}
      {(onRetry || extra) && (
        <Space>
          {onRetry && (
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={onRetry}
              size={compact ? 'small' : 'middle'}
              style={{ borderRadius: 8 }}
            >
              重試
            </Button>
          )}
          {extra}
        </Space>
      )}
    </div>
  );
};

export default ErrorState;
