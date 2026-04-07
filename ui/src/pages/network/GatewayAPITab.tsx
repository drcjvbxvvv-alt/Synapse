import React, { useState, useEffect } from 'react';
import { Tabs, Alert, Button, Space, Typography, App } from 'antd';
import { ReloadOutlined, CopyOutlined, LinkOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import GatewayClassList from './GatewayClassList';
import GatewayList from './GatewayList';
import HTTPRouteList from './HTTPRouteList';
import GRPCRouteList from './GRPCRouteList';
import ReferenceGrantList from './ReferenceGrantList';
import GatewayTopology from './GatewayTopology';
import type { GatewayTabProps } from './gatewayTypes';

const GATEWAY_API_DOCS = 'https://gateway-api.sigs.k8s.io/guides/';

const GatewayAPITab: React.FC<GatewayTabProps> = ({ clusterId }) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [available, setAvailable] = useState<boolean | null>(null);
  const [checking, setChecking] = useState(false);

  const checkAvailability = async () => {
    setChecking(true);
    try {
      const res = await gatewayService.getStatus(clusterId);
      setAvailable(res.available);
      if (res.available) {
        message.success(t('gatewayapi.messages.recheckSuccess'));
      }
    } catch {
      setAvailable(false);
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    checkAvailability();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusterId]);

  const installCmd = t('gatewayapi.installCmd');

  const handleCopyCmd = () => {
    navigator.clipboard.writeText(installCmd).then(() => {
      message.success(t('common:messages.copySuccess', 'Copied!'));
    });
  };

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
      <div style={{ maxWidth: 640, margin: '48px auto', padding: '0 24px' }}>
        <Alert
          type="warning"
          showIcon
          message={t('gatewayapi.notInstalled')}
          description={
            <Space direction="vertical" style={{ width: '100%', marginTop: 8 }}>
              <Typography.Text>{t('gatewayapi.notInstalledDesc')}</Typography.Text>
              <Typography.Text strong style={{ display: 'block', marginTop: 8 }}>
                安裝指令：
              </Typography.Text>
              <pre
                style={{
                  background: '#1e1e1e',
                  color: '#d4d4d4',
                  padding: '12px 16px',
                  borderRadius: 6,
                  fontSize: 13,
                  margin: 0,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {installCmd}
              </pre>
              <Space style={{ marginTop: 8 }}>
                <Button icon={<CopyOutlined />} onClick={handleCopyCmd}>
                  {t('gatewayapi.copyCmd')}
                </Button>
                <Button
                  icon={<LinkOutlined />}
                  href={GATEWAY_API_DOCS}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {t('gatewayapi.officialDocs')}
                </Button>
                <Button
                  icon={<ReloadOutlined />}
                  loading={checking}
                  onClick={checkAvailability}
                >
                  {t('gatewayapi.recheck')}
                </Button>
              </Space>
            </Space>
          }
        />
      </div>
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
      defaultActiveKey="gateways"
      items={tabItems}
      tabBarExtraContent={
        <Button
          size="small"
          icon={<ReloadOutlined />}
          loading={checking}
          onClick={checkAvailability}
        >
          {t('gatewayapi.recheck')}
        </Button>
      }
    />
  );
};

export default GatewayAPITab;
