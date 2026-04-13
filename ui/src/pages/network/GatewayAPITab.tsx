import React, { useState, useEffect } from 'react';
import { Tabs, Button, App } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import NotInstalledCard from '@/components/NotInstalledCard';
import GatewayClassList from './GatewayClassList';
import GatewayList from './GatewayList';
import HTTPRouteList from './HTTPRouteList';
import GRPCRouteList from './GRPCRouteList';
import ReferenceGrantList from './ReferenceGrantList';
import GatewayTopology from './GatewayTopology';
import type { GatewayTabProps } from './gatewayTypes';

const GatewayAPITab: React.FC<GatewayTabProps> = ({ clusterId }) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [available, setAvailable] = useState<boolean | null>(null);
  const [checking, setChecking] = useState(false);

  const checkAvailability = async (showToast = false) => {
    setChecking(true);
    try {
      const res = await gatewayService.getStatus(clusterId);
      setAvailable(res.available);
      if (showToast && res.available) {
        message.success(t('gatewayapi.messages.recheckSuccess'));
      }
    } catch {
      setAvailable(false);
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    checkAvailability(false);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusterId]);

  // 初始載入中
  if (available === null) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: '#8c8c8c' }}>
        {t('gatewayapi.messages.checkingAvailability')}
      </div>
    );
  }

  // Gateway API 未安裝
  if (!available) {
    return (
      <NotInstalledCard
        title={t('gatewayapi.notInstalled')}
        description={t('gatewayapi.notInstalledDesc')}
        command={t('gatewayapi.installCmd')}
        docsUrl="https://gateway-api.sigs.k8s.io/guides/"
        onRecheck={() => {
          setAvailable(null);
          checkAvailability(true);
        }}
        recheckLoading={checking}
      />
    );
  }

  // Gateway API 已安裝 - 顯示子 Tabs
  const tabItems = [
    {
      key: 'gatewayclass',
      label: t('gatewayapi.tabs.gatewayclass'),
      children: <GatewayClassList clusterId={clusterId} />,
    },
    {
      key: 'gateways',
      label: t('gatewayapi.tabs.gateways'),
      children: <GatewayList clusterId={clusterId} />,
    },
    {
      key: 'httproutes',
      label: t('gatewayapi.tabs.httproutes'),
      children: <HTTPRouteList clusterId={clusterId} />,
    },
    {
      key: 'grpcroutes',
      label: t('gatewayapi.tabs.grpcroutes'),
      children: <GRPCRouteList clusterId={clusterId} />,
    },
    {
      key: 'referencegrants',
      label: t('gatewayapi.tabs.referencegrants'),
      children: <ReferenceGrantList clusterId={clusterId} />,
    },
    {
      key: 'topology',
      label: t('gatewayapi.tabs.topology'),
      children: <GatewayTopology clusterId={clusterId} />,
    },
  ];

  return (
    <Tabs
      defaultActiveKey="gatewayclass"
      items={tabItems}
      tabBarExtraContent={
        <Button
          size="small"
          icon={<ReloadOutlined />}
          loading={checking}
          onClick={() => checkAvailability(true)}
        >
          {t('gatewayapi.recheck')}
        </Button>
      }
    />
  );
};

export default GatewayAPITab;
