import React from 'react';
import {
  Card,
  Button,
  Spin,
  Space,
  Typography,
  theme,
  Flex,
  Divider,
  Tag,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import {
  SyncOutlined,
  CheckCircleFilled,
  CloseCircleFilled,
  QuestionCircleFilled,
  ApiOutlined,
  ClusterOutlined,
  AppstoreOutlined,
  DatabaseOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { ControlPlaneStatusResponse, ControlPlaneComponent } from '../../../services/omService';
import { formatBytes, formatTime } from './omUtils';

const { Text } = Typography;

interface ControlPlaneCardProps {
  controlPlaneStatus: ControlPlaneStatusResponse | null;
  controlPlaneLoading: boolean;
  onRefresh: () => void;
  t: TFunction;
}

const getComponentIcon = (type: string, color: string) => {
  const style = { fontSize: 18, color };
  switch (type) {
    case 'apiserver': return <ApiOutlined style={style} />;
    case 'scheduler': return <ClusterOutlined style={style} />;
    case 'controller-manager': return <AppstoreOutlined style={style} />;
    case 'etcd': return <DatabaseOutlined style={style} />;
    default: return <CloudServerOutlined style={style} />;
  }
};

const OverallBadge: React.FC<{ status: string; t: TFunction }> = ({ status, t }) => {
  const { token } = theme.useToken();
  if (status === 'healthy') {
    return (
      <Flex align="center" gap={6}>
        <CheckCircleFilled style={{ color: token.colorSuccess }} />
        <Text style={{ color: token.colorSuccess, fontWeight: 600 }}>
          {t('om:controlPlane.statusHealthy')}
        </Text>
      </Flex>
    );
  }
  if (status === 'unhealthy') {
    return (
      <Flex align="center" gap={6}>
        <CloseCircleFilled style={{ color: token.colorError }} />
        <Text style={{ color: token.colorError, fontWeight: 600 }}>
          {t('om:controlPlane.statusUnhealthy')}
        </Text>
      </Flex>
    );
  }
  return (
    <Flex align="center" gap={6}>
      <QuestionCircleFilled style={{ color: token.colorTextTertiary }} />
      <Text type="secondary">{t('om:controlPlane.statusUnknown')}</Text>
    </Flex>
  );
};

const ComponentRow: React.FC<{ component: ControlPlaneComponent; t: TFunction; isLast: boolean }> = ({
  component,
  t,
  isLast,
}) => {
  const { token } = theme.useToken();

  const statusColor =
    component.status === 'healthy'
      ? token.colorSuccess
      : component.status === 'unhealthy'
      ? token.colorError
      : token.colorTextTertiary;

  const StatusIcon =
    component.status === 'healthy'
      ? CheckCircleFilled
      : component.status === 'unhealthy'
      ? CloseCircleFilled
      : QuestionCircleFilled;

  return (
    <>
      <div style={{ padding: `${token.paddingSM}px 0` }}>
        <Flex align="flex-start" gap={token.marginMD}>
          {/* Icon */}
          <div
            style={{
              flexShrink: 0,
              width: 36,
              height: 36,
              borderRadius: token.borderRadius,
              background: token.colorFillAlter,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {getComponentIcon(component.type, statusColor)}
          </div>

          {/* Body */}
          <div style={{ flex: 1, minWidth: 0 }}>
            <Flex justify="space-between" align="center" style={{ marginBottom: 2 }}>
              <Text strong style={{ fontSize: token.fontSizeSM }}>
                {component.name}
              </Text>
              <Flex align="center" gap={4}>
                <StatusIcon style={{ fontSize: 13, color: statusColor }} />
                <Text style={{ fontSize: token.fontSizeSM, color: statusColor }}>
                  {component.status === 'healthy'
                    ? t('om:controlPlane.statusHealthy')
                    : component.status === 'unhealthy'
                    ? t('om:controlPlane.statusUnhealthy')
                    : t('om:controlPlane.statusUnknown')}
                </Text>
              </Flex>
            </Flex>

            {component.message && (
              <Text
                type="secondary"
                style={{ fontSize: token.fontSizeSM, display: 'block', marginBottom: 4 }}
              >
                {component.message}
              </Text>
            )}

            {/* Metrics */}
            {component.metrics && (
              <Flex gap={token.marginMD} wrap="wrap">
                {component.metrics.request_rate !== undefined && (
                  <Flex align="center" gap={4}>
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('om:controlPlane.requestRate')}:
                    </Text>
                    <Text style={{ fontSize: token.fontSizeSM }}>
                      {component.metrics.request_rate}/s
                    </Text>
                  </Flex>
                )}
                {component.metrics.error_rate !== undefined && (
                  <Flex align="center" gap={4}>
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('om:controlPlane.errorRate')}:
                    </Text>
                    <Text
                      style={{
                        fontSize: token.fontSizeSM,
                        color:
                          component.metrics.error_rate > 1
                            ? token.colorError
                            : token.colorSuccess,
                      }}
                    >
                      {component.metrics.error_rate}%
                    </Text>
                  </Flex>
                )}
                {component.metrics.leader_status !== undefined && (
                  <Flex align="center" gap={4}>
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      Leader:
                    </Text>
                    {component.metrics.leader_status ? (
                      <CheckCircleFilled
                        style={{ fontSize: token.fontSizeSM, color: token.colorSuccess }}
                      />
                    ) : (
                      <CloseCircleFilled
                        style={{ fontSize: token.fontSizeSM, color: token.colorError }}
                      />
                    )}
                  </Flex>
                )}
                {component.metrics.db_size !== undefined && (
                  <Flex align="center" gap={4}>
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('om:controlPlane.dbSize')}:
                    </Text>
                    <Text style={{ fontSize: token.fontSizeSM }}>
                      {formatBytes(component.metrics.db_size)}
                    </Text>
                  </Flex>
                )}
                {component.metrics.queue_length !== undefined && (
                  <Flex align="center" gap={4}>
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('om:controlPlane.queueLength')}:
                    </Text>
                    <Text style={{ fontSize: token.fontSizeSM }}>
                      {component.metrics.queue_length}
                    </Text>
                  </Flex>
                )}
              </Flex>
            )}

            {component.instances && component.instances.length > 0 && (
              <Tag style={{ marginTop: 4, fontSize: token.fontSizeSM - 1 }}>
                {component.instances.length} {t('om:controlPlane.instanceCount')}
              </Tag>
            )}
          </div>
        </Flex>
      </div>
      {!isLast && <Divider style={{ margin: 0 }} />}
    </>
  );
};

const ControlPlaneCard: React.FC<ControlPlaneCardProps> = ({
  controlPlaneStatus,
  controlPlaneLoading,
  onRefresh,
  t,
}) => {
  const { token } = theme.useToken();

  if (controlPlaneLoading) {
    return (
      <Card variant="borderless" title={t('om:controlPlane.title')}>
        <Flex justify="center" style={{ padding: token.paddingXL }}>
          <Spin size="large" />
        </Flex>
      </Card>
    );
  }

  if (!controlPlaneStatus) {
    return (
      <Card variant="borderless" title={t('om:controlPlane.title')}>
        <EmptyState description={t('common:messages.noData')} />
      </Card>
    );
  }

  return (
    <Card
      variant="borderless"
      title={
        <Flex align="center" gap={token.marginSM}>
          <CloudServerOutlined />
          <span>{t('om:controlPlane.title')}</span>
        </Flex>
      }
      extra={
        <Space>
          <OverallBadge status={controlPlaneStatus.overall} t={t} />
          <Button size="small" icon={<SyncOutlined />} onClick={onRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Space>
      }
    >
      <div>
        {controlPlaneStatus.components.map((component, index) => (
          <ComponentRow
            key={component.name}
            component={component}
            t={t}
            isLast={index === controlPlaneStatus.components.length - 1}
          />
        ))}
      </div>

      <div style={{ marginTop: token.marginMD, textAlign: 'right' }}>
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {t('om:controlPlane.checkTime')}: {formatTime(controlPlaneStatus.check_time)}
        </Text>
      </div>
    </Card>
  );
};

export default ControlPlaneCard;
