import React from 'react';
import { Button, Spin, Typography, theme, Flex } from 'antd';
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
import css from '../om.module.css';

const { Text } = Typography;

interface ControlPlaneCardProps {
  controlPlaneStatus: ControlPlaneStatusResponse | null;
  controlPlaneLoading: boolean;
  onRefresh: () => void;
  t: TFunction;
}

const COMPONENT_COLORS: Record<string, { bg: string; color: string }> = {
  apiserver:            { bg: 'rgba(139,92,246,0.1)',  color: '#7c3aed' },
  scheduler:            { bg: 'rgba(245,158,11,0.1)',  color: '#d97706' },
  'controller-manager': { bg: 'rgba(34,197,94,0.1)',   color: '#16a34a' },
  etcd:                 { bg: 'rgba(59,130,246,0.1)',  color: '#2563eb' },
};

const getComponentIcon = (type: string) => {
  switch (type) {
    case 'apiserver':            return <ApiOutlined />;
    case 'scheduler':            return <ClusterOutlined />;
    case 'controller-manager':   return <AppstoreOutlined />;
    case 'etcd':                 return <DatabaseOutlined />;
    default:                     return <CloudServerOutlined />;
  }
};

const STATUS_INDICATOR: React.FC<{ status: string; t: TFunction }> = ({ status, t }) => {
  if (status === 'healthy') {
    return (
      <Flex align="center" gap={6}>
        <span
          className={css.pulseDot}
          style={{ width: 8, height: 8, background: '#22c55e', color: '#22c55e', flexShrink: 0 }}
        />
        <Text style={{ fontSize: 12, fontWeight: 500, color: '#16a34a' }}>
          {t('om:controlPlane.statusHealthy')}
        </Text>
      </Flex>
    );
  }
  if (status === 'unhealthy') {
    return (
      <Flex align="center" gap={6}>
        <CloseCircleFilled style={{ color: '#ef4444', fontSize: 12 }} />
        <Text style={{ fontSize: 12, fontWeight: 500, color: '#ef4444' }}>
          {t('om:controlPlane.statusUnhealthy')}
        </Text>
      </Flex>
    );
  }
  return (
    <Flex align="center" gap={6}>
      <QuestionCircleFilled style={{ color: '#bbb', fontSize: 12 }} />
      <Text style={{ fontSize: 12, color: '#aaa' }}>
        {t('om:controlPlane.statusUnknown')}
      </Text>
    </Flex>
  );
};

const ComponentRow: React.FC<{ component: ControlPlaneComponent; t: TFunction }> = ({
  component, t,
}) => {
  const { token } = theme.useToken();
  const colorCfg = COMPONENT_COLORS[component.type] ?? { bg: 'rgba(0,0,0,0.06)', color: '#666' };

  return (
    <div
      className={css.componentRow}
      style={{ background: '#f7f8fa' }}
    >
      {/* icon box */}
      <div
        style={{
          width: 38, height: 38, borderRadius: 8, flexShrink: 0, marginRight: 14,
          background: colorCfg.bg, color: colorCfg.color,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: 16,
        }}
      >
        {getComponentIcon(component.type)}
      </div>

      {/* name + desc */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 13, fontWeight: 500, color: '#1a1a1a',
            marginBottom: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}
        >
          {component.name}
        </div>
        <Text style={{ fontSize: 11, color: '#bbb' }}>{component.message || t('om:controlPlane.statusHealthy')}</Text>

        {/* metrics inline */}
        {component.metrics && (
          <Flex gap={12} wrap="wrap" style={{ marginTop: 4 }}>
            {component.metrics.request_rate !== undefined && (
              <Text style={{ fontSize: 11, color: token.colorTextTertiary }}>
                {component.metrics.request_rate}/s
              </Text>
            )}
            {component.metrics.error_rate !== undefined && (
              <Text style={{
                fontSize: 11,
                color: component.metrics.error_rate > 1 ? '#ef4444' : '#22c55e',
              }}>
                err {component.metrics.error_rate}%
              </Text>
            )}
            {component.metrics.db_size !== undefined && (
              <Text style={{ fontSize: 11, color: token.colorTextTertiary }}>
                {formatBytes(component.metrics.db_size)}
              </Text>
            )}
          </Flex>
        )}
      </div>

      {/* right — instance badge + status */}
      <Flex align="center" gap={12} style={{ flexShrink: 0 }}>
        {component.instances && component.instances.length > 0 && (
          <span style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 11, padding: '3px 9px',
            borderRadius: 5, background: '#fff',
            border: '1px solid rgba(0,0,0,0.08)',
            color: '#888',
          }}>
            {component.instances.length} {t('om:controlPlane.instanceCount')}
          </span>
        )}
        <STATUS_INDICATOR status={component.status} t={t} />
      </Flex>
    </div>
  );
};

const ControlPlaneCard: React.FC<ControlPlaneCardProps> = ({
  controlPlaneStatus, controlPlaneLoading, onRefresh, t,
}) => {
  if (controlPlaneLoading) {
    return (
      <div style={cardStyle}>
        <Flex justify="center" style={{ padding: 48 }}>
          <Spin size="large" />
        </Flex>
      </div>
    );
  }

  if (!controlPlaneStatus) {
    return (
      <div style={cardStyle}>
        <EmptyState description={t('common:messages.noData')} />
      </div>
    );
  }

  const overall = controlPlaneStatus.overall;

  return (
    <div style={cardStyle} className={css.fadeUpDelay2}>
      {/* header */}
      <Flex justify="space-between" align="center" style={{ marginBottom: 20 }}>
        <Flex align="center" gap={8}>
          <CloudServerOutlined style={{ fontSize: 16, color: '#666' }} />
          <Text strong style={{ fontSize: 15 }}>{t('om:controlPlane.title')}</Text>
        </Flex>
        <Flex align="center" gap={14}>
          {overall === 'healthy' ? (
            <Flex align="center" gap={6}>
              <span
                className={css.pulseDot}
                style={{ width: 8, height: 8, background: '#22c55e', color: '#22c55e' }}
              />
              <CheckCircleFilled style={{ color: '#22c55e', fontSize: 13 }} />
              <Text style={{ fontSize: 13, fontWeight: 500, color: '#16a34a' }}>
                {t('om:controlPlane.statusHealthy')}
              </Text>
            </Flex>
          ) : (
            <Flex align="center" gap={6}>
              <CloseCircleFilled style={{ color: '#ef4444', fontSize: 13 }} />
              <Text style={{ fontSize: 13, fontWeight: 500, color: '#ef4444' }}>
                {t('om:controlPlane.statusUnhealthy')}
              </Text>
            </Flex>
          )}
          <Button size="small" icon={<SyncOutlined />} onClick={onRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Flex>
      </Flex>

      {/* component list */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {controlPlaneStatus.components.map((component) => (
          <ComponentRow key={component.name} component={component} t={t} />
        ))}
      </div>

      <div style={{ marginTop: 14, textAlign: 'right' }}>
        <Text style={{ fontSize: 11, color: '#bbb' }}>
          {t('om:controlPlane.checkTime')}: {formatTime(controlPlaneStatus.check_time)}
        </Text>
      </div>
    </div>
  );
};

const cardStyle: React.CSSProperties = {
  background: '#fff',
  borderRadius: 16,
  padding: 24,
  border: '1px solid rgba(0,0,0,0.06)',
  boxShadow: '0 4px 14px rgba(0,0,0,0.06)',
  height: '100%',
};

export default ControlPlaneCard;
