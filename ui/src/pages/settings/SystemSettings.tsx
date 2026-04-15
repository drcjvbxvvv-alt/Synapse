import React, { useState } from 'react';
import { Typography, Tabs, Breadcrumb, Space } from 'antd';
import {
  SettingOutlined,
  CloudServerOutlined,
  SafetyCertificateOutlined,
  BellOutlined,
  KeyOutlined,
  DashboardOutlined,
  RobotOutlined,
  BranchesOutlined,
  InboxOutlined,
} from '@ant-design/icons';
import { Link } from 'react-router-dom';
import LDAPSettings from './LDAPSettings';
import SSHSettings from './SSHSettings';
import GrafanaSettings from './GrafanaSettings';
import AISettings from './AISettings';
import SecuritySettings from './SecuritySettings';
import NotificationSettings from './NotificationSettings';
import GitProviderSettings from './GitProviderSettings';
import RegistrySettings from './RegistrySettings';
import { useTranslation } from 'react-i18next';

const { Title } = Typography;

const SystemSettings: React.FC = () => {
const { t } = useTranslation(['settings', 'common', 'cicd']);
const [activeTab, setActiveTab] = useState('ssh');

  const tabLabel = (icon: React.ReactNode, text: string) => (
    <Space size={8}>
      {icon}
      <span>{text}</span>
    </Space>
  );

  const tabItems = [
    {
      key: 'ssh',
      label: tabLabel(<KeyOutlined />, t('settings:tabs.ssh')),
      children: <SSHSettings />,
    },
    {
      key: 'ldap',
      label: tabLabel(<CloudServerOutlined />, t('settings:tabs.ldap')),
      children: <LDAPSettings />,
    },
    {
      key: 'grafana',
      label: tabLabel(<DashboardOutlined />, t('settings:tabs.grafana')),
      children: <GrafanaSettings />,
    },
    {
      key: 'ai',
      label: tabLabel(<RobotOutlined />, t('settings:tabs.ai')),
      children: <AISettings />,
    },
    {
      key: 'security',
      label: tabLabel(<SafetyCertificateOutlined />, t('settings:tabs.security')),
      children: <SecuritySettings />,
    },
    {
      key: 'notification',
      label: tabLabel(<BellOutlined />, t('settings:tabs.notification')),
      children: <NotificationSettings />,
    },
    {
      key: 'git-providers',
      label: tabLabel(<BranchesOutlined />, t('cicd:gitProvider.title')),
      children: <GitProviderSettings />,
    },
    {
      key: 'registries',
      label: tabLabel(<InboxOutlined />, t('cicd:registry.title')),
      children: <RegistrySettings />,
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
