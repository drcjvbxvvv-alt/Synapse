import React from 'react';
import { Card, Tabs, Spin } from 'antd';
import { useParams, useSearchParams } from 'react-router-dom';
import ConfigMapList from './ConfigMapList';
import SecretList from './SecretList';
import { useTranslation } from 'react-i18next';

const ConfigSecretManagement: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
const { t } = useTranslation(['config', 'common']);
const [searchParams, setSearchParams] = useSearchParams();
  const loading = false;

  // 從URL讀取當前Tab
  const activeTab = searchParams.get('tab') || 'configmap';

  // Tab切換處理
  const handleTabChange = (key: string) => {
    setSearchParams({ tab: key });
  };

  // Tab項配置
  const tabItems = [
    {
      key: 'configmap',
      label: t('config:tabs.configmap'),
      children: (
        <ConfigMapList
          clusterId={clusterId || ''}
        />
      ),
    },
    {
      key: 'secret',
      label: t('config:tabs.secret'),
      children: (
        <SecretList
          clusterId={clusterId || ''}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Card variant="borderless">
        <Spin spinning={loading}>
          <Tabs
            activeKey={activeTab}
            onChange={handleTabChange}
            items={tabItems}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default ConfigSecretManagement;
