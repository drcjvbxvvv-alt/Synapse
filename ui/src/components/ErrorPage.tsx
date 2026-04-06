import React from 'react';
import { Result, Button, Space, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import { Illus403, Illus404, Illus500, Illus503, IllusNetwork } from './ErrorIllustrations';

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
  /** 錯誤參考編號（不暴露技術細節，供回報用） */
  errorRef?: string;
}

interface ErrorConfig {
  illustration: React.ReactNode;
  title: string;
  subTitle: string;
}

const ERROR_CONFIG: Record<ErrorStatus, ErrorConfig> = {
  403: {
    illustration: <Illus403 />,
    title: '老兄，你沒有通行證',
    subTitle: '這裡是 VIP 專區。要入場的話，去找管理員申請授權吧，我也很無奈。',
  },
  404: {
    illustration: <Illus404 />,
    title: '這頁跑去哪了？',
    subTitle: '我們派了三個 Pod 去找，結果它們也迷路了。試試返回首頁？',
  },
  500: {
    illustration: <Illus500 />,
    title: '糟糕，伺服器中暑了',
    subTitle: 'On-Call 工程師已被緊急叫醒（他非常不高興）。請稍後重試，或者先去泡杯咖啡。',
  },
  503: {
    illustration: <Illus503 />,
    title: '服務去充電了',
    subTitle: '它說最近太累了，需要喘口氣。通常幾分鐘就好，留個便利貼等一下？',
  },
  network: {
    illustration: <IllusNetwork />,
    title: '網路跑掉了！',
    subTitle: '是貓咬斷網路線了嗎？還是有人欠費了？先確認一下連線狀態吧。',
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
      icon={config.illustration}
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
