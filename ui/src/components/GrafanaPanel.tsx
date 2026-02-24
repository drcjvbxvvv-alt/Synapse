import React, { useMemo, useState, useRef, useEffect } from 'react';
import { Card, Button, Space, Spin, Alert, Skeleton } from 'antd';
import { FullscreenOutlined, ReloadOutlined } from '@ant-design/icons';

interface GrafanaPanelProps {
  grafanaUrl: string;            // Grafana 外置地址（从系统设置获取）
  dashboardUid: string;          // Dashboard UID
  panelId: number;               // Panel ID
  
  // 变量参数
  variables?: Record<string, string>;  // 如 { cluster: 'prod', namespace: 'default', pod: 'nginx-xxx' }
  
  // 时间范围
  from?: string;                 // 开始时间，如 'now-1h'
  to?: string;                   // 结束时间，如 'now'
  refresh?: string;              // 刷新间隔，如 '30s'
  
  // 样式
  height?: number;               // 高度
  title?: string;                // 标题
  showToolbar?: boolean;         // 是否显示工具栏
  theme?: 'light' | 'dark';      // 主题
  
  // 加载优化
  lazyLoad?: boolean;            // 是否启用懒加载，默认 true
  loadDelay?: number;            // 延迟加载时间（毫秒），用于分批加载
  priority?: 'high' | 'normal' | 'low'; // 加载优先级
}

const GrafanaPanel: React.FC<GrafanaPanelProps> = ({
  grafanaUrl,
  dashboardUid,
  panelId,
  variables = {},
  from = 'now-1h',
  to = 'now',
  refresh,
  height = 300,
  title,
  showToolbar = true,
  theme = 'light',
  lazyLoad = true,
  loadDelay = 0,
  priority = 'normal',
}) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [iframeKey, setIframeKey] = useState(0);
  const [isVisible, setIsVisible] = useState(!lazyLoad); // 如果不启用懒加载，默认可见
  const [shouldRender, setShouldRender] = useState(loadDelay === 0 && priority === 'high');
  const containerRef = useRef<HTMLDivElement>(null);

  // 延迟加载优化
  useEffect(() => {
    if (!isVisible || shouldRender) return;
    
    // 根据优先级和延迟时间决定何时渲染
    const delay = priority === 'high' ? 0 : (priority === 'low' ? loadDelay + 500 : loadDelay);
    
    const timer = setTimeout(() => {
      setShouldRender(true);
    }, delay);

    return () => clearTimeout(timer);
  }, [isVisible, shouldRender, loadDelay, priority]);

  // 使用 IntersectionObserver 检测可见性
  useEffect(() => {
    if (!lazyLoad) {
      setIsVisible(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setIsVisible(true);
            // 一旦可见就不需要再观察了
            observer.disconnect();
          }
        });
      },
      {
        rootMargin: '200px', // 提前 200px 开始加载（增加预加载距离）
        threshold: 0.01,
      }
    );

    if (containerRef.current) {
      observer.observe(containerRef.current);
    }

    return () => observer.disconnect();
  }, [lazyLoad]);

  // 构建嵌入 URL
  const embedUrl = useMemo(() => {
    const params = new URLSearchParams({
      orgId: '1',
      from,
      to,
      theme,
      panelId: String(panelId),
    });

    // 添加变量参数
    Object.entries(variables).forEach(([key, value]) => {
      if (value) {
        params.append(`var-${key}`, value);
      }
    });

    // 添加刷新间隔
    if (refresh) {
      params.append('refresh', refresh);
    }

    // Grafana 嵌入 URL 格式：/d-solo/{uid}/{slug}?{params}
    return `${grafanaUrl}/d-solo/${dashboardUid}/?${params.toString()}`;
  }, [grafanaUrl, dashboardUid, panelId, variables, from, to, refresh, theme]);

  // 完整 Dashboard URL（用于"在 Grafana 中打开"）
  const fullDashboardUrl = useMemo(() => {
    const params = new URLSearchParams({
      orgId: '1',
      from,
      to,
    });
    Object.entries(variables).forEach(([key, value]) => {
      if (value) {
        params.append(`var-${key}`, value);
      }
    });
    return `${grafanaUrl}/d/${dashboardUid}/?${params.toString()}`;
  }, [grafanaUrl, dashboardUid, variables, from, to]);

  const handleRefresh = () => {
    setLoading(true);
    setError(false);
    setIframeKey(prev => prev + 1);
  };

  const handleOpenInGrafana = () => {
    window.open(fullDashboardUrl, '_blank');
  };

  return (
    <div ref={containerRef}>
      <Card
        title={title}
        size="small"
        extra={
          showToolbar && isVisible && (
            <Space>
              <Button 
                icon={<ReloadOutlined />} 
                size="small" 
                onClick={handleRefresh}
                title="刷新"
              />
              <Button 
                icon={<FullscreenOutlined />} 
                size="small" 
                onClick={handleOpenInGrafana}
                title="在 Grafana 中打开"
              />
            </Space>
          )
        }
        styles={{ body: { padding: 0, position: 'relative', minHeight: height } }}
      >
        {/* 懒加载占位符 - 不可见或未到渲染时间 */}
        {(!isVisible || !shouldRender) && (
          <div style={{ padding: 16, height }}>
            <Skeleton active paragraph={{ rows: Math.max(1, Math.floor(height / 60)) }} />
          </div>
        )}

        {/* 加载状态 */}
        {isVisible && shouldRender && loading && !error && (
          <div style={{ 
            position: 'absolute', 
            top: '50%', 
            left: '50%', 
            transform: 'translate(-50%, -50%)',
            zIndex: 10
          }}>
            <Spin tip="加载图表中..." />
          </div>
        )}
        
        {/* 错误状态 */}
        {isVisible && shouldRender && error && (
          <Alert
            message="图表加载失败"
            description="请检查 Grafana 服务是否正常运行，或刷新重试"
            type="error"
            showIcon
            action={
              <Button size="small" onClick={handleRefresh}>
                重试
              </Button>
            }
            style={{ margin: 16 }}
          />
        )}
        
        {/* iframe - 只在可见且应该渲染时加载 */}
        {isVisible && shouldRender && (
          <iframe
            key={iframeKey}
            src={embedUrl}
            width="100%"
            height={height}
            frameBorder="0"
            style={{ border: 'none', display: error ? 'none' : 'block' }}
            title={title || `Grafana Panel ${panelId}`}
            onLoad={() => setLoading(false)}
            onError={() => {
              setLoading(false);
              setError(true);
            }}
          />
        )}
      </Card>
    </div>
  );
};

export default GrafanaPanel;
