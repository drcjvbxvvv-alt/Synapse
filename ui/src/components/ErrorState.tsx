/**
 * ErrorState — 行內錯誤狀態元件
 *
 * 用於 Card / 區塊內的資料載入失敗狀態。
 * 與 ErrorPage（全頁錯誤）有所區別：此元件為行內使用，體積較小。
 *
 * types:
 *   - network    : 網路請求失敗（預設）
 *   - permission : 無存取權限（403）
 *   - offline    : 叢集離線 / API 不可達
 *   - unknown    : 未知錯誤
 */
import React from 'react';
import { Result, Button, Space } from 'antd';
import {
  WifiOutlined,
  LockOutlined,
  DisconnectOutlined,
  WarningOutlined,
} from '@ant-design/icons';

export type ErrorStateType = 'network' | 'permission' | 'offline' | 'unknown';

interface ErrorStateProps {
  /** 錯誤類型，決定預設圖示與文字 */
  type?: ErrorStateType;
  /** 覆蓋預設標題 */
  title?: string;
  /** 覆蓋預設說明文字 */
  description?: string;
  /** 重試回呼，有值時顯示「重試」按鈕 */
  onRetry?: () => void;
  /** 額外行動按鈕 */
  extra?: React.ReactNode;
  /** 緊湊模式（減少 padding） */
  compact?: boolean;
}

interface TypeConfig {
  icon: React.ReactNode;
  title: string;
  description: string;
}

const TYPE_CONFIG: Record<ErrorStateType, TypeConfig> = {
  network: {
    icon: <WifiOutlined style={{ fontSize: 40, color: '#ff4d4f' }} />,
    title: '資料載入失敗',
    description: '請求失敗，請確認網路狀態後重試。',
  },
  permission: {
    icon: <LockOutlined style={{ fontSize: 40, color: '#faad14' }} />,
    title: '無存取權限',
    description: '您沒有存取此資源的權限，請聯絡管理員。',
  },
  offline: {
    icon: <DisconnectOutlined style={{ fontSize: 40, color: '#ff4d4f' }} />,
    title: '叢集無法連線',
    description: '無法連線至叢集 API，請確認叢集狀態。',
  },
  unknown: {
    icon: <WarningOutlined style={{ fontSize: 40, color: '#faad14' }} />,
    title: '發生錯誤',
    description: '操作發生未預期的錯誤，請重試或聯絡管理員。',
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

  const actions = (
    <Space>
      {onRetry && (
        <Button type="primary" onClick={onRetry}>
          重試
        </Button>
      )}
      {extra}
    </Space>
  );

  return (
    <Result
      icon={config.icon}
      title={title ?? config.title}
      subTitle={description ?? config.description}
      extra={onRetry || extra ? actions : undefined}
      style={compact ? { padding: '24px 16px' } : undefined}
    />
  );
};

export default ErrorState;
