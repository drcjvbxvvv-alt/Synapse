import React from 'react';
import { Button, Space, Typography, Tag } from 'antd';
import { useNavigate } from 'react-router-dom';
import { HomeOutlined, ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons';
import { Illus403, Illus404, Illus500, Illus503, IllusNetwork } from './ErrorIllustrations';

const { Title, Text, Paragraph } = Typography;

export type ErrorStatus = 403 | 404 | 500 | 503 | 'network';

interface ErrorPageProps {
  status?: ErrorStatus;
  title?: string;
  subTitle?: string;
  onRetry?: () => void;
  retryLabel?: string;
  showHome?: boolean;
  showBack?: boolean;
  errorRef?: string;
}

interface ErrorConfig {
  illustration: React.ReactNode;
  code: string;
  title: string;
  subTitle: string;
  tagColor: string;
  tagLabel: string;
  accentColor: string;
  bgGradient: string;
}

const ERROR_CONFIG: Record<ErrorStatus, ErrorConfig> = {
  403: {
    illustration: <Illus403 />,
    code: '403',
    title: '老兄，你沒有通行證',
    subTitle: '這裡是 VIP 專區。要入場的話，去找管理員申請授權吧，我也很無奈。',
    tagColor: 'warning',
    tagLabel: 'Forbidden',
    accentColor: '#f59e0b',
    bgGradient: 'radial-gradient(ellipse at 60% 20%, #fffbeb 0%, #fef3c7 40%, transparent 70%)',
  },
  404: {
    illustration: <Illus404 />,
    code: '404',
    title: '這頁跑去哪了？',
    subTitle: '我們派了三個 Pod 去找，結果它們也迷路了。試試返回首頁？',
    tagColor: 'blue',
    tagLabel: 'Not Found',
    accentColor: '#3b82f6',
    bgGradient: 'radial-gradient(ellipse at 60% 20%, #eff6ff 0%, #dbeafe 40%, transparent 70%)',
  },
  500: {
    illustration: <Illus500 />,
    code: '500',
    title: '糟糕，伺服器中暑了',
    subTitle: 'On-Call 工程師已被緊急叫醒（他非常不高興）。請稍後重試，或者先去泡杯咖啡。',
    tagColor: 'error',
    tagLabel: 'Internal Server Error',
    accentColor: '#ef4444',
    bgGradient: 'radial-gradient(ellipse at 60% 20%, #fff1f2 0%, #fee2e2 40%, transparent 70%)',
  },
  503: {
    illustration: <Illus503 />,
    code: '503',
    title: '服務去充電了',
    subTitle: '它說最近太累了，需要喘口氣。通常幾分鐘就好，留個便利貼等一下？',
    tagColor: 'processing',
    tagLabel: 'Service Unavailable',
    accentColor: '#6366f1',
    bgGradient: 'radial-gradient(ellipse at 60% 20%, #eef2ff 0%, #e0e7ff 40%, transparent 70%)',
  },
  network: {
    illustration: <IllusNetwork />,
    code: '—',
    title: '網路跑掉了！',
    subTitle: '是貓咬斷網路線了嗎？還是有人欠費了？先確認一下連線狀態吧。',
    tagColor: 'default',
    tagLabel: 'Network Error',
    accentColor: '#6b7280',
    bgGradient: 'radial-gradient(ellipse at 60% 20%, #f9fafb 0%, #f3f4f6 40%, transparent 70%)',
  },
};

const ErrorPage: React.FC<ErrorPageProps> = ({
  status = 500,
  title,
  subTitle,
  onRetry,
  retryLabel = '重試',
  showHome = true,
  showBack = false,
  errorRef,
}) => {
  const navigate = useNavigate();
  const config = ERROR_CONFIG[status];

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '40px 24px',
        background: '#f8fafc',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Background decorative blobs */}
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background: config.bgGradient,
          pointerEvents: 'none',
        }}
      />
      <div
        style={{
          position: 'absolute',
          bottom: '-80px',
          left: '-80px',
          width: 320,
          height: 320,
          borderRadius: '50%',
          background: `${config.accentColor}08`,
          pointerEvents: 'none',
        }}
      />
      <div
        style={{
          position: 'absolute',
          top: '-60px',
          right: '-60px',
          width: 240,
          height: 240,
          borderRadius: '50%',
          background: `${config.accentColor}06`,
          pointerEvents: 'none',
        }}
      />

      {/* Card */}
      <div
        style={{
          position: 'relative',
          background: 'rgba(255,255,255,0.88)',
          backdropFilter: 'blur(12px)',
          borderRadius: 24,
          boxShadow: '0 8px 40px rgba(0,0,0,0.08), 0 1px 3px rgba(0,0,0,0.06)',
          border: '1px solid rgba(255,255,255,0.7)',
          padding: '52px 64px 48px',
          maxWidth: 520,
          width: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          textAlign: 'center',
          gap: 0,
        }}
      >
        {/* Error code badge */}
        <div style={{ marginBottom: 8 }}>
          <Tag
            color={config.tagColor}
            style={{
              fontSize: 12,
              fontWeight: 600,
              letterSpacing: '0.05em',
              padding: '2px 10px',
              borderRadius: 20,
            }}
          >
            {config.tagLabel}
          </Tag>
        </div>

        {/* Error code number */}
        <div
          style={{
            fontSize: 80,
            fontWeight: 900,
            lineHeight: 1,
            letterSpacing: '-4px',
            color: config.accentColor,
            opacity: 0.15,
            marginBottom: -20,
            fontFamily: '"SF Pro Display", -apple-system, BlinkMacSystemFont, sans-serif',
            userSelect: 'none',
          }}
        >
          {config.code}
        </div>

        {/* Illustration */}
        <div
          style={{
            position: 'relative',
            zIndex: 1,
            marginBottom: 8,
            filter: 'drop-shadow(0 8px 16px rgba(0,0,0,0.08))',
          }}
        >
          {config.illustration}
        </div>

        {/* Title */}
        <Title
          level={3}
          style={{
            margin: '0 0 12px',
            fontSize: 22,
            fontWeight: 700,
            color: '#111827',
          }}
        >
          {title ?? config.title}
        </Title>

        {/* Subtitle */}
        <Paragraph
          style={{
            margin: '0 0 32px',
            fontSize: 14,
            color: '#6b7280',
            lineHeight: 1.7,
            maxWidth: 340,
          }}
        >
          {subTitle ?? config.subTitle}
          {errorRef && (
            <>
              <br />
              <Text
                type="secondary"
                style={{
                  fontSize: 11,
                  display: 'inline-block',
                  marginTop: 8,
                  background: '#f3f4f6',
                  padding: '2px 8px',
                  borderRadius: 6,
                  fontFamily: 'monospace',
                }}
              >
                ref: {errorRef}
              </Text>
            </>
          )}
        </Paragraph>

        {/* Divider */}
        <div
          style={{
            width: 48,
            height: 3,
            borderRadius: 2,
            background: `linear-gradient(90deg, ${config.accentColor}, ${config.accentColor}40)`,
            marginBottom: 32,
          }}
        />

        {/* Action buttons */}
        <Space size={12} wrap style={{ justifyContent: 'center' }}>
          {onRetry && (
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={onRetry}
              size="large"
              style={{ borderRadius: 10, fontWeight: 600 }}
            >
              {retryLabel}
            </Button>
          )}
          {showBack && (
            <Button
              icon={<ArrowLeftOutlined />}
              onClick={() => navigate(-1)}
              size="large"
              style={{ borderRadius: 10 }}
            >
              返回上一頁
            </Button>
          )}
          {showHome && (
            <Button
              type={onRetry ? 'default' : 'primary'}
              icon={<HomeOutlined />}
              onClick={() => navigate('/')}
              size="large"
              style={{
                borderRadius: 10,
                fontWeight: 600,
                ...(onRetry ? {} : {
                  background: config.accentColor,
                  borderColor: config.accentColor,
                }),
              }}
            >
              返回首頁
            </Button>
          )}
        </Space>
      </div>
    </div>
  );
};

export default ErrorPage;
