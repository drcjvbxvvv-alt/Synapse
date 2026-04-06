import React, { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import {
  Card,
  Tabs,
  Spin,
} from 'antd';
import ServiceTab from './ServiceTab';
import IngressTab from './IngressTab';
import NetworkPolicyTab from './NetworkPolicyTab';
import ServiceMeshTab from './ServiceMeshTab';
import { useTranslation } from 'react-i18next';
import { namespaceService } from '../../services/namespaceService';

const NetworkList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
const { t } = useTranslation(['network', 'common']);
const [searchParams, setSearchParams] = useSearchParams();
  const loading = false;
  const [namespaces, setNamespaces] = useState<string[]>([]);

  // 從URL讀取當前Tab
  const activeTab = searchParams.get('tab') || 'service';

  // 統計資訊狀態（保留用於回撥，但不顯示）
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_serviceCount, setServiceCount] = useState(0);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_ingressCount, setIngressCount] = useState(0);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_npCount, setNpCount] = useState(0);

  useEffect(() => {
    if (clusterId) {
      namespaceService.getNamespaces(clusterId)
        .then(res => setNamespaces((res as { items?: { name: string }[] }).items?.map(n => n.name) ?? []))
        .catch(() => {});
    }
  }, [clusterId]);

  // Tab切換處理
  const handleTabChange = (key: string) => {
    setSearchParams({ tab: key });
  };

  // Tab項配置
  const tabItems = [
    {
      key: 'service',
      label: t('network:tabs.service'),
      children: (
        <ServiceTab
          clusterId={clusterId || ''}
          onCountChange={setServiceCount}
        />
      ),
    },
    {
      key: 'ingress',
      label: t('network:tabs.ingress'),
      children: (
        <IngressTab
          clusterId={clusterId || ''}
          onCountChange={setIngressCount}
        />
      ),
    },
    {
      key: 'networkpolicy',
      label: t('network:tabs.networkpolicy'),
      children: (
        <NetworkPolicyTab
          clusterId={clusterId || ''}
          onCountChange={setNpCount}
        />
      ),
    },
    {
      key: 'service-mesh',
      label: 'Service Mesh',
      children: (
        <ServiceMeshTab
          clusterId={clusterId || ''}
          namespaces={namespaces}
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

export default NetworkList;
