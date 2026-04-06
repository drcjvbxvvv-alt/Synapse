import React, { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Result, Button } from 'antd';

interface Props {
  children: ReactNode;
  fallbackType?: 'page' | 'section';
}

interface State {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallbackType === 'section') {
        return (
          <Result
            status="error"
            title="元件載入失敗"
            subTitle={this.state.error?.message}
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
            title="頁面出錯了"
            subTitle={this.state.error?.message || '發生了未知錯誤，請重新整理頁面重試'}
            extra={[
              <Button type="primary" key="retry" onClick={this.handleReset}>
                重試
              </Button>,
              <Button key="home" onClick={() => window.location.href = '/'}>
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
