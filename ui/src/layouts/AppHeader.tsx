import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Layout, Dropdown, Avatar, Space } from 'antd';
import {
  UserOutlined,
  GlobalOutlined,
  SettingOutlined,
  LogoutOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { MenuProps } from 'antd';
import synapseIcon from '../assets/synapse-icon.svg';
import NotificationPopover from '../components/NotificationPopover';
import { tokenManager } from '../services/authService';
import { supportedLanguages } from '../i18n';

const { Header } = Layout;

const AppHeader: React.FC = () => {
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const currentUser = tokenManager.getUser();

  const getDisplayName = () => {
    const name = currentUser?.display_name || currentUser?.username || 'User';
    return name.replace(/\d+$/, '');
  };

  const handleUserMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'logout') {
      tokenManager.clear();
      navigate('/login');
    } else if (key === 'profile') {
      navigate('/profile');
    } else if (key === 'settings') {
      navigate('/settings');
    } else if (supportedLanguages.some(l => l.code === key)) {
      i18n.changeLanguage(key);
    }
  };

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: t('common:menu.profile'),
    },
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: t('common:menu.settings'),
    },
    { type: 'divider' },
    {
      key: 'language',
      icon: <GlobalOutlined />,
      label: t('common:menu.language', '語言'),
      children: supportedLanguages.map(lang => ({
        key: lang.code,
        label: lang.name,
      })),
    },
    { type: 'divider' },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: t('auth.logout'),
      danger: true,
    },
  ];

  return (
    <Header
      style={{
        position: 'fixed',
        top: 0,
        zIndex: 1000,
        width: '100%',
        height: 48,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 16px',
        background: '#1f2937',
        borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
      }}
    >
      {/* Logo */}
      <div
        style={{ display: 'flex', alignItems: 'center', marginLeft: 16, cursor: 'pointer' }}
        onClick={() => navigate('/')}
      >
        <img src={synapseIcon} alt="Synapse" style={{ width: 32, height: 32, marginRight: 8 }} />
        <span style={{ fontSize: 18, fontWeight: 'bold', color: '#ffffff' }}>Synapse</span>
      </div>

      {/* 右側工具區 */}
      <Space size="middle">
        <NotificationPopover />

        <Dropdown
          menu={{
            items: userMenuItems,
            onClick: handleUserMenuClick,
            selectedKeys: [i18n.language],
          }}
          placement="bottomRight"
        >
          <Space style={{ cursor: 'pointer' }}>
            <Avatar icon={<UserOutlined />} style={{ backgroundColor: '#667eea' }} />
            <span style={{ color: '#ffffff' }}>{getDisplayName()}</span>
          </Space>
        </Dropdown>
      </Space>
    </Header>
  );
};

export default React.memo(AppHeader);
