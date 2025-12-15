import React, { useState } from 'react';
import { Typography, Tabs, Breadcrumb } from 'antd';
import {
  SettingOutlined,
  CloudServerOutlined,
  SafetyCertificateOutlined,
  BellOutlined,
} from '@ant-design/icons';
import { Link } from 'react-router-dom';
import LDAPSettings from './LDAPSettings';

const { Title } = Typography;

const SystemSettings: React.FC = () => {
  const [activeTab, setActiveTab] = useState('ldap');

  const tabItems = [
    {
      key: 'ldap',
      label: (
        <span>
          <CloudServerOutlined />
          LDAP 设置
        </span>
      ),
      children: <LDAPSettings />,
    },
    {
      key: 'security',
      label: (
        <span>
          <SafetyCertificateOutlined />
          安全设置
        </span>
      ),
      children: (
        <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>
          安全设置功能开发中...
        </div>
      ),
    },
    {
      key: 'notification',
      label: (
        <span>
          <BellOutlined />
          通知设置
        </span>
      ),
      children: (
        <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>
          通知设置功能开发中...
        </div>
      ),
    },
  ];

  return (
    <div>
      <Breadcrumb
        items={[
          { title: <Link to="/">首页</Link> },
          { title: '系统设置' },
        ]}
        style={{ marginBottom: 16 }}
      />

      <div style={{ marginBottom: 24 }}>
        <Title level={3} style={{ margin: 0 }}>
          <SettingOutlined style={{ marginRight: 8 }} />
          系统设置
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
