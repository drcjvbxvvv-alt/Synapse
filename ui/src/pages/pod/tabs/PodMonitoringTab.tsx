/**
 * Pod 監控 Tab - 使用整個 Grafana Dashboard 嵌入
 * 相比多 Panel 分別嵌入，整體嵌入載入更快
 */
import React, { useState, useMemo, useCallback } from 'react';
import { Card, Space, Button, Switch, Spin, DatePicker, Popover, Divider, Typography, Alert } from 'antd';
import { ReloadOutlined, ClockCircleOutlined } from '@ant-design/icons';
import type { Dayjs } from 'dayjs';
import EmptyState from '@/components/EmptyState';
import { generateDataSourceUID } from '../../../config/grafana.config';
import { useTranslation } from 'react-i18next';
import { useGrafanaUrl } from '../../../hooks/useGrafanaUrl';

const { Text } = Typography;

const DASHBOARD_UID = 'synapse-pod-detail';

// Grafana 風格的時間範圍選項
const TIME_RANGE_OPTIONS = [
  {
    label: '快速選擇',
    options: [
      { value: '5m', label: 'Last 5 minutes' },
      { value: '15m', label: 'Last 15 minutes' },
      { value: '30m', label: 'Last 30 minutes' },
      { value: '1h', label: 'Last 1 hour' },
      { value: '3h', label: 'Last 3 hours' },
      { value: '6h', label: 'Last 6 hours' },
      { value: '12h', label: 'Last 12 hours' },
      { value: '24h', label: 'Last 24 hours' },
    ],
  },
  {
    label: '更長時間',
    options: [
      { value: '2d', label: 'Last 2 days' },
      { value: '7d', label: 'Last 7 days' },
      { value: '30d', label: 'Last 30 days' },
    ],
  },
];

interface PodMonitoringTabProps {
  clusterId: string;
  clusterName?: string;
  namespace: string;
  podName: string;
}

const PodMonitoringTab: React.FC<PodMonitoringTabProps> = ({
  clusterId,
  clusterName,
  namespace,
  podName,
}) => {
const { grafanaUrl, loading: grafanaUrlLoading } = useGrafanaUrl();
const { t } = useTranslation(['pod', 'common']);
const [timeRange, setTimeRange] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  const [timePickerOpen, setTimePickerOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  
  // 自定義時間範圍狀態
  const [isCustomRange, setIsCustomRange] = useState(false);
  const [customFromTime, setCustomFromTime] = useState<Dayjs | null>(null);
  const [customToTime, setCustomToTime] = useState<Dayjs | null>(null);

  // 根據叢集名生成資料來源 UID
  const dataSourceUid = clusterName ? generateDataSourceUID(clusterName) : '';

  // 獲取時間範圍參數
  const getFromTime = useCallback(() => {
    if (isCustomRange && customFromTime) {
      return customFromTime.valueOf().toString();
    }
    return `now-${timeRange}`;
  }, [isCustomRange, customFromTime, timeRange]);

  const getToTime = useCallback(() => {
    if (isCustomRange && customToTime) {
      return customToTime.valueOf().toString();
    }
    return 'now';
  }, [isCustomRange, customToTime]);

  // 獲取顯示的時間範圍文字
  const getTimeRangeDisplay = () => {
    if (isCustomRange && customFromTime && customToTime) {
      return `${customFromTime.format('MM-DD HH:mm')} to ${customToTime.format('MM-DD HH:mm')}`;
    }
    const option = TIME_RANGE_OPTIONS.flatMap(g => g.options).find(o => o.value === timeRange);
    return option?.label || 'Last 1 hour';
  };

  // 構建完整 Dashboard 嵌入 URL
  const dashboardUrl = useMemo(() => {
    const params = new URLSearchParams({
      orgId: '1',
      from: getFromTime(),
      to: getToTime(),
      theme: 'light',
    });

    // 新增資料來源變數
    if (dataSourceUid) {
      params.append('var-DS_PROMETHEUS', dataSourceUid);
    }
    
    // 新增 Pod 相關變數
    params.append('var-namespace', namespace);
    params.append('var-podname', podName);
    params.append('var-Interface', 'eth0');
    params.append('var-Intervals', '2m');

    // 新增自動重新整理
    if (autoRefresh) {
      params.append('refresh', '30s');
    }

    // 完全 kiosk 模式：隱藏側邊欄和頂部導航欄
    return `${grafanaUrl}/d/${DASHBOARD_UID}/?${params.toString()}&kiosk`;
  }, [grafanaUrl, getFromTime, getToTime, dataSourceUid, namespace, podName, autoRefresh]);

  const handleRefresh = () => {
    setLoading(true);
    setRefreshKey(prev => prev + 1);
  };

  const handleIframeLoad = () => {
    setLoading(false);
  };

  // 應用自定義時間範圍
  const applyCustomRange = () => {
    if (customFromTime && customToTime) {
      setIsCustomRange(true);
      setTimePickerOpen(false);
      handleRefresh();
    }
  };

  // 選擇快速時間範圍
  const handleQuickRangeSelect = (value: string) => {
    setTimeRange(value);
    setIsCustomRange(false);
    setTimePickerOpen(false);
    handleRefresh();
  };

  // 檢查必要的參數
  if (!clusterName) {
    return (
      <EmptyState
        description={t('pod:terminal.cannotGetCluster')}
      />
    );
  }

  // 時間選擇器 Popover 內容
  const timePickerContent = (
    <div style={{ display: 'flex', gap: 16, padding: 8 }}>
      <div style={{ width: 240 }}>
        <Text strong style={{ marginBottom: 8, display: 'block' }}>Absolute time range</Text>
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>From</Text>
          <DatePicker
            showTime
            value={customFromTime}
            onChange={setCustomFromTime}
            style={{ width: '100%', marginTop: 4 }}
            placeholder={t('logs:center.startTime')}
            format="YYYY-MM-DD HH:mm:ss"
          />
        </div>
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>To</Text>
          <DatePicker
            showTime
            value={customToTime}
            onChange={setCustomToTime}
            style={{ width: '100%', marginTop: 4 }}
            placeholder={t('logs:center.endTime')}
            format="YYYY-MM-DD HH:mm:ss"
          />
        </div>
        <Button
          type="primary"
          block
          onClick={applyCustomRange}
          disabled={!customFromTime || !customToTime}
        >
          Apply time range
        </Button>
      </div>

      <Divider type="vertical" style={{ height: 'auto' }} />

      <div style={{ width: 160 }}>
        {TIME_RANGE_OPTIONS.map(group => (
          <div key={group.label} style={{ marginBottom: 12 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>{group.label}</Text>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginTop: 4 }}>
              {group.options.map(opt => (
                <Button
                  key={opt.value}
                  type={!isCustomRange && timeRange === opt.value ? 'primary' : 'text'}
                  size="small"
                  style={{ textAlign: 'left', justifyContent: 'flex-start' }}
                  onClick={() => handleQuickRangeSelect(opt.value)}
                >
                  {opt.label}
                </Button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );

  return (
    <Card
      title={t('pod:terminal.monitorChart')}
      extra={
        <Space>
          <Popover
            content={timePickerContent}
            trigger="click"
            open={timePickerOpen}
            onOpenChange={setTimePickerOpen}
            placement="bottomRight"
          >
            <Button icon={<ClockCircleOutlined />} style={{ minWidth: 180 }}>
              {getTimeRangeDisplay()}
            </Button>
          </Popover>
          <Space>
            <span>{t('pod:terminal.autoRefresh')}</span>
            <Switch
              checked={autoRefresh}
              onChange={(checked) => {
                setAutoRefresh(checked);
                handleRefresh();
              }}
              checkedChildren={t('pod:terminal.on')}
              unCheckedChildren={t('pod:terminal.off')}
            />
          </Space>
          <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Space>
      }
      styles={{ body: { padding: 0, position: 'relative', minHeight: 800 } }}
    >
      {grafanaUrlLoading ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin size="large" /></div>
      ) : !grafanaUrl ? (
        <Alert
          message="Grafana 未配置"
          description="請在「系統設定 → Grafana 設定」中配置 Grafana 地址，然後重新整理頁面。"
          type="warning"
          showIcon
          style={{ margin: 24 }}
        />
      ) : (
        <>
          {loading && (
            <div style={{
              position: 'absolute',
              top: '50%',
              left: '50%',
              transform: 'translate(-50%, -50%)',
              zIndex: 10,
              textAlign: 'center',
            }}>
              <Spin size="large" />
              <div style={{ marginTop: 16, color: '#666' }}>{t('pod:terminal.monitoringData')}</div>
            </div>
          )}
          <iframe
            key={`${refreshKey}-${clusterId}-${namespace}-${podName}`}
            src={dashboardUrl}
            width="100%"
            height="800"
            frameBorder="0"
            style={{ border: 'none', display: 'block' }}
            title="Grafana Pod Monitoring Dashboard"
            onLoad={handleIframeLoad}
          />
        </>
      )}
    </Card>
  );
};

export default PodMonitoringTab;
