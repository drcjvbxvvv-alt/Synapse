import React from 'react';
import { App, Row, Col, Button, Typography, Flex } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import { useMonitoringCenter } from './hooks/useMonitoringCenter';
import HealthScoreCard from './components/HealthScoreCard';
import ResourceTopCard from './components/ResourceTopCard';
import ControlPlaneCard from './components/ControlPlaneCard';

const { Text } = Typography;

const MonitoringCenterInner: React.FC = () => {
  const state = useMonitoringCenter();

  return (
    <div style={{ padding: '24px 28px', background: '#f7f8fa', minHeight: '100vh' }}>
      {/* header */}
      <Flex justify="space-between" align="center" style={{ marginBottom: 24 }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 600, color: '#1a1a1a', marginBottom: 2 }}>
            {state.t('om:title')}
          </div>
          <Text style={{ fontSize: 13, color: '#999' }}>{state.t('om:subtitle')}</Text>
        </div>
        <Button icon={<SyncOutlined />} onClick={state.handleRefreshAll}>
          {state.t('om:refreshAll')}
        </Button>
      </Flex>

      {/* health score — full width */}
      <div style={{ marginBottom: 20 }}>
        <HealthScoreCard
          healthDiagnosis={state.healthDiagnosis}
          healthLoading={state.healthLoading}
          onRefresh={state.loadHealthDiagnosis}
          t={state.t}
        />
      </div>

      {/* bottom two columns */}
      <Row gutter={20}>
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
