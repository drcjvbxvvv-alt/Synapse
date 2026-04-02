import React, { useState } from 'react';
import { Button, Tabs, Typography } from 'antd';
import { SwapOutlined, SyncOutlined } from '@ant-design/icons';
import MigrationWizard from './MigrationWizard';
import SyncPolicyList from './SyncPolicyList';

const { Title } = Typography;

const MultiClusterPage: React.FC = () => {
  const [wizardOpen, setWizardOpen] = useState(false);

  const items = [
    {
      key: 'sync',
      label: <span><SyncOutlined /> 配置同步策略</span>,
      children: <SyncPolicyList />,
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>多叢集工作流程</Title>
        <Button
          type="primary"
          icon={<SwapOutlined />}
          onClick={() => setWizardOpen(true)}
        >
          工作負載遷移精靈
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
