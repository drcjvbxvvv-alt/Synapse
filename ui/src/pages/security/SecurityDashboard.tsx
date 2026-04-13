import React from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Tabs, Typography, Space } from 'antd';
import {
  SafetyOutlined,
  ScanOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { ImageScanTab } from './components/ImageScanTab';
import { BenchTab } from './components/BenchTab';
import { GatekeeperTab } from './components/GatekeeperTab';

const { Title } = Typography;

const SecurityDashboard: React.FC = () => {
  const { t } = useTranslation('security');
  const { id } = useParams<{ id: string }>();
  const clusterId = Number(id);

  const tabItems = [
    {
      key: 'imageScan',
      label: (
        <Space>
          <ScanOutlined />
          {t('tabs.imageScan')}
        </Space>
      ),
      children: <ImageScanTab clusterId={clusterId} />,
    },
    {
      key: 'bench',
      label: (
        <Space>
          <SafetyOutlined />
          {t('tabs.bench')}
        </Space>
      ),
      children: <BenchTab clusterId={clusterId} />,
    },
    {
      key: 'gatekeeper',
      label: (
        <Space>
          <ExclamationCircleOutlined />
          {t('tabs.gatekeeper')}
        </Space>
      ),
      children: <GatekeeperTab clusterId={clusterId} />,
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Title level={4} style={{ marginBottom: 24 }}>
        <SafetyOutlined style={{ marginRight: 8 }} />
        {t('title')}
      </Title>
      <Tabs items={tabItems} destroyInactiveTabPane />
    </div>
  );
};

export default SecurityDashboard;
