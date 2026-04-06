import React, { useMemo, useState, useRef, useEffect } from 'react';
import { Card, Button, Space, Spin, Alert, Skeleton } from 'antd';
import { FullscreenOutlined, ReloadOutlined } from '@ant-design/icons';

interface GrafanaPanelProps {
  grafanaUrl: string;            // Grafana 外接地址（從系統設定獲取）
  dashboardUid: string;          // Dashboard UID
  panelId: number;               // Panel ID
  
  // 變數參數
  variables?: Record<string, string>;  // 如 { cluster: 'prod', namespace: 'default', pod: 'nginx-xxx' }
  
  // 時間範圍
  from?: string;                 // 開始時間，如 'now-1h'
  to?: string;                   // 結束時間，如 'now'
  refresh?: string;              // 重新整理間隔，如 '30s'
  
  // 樣式
  height?: number;               // 高度
  title?: string;                // 標題
  showToolbar?: boolean;         // 是否顯示工具欄
  theme?: 'light' | 'dark';      // 主題
  
  // 載入最佳化
  lazyLoad?: boolean;            // 是否啟用懶載入，預設 true
  loadDelay?: number;            // 延遲載入時間（毫秒），用於分批載入
  priority?: 'high' | 'normal' | 'low'; // 載入優先順序
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
  const [isVisible, setIsVisible] = useState(!lazyLoad); // 如果不啟用懶載入，預設可見
  const [shouldRender, setShouldRender] = useState(loadDelay === 0 && priority === 'high');
  const containerRef = useRef<HTMLDivElement>(null);

  // 延遲載入最佳化
  useEffect(() => {
    if (!isVisible || shouldRender) return;
    
    // 根據優先順序和延遲時間決定何時渲染
    const delay = priority === 'high' ? 0 : (priority === 'low' ? loadDelay + 500 : loadDelay);
    
    const timer = setTimeout(() => {
      setShouldRender(true);
    }, delay);

    return () => clearTimeout(timer);
  }, [isVisible, shouldRender, loadDelay, priority]);

  // 使用 IntersectionObserver 檢測可見性
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
            // 一旦可見就不需要再觀察了
            observer.disconnect();
          }
        });
      },
      {
        rootMargin: '200px', // 提前 200px 開始載入（增加預載入距離）
        threshold: 0.01,
      }
    );

    if (containerRef.current) {
      observer.observe(containerRef.current);
    }

    return () => observer.disconnect();
  }, [lazyLoad]);

  // 構建嵌入 URL
  const embedUrl = useMemo(() => {
    const params = new URLSearchParams({
      orgId: '1',
      from,
      to,
      theme,
      panelId: String(panelId),
    });

    // 新增變數參數
    Object.entries(variables).forEach(([key, value]) => {
      if (value) {
        params.append(`var-${key}`, value);
      }
    });

    // 新增重新整理間隔
    if (refresh) {
      params.append('refresh', refresh);
    }

    // Grafana 嵌入 URL 格式：/d-solo/{uid}/{slug}?{params}
    return `${grafanaUrl}/d-solo/${dashboardUid}/?${params.toString()}`;
  }, [grafanaUrl, dashboardUid, panelId, variables, from, to, refresh, theme]);

  // 完整 Dashboard URL（用於"在 Grafana 中開啟"）
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
                title="重新整理"
              />
              <Button 
                icon={<FullscreenOutlined />} 
                size="small" 
                onClick={handleOpenInGrafana}
                title="在 Grafana 中開啟"
              />
            </Space>
          )
        }
        styles={{ body: { padding: 0, position: 'relative', minHeight: height } }}
      >
        {/* 懶載入佔位符 - 不可見或未到渲染時間 */}
        {(!isVisible || !shouldRender) && (
          <div style={{ padding: 16, height }}>
            <Skeleton active paragraph={{ rows: Math.max(1, Math.floor(height / 60)) }} />
          </div>
        )}

        {/* 載入狀態 */}
        {isVisible && shouldRender && loading && !error && (
          <div style={{ 
            position: 'absolute', 
            top: '50%', 
            left: '50%', 
            transform: 'translate(-50%, -50%)',
            zIndex: 10
          }}>
            <Spin tip="載入圖表中..." />
          </div>
        )}
        
        {/* 錯誤狀態 */}
        {isVisible && shouldRender && error && (
          <Alert
            message="圖表載入失敗"
            description="請檢查 Grafana 服務是否正常執行，或重新整理重試"
            type="error"
            showIcon
            action={
              <Button size="small" onClick={handleRefresh}>
                重試
              </Button>
            }
            style={{ margin: 16 }}
          />
        )}
        
        {/* iframe - 只在可見且應該渲染時載入 */}
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
