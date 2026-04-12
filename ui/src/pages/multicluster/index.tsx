import React, { useState } from 'react';
import { Button, Tabs, Typography } from 'antd';
import { SwapOutlined, SyncOutlined, ApartmentOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import MigrationWizard from './MigrationWizard';
import SyncPolicyList from './SyncPolicyList';
import MultiClusterTopologyPage from './MultiClusterTopologyPage';

const { Title } = Typography;

const MultiClusterPage: React.FC = () => {
  const { t } = useTranslation(['multicluster', 'common']);
  const [wizardOpen, setWizardOpen] = useState(false);

  const items = [
    {
      key: 'topology',
      label: <span><ApartmentOutlined /> {t('multicluster:tabs.topology')}</span>,
      children: <MultiClusterTopologyPage />,
    },
    {
      key: 'sync',
      label: <span><SyncOutlined /> {t('multicluster:tabs.syncPolicy')}</span>,
      children: <SyncPolicyList />,
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>{t('multicluster:pageTitle')}</Title>
        <Button
          type="primary"
          icon={<SwapOutlined />}
          onClick={() => setWizardOpen(true)}
        >
          {t('multicluster:migrationWizard.buttonLabel')}
        </Button>
      </div>

      <Tabs items={items} />

      <MigrationWizard
        open={wizardOpen}
        onClose={() => setWizardOpen(false)}
        onMigrated={() => setWizardOpen(false)}
      />
    </div>
  );
};

export default MultiClusterPage;
