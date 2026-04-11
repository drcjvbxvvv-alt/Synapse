import React from 'react';
import { Space, Switch, Select, Button } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface OverviewToolbarProps {
  loading: boolean;
  lastRefreshTime: Date;
  autoRefresh: boolean;
  refreshInterval: number;
  onAutoRefreshChange: (val: boolean) => void;
  onRefreshIntervalChange: (val: number) => void;
  onRefresh: () => void;
}

export const OverviewToolbar: React.FC<OverviewToolbarProps> = ({
  loading,
  lastRefreshTime,
  autoRefresh,
  refreshInterval,
  onAutoRefreshChange,
  onRefreshIntervalChange,
  onRefresh,
}) => {
  const { t } = useTranslation(['overview', 'common']);

  return (
    <div style={{
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      marginBottom: 16,
      padding: '12px 16px',
      background: '#fff',
      borderRadius: 8,
      boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
    }}>
      <div>
        <span style={{ fontSize: 18, fontWeight: 600, color: '#1f2937' }}>{t('title')}</span>
        <span style={{ marginLeft: 16, color: '#9ca3af', fontSize: 13 }}>
          {t('common:time.lastUpdate')}: {lastRefreshTime.toLocaleTimeString()}
        </span>
      </div>
      <Space>
        <span style={{ color: '#6b7280' }}>{t('autoRefresh')}:</span>
        <Switch
          checked={autoRefresh}
          onChange={onAutoRefreshChange}
          size="small"
        />
        {autoRefresh && (
          <Select
            value={refreshInterval}
            onChange={onRefreshIntervalChange}
            size="small"
            style={{ width: 90 }}
          >
            <Select.Option value={30}>{t('common:units.second30')}</Select.Option>
            <Select.Option value={60}>{t('common:units.minute1')}</Select.Option>
            <Select.Option value={300}>{t('common:units.minute5')}</Select.Option>
          </Select>
        )}
        <Button
          icon={<ReloadOutlined spin={loading} />}
          onClick={onRefresh}
          loading={loading}
          size="small"
        >
          {t('common:actions.refresh')}
        </Button>
      </Space>
    </div>
  );
};
