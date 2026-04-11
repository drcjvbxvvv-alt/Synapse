import React from 'react';
import { App, Row, Col, Button, Typography } from 'antd';
import { SyncOutlined, DashboardOutlined } from '@ant-design/icons';
import { useMonitoringCenter } from './hooks/useMonitoringCenter';
import HealthScoreCard from './components/HealthScoreCard';
import ResourceTopCard from './components/ResourceTopCard';
import ControlPlaneCard from './components/ControlPlaneCard';

const { Title, Text } = Typography;

const MonitoringCenterInner: React.FC = () => {
  const state = useMonitoringCenter();

  return (
    <div style={{ padding: 24, background: '#f0f2f5', minHeight: '100vh' }}>
      {/* Page header */}
      <div style={{ marginBottom: 24 }}>
        <Row justify="space-between" align="middle">
          <Col>
            <Title level={3} style={{ margin: 0 }}>
              <DashboardOutlined style={{ marginRight: 12 }} />
              {state.t('om:title')}
            </Title>
            <Text type="secondary">{state.t('om:subtitle')}</Text>
          </Col>
          <Col>
            <Button type="primary" icon={<SyncOutlined />} onClick={state.handleRefreshAll}>
              {state.t('om:refreshAll')}
            </Button>
          </Col>
        </Row>
      </div>

      {/* Main content */}
      <Row gutter={[24, 24]}>
        <Col span={24}>
          <HealthScoreCard
            healthDiagnosis={state.healthDiagnosis}
            healthLoading={state.healthLoading}
            onRefresh={state.loadHealthDiagnosis}
            t={state.t}
          />
        </Col>

        <Col xs={24} lg={12}>
          <ResourceTopCard
            resourceTop={state.resourceTop}
            resourceLoading={state.resourceLoading}
            resourceType={state.resourceType}
            setResourceType={state.setResourceType}
            resourceLevel={state.resourceLevel}
            setResourceLevel={state.setResourceLevel}
            onRefresh={state.loadResourceTop}
            t={state.t}
          />
        </Col>

        <Col xs={24} lg={12}>
          <ControlPlaneCard
            controlPlaneStatus={state.controlPlaneStatus}
            controlPlaneLoading={state.controlPlaneLoading}
            onRefresh={state.loadControlPlaneStatus}
            t={state.t}
          />
        </Col>
      </Row>
    </div>
  );
};

const MonitoringCenter: React.FC = () => (
  <App>
    <MonitoringCenterInner />
  </App>
);

export default MonitoringCenter;
