import React from 'react';
import { Result, Button, Space, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import {
  LockOutlined,
  FileUnknownOutlined,
  CloudServerOutlined,
  DisconnectOutlined,
} from '@ant-design/icons';

const { Text } = Typography;

export type ErrorStatus = 403 | 404 | 500 | 503 | 'network';

interface ErrorPageProps {
  status?: ErrorStatus;
  /** 覆蓋預設標題 */
  title?: string;
  /** 覆蓋預設副標題 */
  subTitle?: string;
  /** 顯示重試按鈕時的回呼 */
  onRetry?: () => void;
  /** 是否顯示「返回首頁」按鈕（預設 true） */
  showHome?: boolean;
  /** 是否顯示「返回上一頁」按鈕（預設 false） */
  showBack?: boolean;
  /** 錯誤參考編號（僅顯示，不暴露技術細節） */
  errorRef?: string;
}

interface ErrorConfig {
  resultStatus: 403 | 404 | 500 | 'error';
  icon?: React.ReactNode;
  title: string;
  subTitle: string;
}

const ERROR_CONFIG: Record<ErrorStatus, ErrorConfig> = {
  403: {
    resultStatus: 403,
    icon: <LockOutlined style={{ fontSize: 64, color: '#faad14' }} />,
    title: '無存取權限',
    subTitle: '您沒有存取此頁面的權限，請聯絡管理員或返回上一頁。',
  },
  404: {
    resultStatus: 404,
    icon: <FileUnknownOutlined style={{ fontSize: 64, color: '#1677ff' }} />,
    title: '頁面不存在',
    subTitle: '您造訪的頁面不存在或已被移除，請確認網址是否正確。',
  },
  500: {
    resultStatus: 500,
    icon: <CloudServerOutlined style={{ fontSize: 64, color: '#ff4d4f' }} />,
    title: '伺服器錯誤',
    subTitle: '伺服器發生內部錯誤，請稍後重試或聯絡管理員。',
  },
  503: {
    resultStatus: 500,
    icon: <CloudServerOutlined style={{ fontSize: 64, color: '#ff7a00' }} />,
    title: '服務暫時無法使用',
    subTitle: '服務目前正在維護或暫時不可用，請稍後再試。',
  },
  network: {
    resultStatus: 'error',
    icon: <DisconnectOutlined style={{ fontSize: 64, color: '#ff4d4f' }} />,
    title: '網路連線異常',
    subTitle: '無法連線至伺服器，請檢查您的網路連線後重試。',
  },
};

const ErrorPage: React.FC<ErrorPageProps> = ({
  status = 500,
  title,
  subTitle,
  onRetry,
  showHome = true,
  showBack = false,
  errorRef,
}) => {
  const navigate = useNavigate();
  const config = ERROR_CONFIG[status];

  const actions: React.ReactNode[] = [];

  if (onRetry) {
    actions.push(
      <Button key="retry" type="primary" onClick={onRetry}>
        重試
      </Button>
    );
  }
  if (showBack) {
    actions.push(
      <Button key="back" onClick={() => navigate(-1)}>
        返回上一頁
      </Button>
    );
  }
  if (showHome) {
    actions.push(
      <Button key="home" onClick={() => navigate('/')}>
        返回首頁
      </Button>
    );
  }

  return (
    <Result
      status={config.resultStatus}
      icon={config.icon}
      title={title ?? config.title}
      subTitle={
        <Space direction="vertical" size={4}>
          <span>{subTitle ?? config.subTitle}</span>
          {errorRef && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              錯誤參考編號：{errorRef}
            </Text>
          )}
        </Space>
      }
      extra={actions.length > 0 ? <Space>{actions}</Space> : undefined}
    />
  );
};

export default ErrorPage;
