import React from 'react';
import { App, Row, Col, Button, Typography, theme, Flex } from 'antd';
import { SyncOutlined, RadarChartOutlined } from '@ant-design/icons';
import { useMonitoringCenter } from './hooks/useMonitoringCenter';
import HealthScoreCard from './components/HealthScoreCard';
import ResourceTopCard from './components/ResourceTopCard';
import ControlPlaneCard from './components/ControlPlaneCard';

const { Title, Text } = Typography;

const MonitoringCenterInner: React.FC = () => {
  const { token } = theme.useToken();
  const state = useMonitoringCenter();

  return (
    <div
      style={{
        padding: token.paddingLG,
        background: token.colorBgLayout,
        minHeight: '100vh',
      }}
    >
      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginLG }}>
        <div>
          <Flex align="center" gap={token.marginSM} style={{ marginBottom: token.marginXS }}>
            <RadarChartOutlined style={{ fontSize: token.fontSizeXL, color: token.colorPrimary }} />
            <Title level={4} style={{ margin: 0 }}>
              {state.t('om:title')}
            </Title>
          </Flex>
          <Text type="secondary">{state.t('om:subtitle')}</Text>
        </div>
        <Button icon={<SyncOutlined />} onClick={state.handleRefreshAll}>
          {state.t('om:refreshAll')}
        </Button>
      </Flex>

      <Row gutter={[token.marginLG, token.marginLG]}>
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
