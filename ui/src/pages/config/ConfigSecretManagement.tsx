import React from 'react';
import { Card, Tabs, Spin } from 'antd';
import { useParams, useSearchParams } from 'react-router-dom';
import ConfigMapList from './ConfigMapList';
import SecretList from './SecretList';

const ConfigSecretManagement: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const loading = false;

  // 从URL读取当前Tab
  const activeTab = searchParams.get('tab') || 'configmap';

  // Tab切换处理
  const handleTabChange = (key: string) => {
    setSearchParams({ tab: key });
  };

  // Tab项配置
  const tabItems = [
    {
      key: 'configmap',
      label: '配置项（ConfigMap）',
      children: (
        <ConfigMapList
          clusterId={clusterId || ''}
        />
      ),
    },
    {
      key: 'secret',
      label: '密钥（Secret）',
      children: (
        <SecretList
          clusterId={clusterId || ''}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Card bordered={false}>
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
