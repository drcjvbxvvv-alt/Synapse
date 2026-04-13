import React, { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Button, Typography } from 'antd';
import { WarningOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ErrorPage from './ErrorPage';

const { Text } = Typography;

// ── Functional fallback components (hooks allowed here) ──────────────────────

const SectionFallback: React.FC<{ errorRef: string | null; onRetry: () => void }> = ({ errorRef, onRetry }) => {
  const { t } = useTranslation('common');
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: 240,
      padding: 32,
    }}>
      <div style={{
        background: '#fff',
        border: '1px solid #fee2e2',
        borderRadius: 16,
        padding: '32px 40px',
        textAlign: 'center',
        maxWidth: 420,
      }}>
        <div style={{
          width: 56,
          height: 56,
          borderRadius: '50%',
          background: '#fef2f2',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          margin: '0 auto 16px',
        }}>
          <WarningOutlined style={{ fontSize: 24, color: '#ef4444' }} />
        </div>
        <div style={{ fontSize: 16, fontWeight: 600, color: '#111827', marginBottom: 8 }}>
          {t('errorPage.boundary.sectionTitle')}
        </div>
        <div style={{ fontSize: 13, color: '#6b7280', marginBottom: 20, lineHeight: 1.6 }}>
          {t('errorPage.boundary.sectionDesc')}
        </div>
        {errorRef && (
          <Text
            type="secondary"
            style={{
              display: 'inline-block',
              marginBottom: 20,
              fontSize: 11,
              background: '#f3f4f6',
              padding: '2px 8px',
              borderRadius: 6,
              fontFamily: 'monospace',
            }}
          >
            {errorRef}
          </Text>
        )}
        <br />
        <Button type="primary" icon={<ReloadOutlined />} onClick={onRetry} style={{ borderRadius: 8 }}>
          {t('errorPage.actions.retry')}
        </Button>
      </div>
    </div>
  );
};

const PageFallback: React.FC<{ errorRef: string | null; onRetry: () => void }> = ({ errorRef, onRetry }) => {
  const { t } = useTranslation('common');
  return (
    <ErrorPage
      status={500}
      title={t('errorPage.boundary.pageTitle')}
      subTitle={t('errorPage.boundary.pageSubTitle')}
      errorRef={errorRef ?? undefined}
      onRetry={onRetry}
      retryLabel={t('errorPage.actions.reload')}
      showHome
      showBack={false}
    />
  );
};

// ── Class component ───────────────────────────────────────────────────────────

interface Props {
  children: ReactNode;
  fallbackType?: 'page' | 'section';
}

interface State {
  hasError: boolean;
  errorRef: string | null;
}

function generateErrorRef(): string {
  return `ERR-${Date.now().toString(36).toUpperCase()}-${Math.random().toString(36).slice(2, 6).toUpperCase()}`;
}

class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, errorRef: null };
  }

  static getDerivedStateFromError(): State {
    return { hasError: true, errorRef: generateErrorRef() };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('[ErrorBoundary]', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false, errorRef: null });
  };

  handleReload = () => {
    window.location.reload();
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallbackType === 'section') {
        return <SectionFallback errorRef={this.state.errorRef} onRetry={this.handleReset} />;
      }
      return <PageFallback errorRef={this.state.errorRef} onRetry={this.handleReload} />;
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
