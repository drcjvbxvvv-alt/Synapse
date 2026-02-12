import React from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Card, Tabs } from 'antd';
import { BarChartOutlined, AlertOutlined, BranchesOutlined } from '@ant-design/icons';
import MonitoringConfigForm from '../../components/MonitoringConfigForm';
import AlertManagerConfigForm from '../../components/AlertManagerConfigForm';
import ArgoCDConfigForm from '../../components/ArgoCDConfigForm';
import type { TabsProps } from 'antd';
import { useTranslation } from 'react-i18next';
const ConfigCenter: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get('tab') || 'monitoring';
const { t } = useTranslation(['cluster']);
const handleTabChange = (key: string) => {
    setSearchParams({ tab: key });
  };

  const tabItems: TabsProps['items'] = [
    {
      key: 'monitoring',
      label: (
        <span>
          <BarChartOutlined />
          {t('configCenter.monitoringConfig')}
        </span>
      ),
      children: (
        <MonitoringConfigForm 
          clusterId={clusterId || ''} 
          onConfigChange={() => {
            // 配置更新后的回调
          }}
        />
      ),
    },
    {
      key: 'alertmanager',
      label: (
        <span>
          <AlertOutlined />
          {t('configCenter.alertConfig')}
        </span>
      ),
      children: (
        <AlertManagerConfigForm 
          clusterId={clusterId || ''} 
          onConfigChange={() => {
            // 配置更新后的回调
          }}
        />
      ),
    },
    {
      key: 'argocd',
      label: (
        <span>
          <BranchesOutlined />
          {t('configCenter.argocdConfig')}
        </span>
      ),
      children: (
        <ArgoCDConfigForm 
          clusterId={clusterId || ''} 
          onConfigChange={() => {
            // 配置更新后的回调
          }}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Card bordered={false}>
        <Tabs
          activeKey={activeTab}
          onChange={handleTabChange}
          items={tabItems}
        />
      </Card>
    </div>
  );
};

export default ConfigCenter;

