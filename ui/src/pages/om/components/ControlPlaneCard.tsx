import React from 'react';
import {
  Card,
  Row,
  Col,
  Button,
  Spin,
  Space,
  Badge,
  Statistic,
  Typography,
  Empty,
} from 'antd';
import {
  SyncOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ApiOutlined,
  ClusterOutlined,
  AppstoreOutlined,
  DatabaseOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { ControlPlaneStatusResponse } from '../../../services/omService';
import { formatBytes, formatTime } from './omUtils';

const { Text } = Typography;

interface ControlPlaneCardProps {
  controlPlaneStatus: ControlPlaneStatusResponse | null;
  controlPlaneLoading: boolean;
  onRefresh: () => void;
  t: TFunction;
}

const getStatusBadge = (status: string, t: TFunction) => {
  switch (status) {
    case 'healthy': return <Badge status="success" text={t('om:controlPlane.statusHealthy')} />;
    case 'unhealthy': return <Badge status="error" text={t('om:controlPlane.statusUnhealthy')} />;
    case 'unknown': return <Badge status="default" text={t('om:controlPlane.statusUnknown')} />;
    default: return <Badge status="processing" text={status} />;
  }
};

const getComponentIcon = (type: string) => {
  switch (type) {
    case 'apiserver': return <ApiOutlined style={{ fontSize: 24, color: '#1890ff' }} />;
    case 'scheduler': return <ClusterOutlined style={{ fontSize: 24, color: '#722ed1' }} />;
    case 'controller-manager': return <AppstoreOutlined style={{ fontSize: 24, color: '#13c2c2' }} />;
    case 'etcd': return <DatabaseOutlined style={{ fontSize: 24, color: '#fa8c16' }} />;
    default: return <CloudServerOutlined style={{ fontSize: 24 }} />;
  }
};

const ControlPlaneCard: React.FC<ControlPlaneCardProps> = ({
  controlPlaneStatus,
  controlPlaneLoading,
  onRefresh,
  t,
}) => {
  if (controlPlaneLoading) {
    return (
      <Card title={t('om:controlPlane.title')} extra={<Button icon={<SyncOutlined spin />} disabled>{t('om:refreshing')}</Button>}>
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin size="large" />
        </div>
      </Card>
    );
  }

  if (!controlPlaneStatus) {
    return (
      <Card title={t('om:controlPlane.title')}>
        <Empty description={t('common:messages.noData')} />
      </Card>
    );
  }

  return (
    <Card
      title={
        <Space>
          <CloudServerOutlined />
          <span>{t('om:controlPlane.title')}</span>
        </Space>
      }
      extra={
        <Space>
          {getStatusBadge(controlPlaneStatus.overall, t)}
          <Button icon={<SyncOutlined />} onClick={onRefresh}>{t('common:actions.refresh')}</Button>
        </Space>
      }
    >
      <Row gutter={[16, 16]}>
        {controlPlaneStatus.components.map((component) => (
          <Col xs={24} sm={12} key={component.name}>
            <Card
              size="small"
              hoverable
              style={{
                borderLeft: `4px solid ${component.status === 'healthy' ? '#52c41a' : component.status === 'unhealthy' ? '#ff4d4f' : '#d9d9d9'}`,
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', marginBottom: 12 }}>
                {getComponentIcon(component.type)}
                <div style={{ marginLeft: 12 }}>
                  <Text strong>{component.name}</Text>
                  <br />
                  {getStatusBadge(component.status, t)}
                </div>
              </div>

              <Text type="secondary" style={{ fontSize: 12 }}>{component.message}</Text>

              {component.metrics && (
                <div style={{ marginTop: 12, borderTop: '1px solid #f0f0f0', paddingTop: 12 }}>
                  {component.metrics.request_rate !== undefined && (
                    <Statistic
                      title={t('om:controlPlane.requestRate')}
                      value={component.metrics.request_rate}
                      suffix="/s"
                      valueStyle={{ fontSize: 14 }}
                    />
                  )}
                  {component.metrics.error_rate !== undefined && (
                    <Statistic
                      title={t('om:controlPlane.errorRate')}
                      value={component.metrics.error_rate}
                      suffix="%"
                      valueStyle={{ fontSize: 14, color: component.metrics.error_rate > 1 ? '#ff4d4f' : '#52c41a' }}
                    />
                  )}
                  {component.metrics.leader_status !== undefined && (
                    <div>
                      <Text type="secondary">Leader: </Text>
                      {component.metrics.leader_status
                        ? <CheckCircleOutlined style={{ color: '#52c41a' }} />
                        : <CloseCircleOutlined style={{ color: '#ff4d4f' }} />
                      }
                    </div>
                  )}
                  {component.metrics.db_size !== undefined && (
                    <div>
                      <Text type="secondary">{t('om:controlPlane.dbSize')}: </Text>
                      <Text>{formatBytes(component.metrics.db_size)}</Text>
                    </div>
                  )}
                  {component.metrics.queue_length !== undefined && (
                    <div>
                      <Text type="secondary">{t('om:controlPlane.queueLength')}: </Text>
                      <Text>{component.metrics.queue_length}</Text>
                    </div>
                  )}
                </div>
              )}

              {component.instances && component.instances.length > 0 && (
                <div style={{ marginTop: 12, borderTop: '1px solid #f0f0f0', paddingTop: 12 }}>
                  <Text type="secondary">{t('om:controlPlane.instanceCount')}: {component.instances.length}</Text>
                </div>
              )}
            </Card>
          </Col>
        ))}
      </Row>

      <div style={{ marginTop: 12, textAlign: 'right' }}>
        <Text type="secondary">{t('om:controlPlane.checkTime')}: {formatTime(controlPlaneStatus.check_time)}</Text>
      </div>
    </Card>
  );
};

export default ControlPlaneCard;
