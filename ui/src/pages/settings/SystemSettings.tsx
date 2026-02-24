import React, { useState } from 'react';
import { Typography, Tabs, Breadcrumb } from 'antd';
import {
  SettingOutlined,
  CloudServerOutlined,
  SafetyCertificateOutlined,
  BellOutlined,
  KeyOutlined,
  DashboardOutlined,
} from '@ant-design/icons';
import { Link } from 'react-router-dom';
import LDAPSettings from './LDAPSettings';
import SSHSettings from './SSHSettings';
import GrafanaSettings from './GrafanaSettings';
import { useTranslation } from 'react-i18next';

const { Title } = Typography;

const SystemSettings: React.FC = () => {
const { t } = useTranslation(['settings', 'common']);
const [activeTab, setActiveTab] = useState('ssh');

  const tabItems = [
    {
      key: 'ssh',
      label: (
        <span>
          <KeyOutlined />
          {t('settings:tabs.ssh')}
        </span>
      ),
      children: <SSHSettings />,
    },
    {
      key: 'ldap',
      label: (
        <span>
          <CloudServerOutlined />
          {t('settings:tabs.ldap')}
        </span>
      ),
      children: <LDAPSettings />,
    },
    {
      key: 'grafana',
      label: (
        <span>
          <DashboardOutlined />
          {t('settings:tabs.grafana')}
        </span>
      ),
      children: <GrafanaSettings />,
    },
    {
      key: 'security',
      label: (
        <span>
          <SafetyCertificateOutlined />
          {t('settings:tabs.security')}
        </span>
      ),
      children: (
        <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>
          {t('settings:featureInDev', { feature: t('settings:tabs.security') })}
        </div>
      ),
    },
    {
      key: 'notification',
      label: (
        <span>
          <BellOutlined />
          {t('settings:tabs.notification')}
        </span>
      ),
      children: (
        <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>
          {t('settings:featureInDev', { feature: t('settings:tabs.notification') })}
        </div>
      ),
    },
  ];

  return (
    <div>
      <Breadcrumb
        items={[
          { title: <Link to="/">{t('settings:breadcrumb.home')}</Link> },
          { title: t('settings:title') },
        ]}
        style={{ marginBottom: 16 }}
      />

      <div style={{ marginBottom: 24 }}>
        <Title level={3} style={{ margin: 0 }}>
          <SettingOutlined style={{ marginRight: 8 }} />
          {t('settings:title')}
        </Title>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={tabItems}
        tabPosition="left"
        style={{ minHeight: 500 }}
      />
    </div>
  );
};

export default SystemSettings;
