import React, { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Result, Button } from 'antd';

interface Props {
  children: ReactNode;
  fallbackType?: 'page' | 'section';
}

interface State {
  hasError: boolean;
  errorRef: string | null;
}

/** 產生隨機參考編號供使用者回報，不暴露技術細節 */
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
    // 僅記錄到主控台，不暴露給使用者介面
    console.error('[ErrorBoundary]', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false, errorRef: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallbackType === 'section') {
        return (
          <Result
            status="error"
            title="元件載入失敗"
            subTitle="此元件發生錯誤，請重試或重新整理頁面。"
            extra={
              <Button type="primary" onClick={this.handleReset}>
                重試
              </Button>
            }
          />
        );
      }

      return (
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
          <Result
            status="500"
            title="頁面發生錯誤"
            subTitle={
              <>
                發生了未預期的錯誤，請重新整理頁面或聯絡管理員。
                {this.state.errorRef && (
                  <div style={{ marginTop: 4, fontSize: 12, color: '#999' }}>
                    參考編號：{this.state.errorRef}
                  </div>
                )}
              </>
            }
            extra={[
              <Button type="primary" key="retry" onClick={this.handleReset}>
                重試
              </Button>,
              <Button key="home" onClick={() => { window.location.href = '/'; }}>
                返回首頁
              </Button>,
            ]}
          />
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
