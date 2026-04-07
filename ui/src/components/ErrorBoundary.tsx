import React, { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Button, Space, Typography } from 'antd';
import { WarningOutlined, ReloadOutlined, HomeOutlined } from '@ant-design/icons';
import ErrorPage from './ErrorPage';

const { Text } = Typography;

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

  render() {
    if (this.state.hasError) {
      // Section fallback：行內卡片式錯誤（用於 Terminal、LogCenter 等局部元件）
      if (this.props.fallbackType === 'section') {
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
                元件載入失敗
              </div>
              <div style={{ fontSize: 13, color: '#6b7280', marginBottom: 20, lineHeight: 1.6 }}>
                此元件發生錯誤，請重試或重新整理頁面。
              </div>
              {this.state.errorRef && (
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
                  {this.state.errorRef}
                </Text>
              )}
              <br />
              <Button type="primary" icon={<ReloadOutlined />} onClick={this.handleReset} style={{ borderRadius: 8 }}>
                重試
              </Button>
            </div>
          </div>
        );
      }

      // Page fallback：全頁錯誤 → 使用自製 ErrorPage
      return (
        <ErrorPage
          status={500}
          errorRef={this.state.errorRef ?? undefined}
          onRetry={this.handleReset}
          showHome
          showBack={false}
        />
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
