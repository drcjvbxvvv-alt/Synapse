import React, { useCallback } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Layout } from 'antd';
import AppHeader from './AppHeader';
import AppSider from './AppSider';
import ClusterContextBar from './ClusterContextBar';
import AIChatPanel from '../components/AIChat/AIChatPanel';

const { Content } = Layout;

const MainLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const isClusterDetail = !!location.pathname.match(/\/clusters\/[^/]+\//);

  const handleSearch = useCallback((value: string) => {
    if (value.trim()) {
      navigate(`/search?q=${encodeURIComponent(value)}`);
    }
  }, [navigate]);

  return (
    <Layout style={{ minHeight: '100vh', background: '#fafbfc' }}>
      <AppHeader onSearch={handleSearch} />
      {isClusterDetail && <ClusterContextBar />}

      <Layout style={{ marginTop: isClusterDetail ? 112 : 64 }}>
        <AppSider isClusterDetail={isClusterDetail} />

        <Layout style={{ marginLeft: 192 }}>
          <Content
            style={{
              margin: '0px 4px',
              padding: 16,
              minHeight: 'calc(100vh - 96px)',
              background: '#ffffff',
              borderRadius: 8,
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.06)',
            }}
          >
            <Outlet />
          </Content>
        </Layout>
      </Layout>

      <AIChatPanel />
    </Layout>
  );
};

export default MainLayout;
