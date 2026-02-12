import React, { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import {
  Card,
  Tabs,
  Spin,
} from 'antd';
import PVCTab from './PVCTab';
import PVTab from './PVTab';
import StorageClassTab from './StorageClassTab';
import { useTranslation } from 'react-i18next';

const StorageList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
const { t } = useTranslation(['storage', 'common']);
const [searchParams, setSearchParams] = useSearchParams();
  const loading = false;
  
  // 从URL读取当前Tab
  const activeTab = searchParams.get('tab') || 'pvc';

  // 统计信息状态（保留用于回调，但不显示）
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_pvcCount, setPVCCount] = useState(0);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_pvCount, setPVCount] = useState(0);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_scCount, setSCCount] = useState(0);

  // Tab切换处理
  const handleTabChange = (key: string) => {
    setSearchParams({ tab: key });
  };

  // Tab项配置
  const tabItems = [
    {
      key: 'pvc',
      label: t('storage:tabs.pvc'),
      children: (
        <PVCTab
          clusterId={clusterId || ''}
          onCountChange={setPVCCount}
        />
      ),
    },
    {
      key: 'pv',
      label: t('storage:tabs.pv'),
      children: (
        <PVTab
          clusterId={clusterId || ''}
          onCountChange={setPVCount}
        />
      ),
    },
    {
      key: 'storageclass',
      label: t('storage:tabs.storageClass'),
      children: (
        <StorageClassTab
          clusterId={clusterId || ''}
          onCountChange={setSCCount}
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

export default StorageList;
