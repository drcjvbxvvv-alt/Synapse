import React from 'react';
import { Row, Col, Card, Statistic, Badge } from 'antd';
import {
  ClusterOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  DesktopOutlined,
  CloudServerOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ClusterStatsData, NodeStatsData, PodStatsData, GlobalAlertStats } from '../../../services/overviewService';

interface StatCardsProps {
  clusterStats: ClusterStatsData;
  nodeStats: NodeStatsData;
  podStats: PodStatsData;
  alertStats: GlobalAlertStats | null;
  formatNumber: (num: number, unit?: string) => string;
  onNavigate: (path: string) => void;
}

const cardStyle = { boxShadow: '0 1px 4px rgba(0,0,0,0.08)', borderRadius: 8 };

export const StatCards: React.FC<StatCardsProps> = ({
  clusterStats,
  nodeStats,
  podStats,
  alertStats,
  formatNumber,
  onNavigate,
}) => {
  const { t } = useTranslation('overview');

  return (
    <Row gutter={16} style={{ marginBottom: 16 }}>
      <Col span={4}>
        <Card bordered={false} style={{ ...cardStyle, height: 140 }} styles={{ body: { padding: '20px 16px' } }}>
          <Statistic
            title={<span style={{ color: '#6b7280' }}><ClusterOutlined /> {t('stats.clusterTotal')}</span>}
            value={clusterStats.total}
            valueStyle={{ color: '#1f2937', fontSize: 32, fontWeight: 700 }}
          />
        </Card>
      </Col>
      <Col span={4}>
        <Card
          bordered={false}
          style={{ ...cardStyle, height: 140, cursor: 'pointer' }}
          styles={{ body: { padding: '20px 16px' } }}
          onClick={() => onNavigate('/clusters')}
        >
          <Statistic
            title={<span style={{ color: '#6b7280' }}><CheckCircleOutlined style={{ color: '#10b981' }} /> {t('stats.clusterHealthy')}</span>}
            value={clusterStats.healthy}
            valueStyle={{ color: '#10b981', fontSize: 32, fontWeight: 700 }}
            suffix={<span style={{ fontSize: 14, color: '#9ca3af' }}>/ {clusterStats.total}</span>}
          />
        </Card>
      </Col>
      <Col span={4}>
        <Card
          bordered={false}
          style={{ ...cardStyle, height: 140, cursor: clusterStats.unhealthy > 0 ? 'pointer' : 'default' }}
          styles={{ body: { padding: '20px 16px' } }}
          onClick={() => clusterStats.unhealthy > 0 && onNavigate('/clusters')}
        >
          <Badge dot={clusterStats.unhealthy > 0} offset={[8, 0]}>
            <Statistic
              title={<span style={{ color: '#6b7280' }}><ExclamationCircleOutlined style={{ color: '#ef4444' }} /> {t('stats.clusterUnhealthy')}</span>}
              value={clusterStats.unhealthy}
              valueStyle={{ color: clusterStats.unhealthy > 0 ? '#ef4444' : '#9ca3af', fontSize: 32, fontWeight: 700 }}
            />
          </Badge>
        </Card>
      </Col>
      <Col span={4}>
        <Card bordered={false} style={{ ...cardStyle, height: 140 }} styles={{ body: { padding: '20px 16px' } }}>
          <Statistic
            title={<span style={{ color: '#6b7280' }}><DesktopOutlined /> {t('stats.nodeStatus')}</span>}
            value={nodeStats.ready}
            valueStyle={{ color: '#1f2937', fontSize: 32, fontWeight: 700 }}
            suffix={<span style={{ fontSize: 14, color: '#9ca3af' }}>/ {nodeStats.total}</span>}
          />
          {nodeStats.notReady > 0 && (
            <div style={{ marginTop: 4, color: '#ef4444', fontSize: 12 }}>
              <WarningOutlined /> {t('stats.nodeAbnormal', { count: nodeStats.notReady })}
            </div>
          )}
        </Card>
      </Col>
      <Col span={4}>
        <Card bordered={false} style={{ ...cardStyle, height: 140 }} styles={{ body: { padding: '20px 16px' } }}>
          <Statistic
            title={<span style={{ color: '#6b7280' }}><CloudServerOutlined /> {t('stats.podRunning')}</span>}
            value={podStats.running}
            valueStyle={{ color: '#1f2937', fontSize: 32, fontWeight: 700 }}
            suffix={<span style={{ fontSize: 14, color: '#9ca3af' }}>/ {formatNumber(podStats.total)}</span>}
          />
          {(podStats.pending > 0 || podStats.failed > 0) && (
            <div style={{ marginTop: 4, fontSize: 12 }}>
              {podStats.pending > 0 && <span style={{ color: '#f59e0b', marginRight: 8 }}>Pending: {podStats.pending}</span>}
              {podStats.failed > 0 && <span style={{ color: '#ef4444' }}>Failed: {podStats.failed}</span>}
            </div>
          )}
        </Card>
      </Col>
      <Col span={4}>
        <Card
          bordered={false}
          style={{ ...cardStyle, height: 140, cursor: (alertStats?.firing || 0) > 0 ? 'pointer' : 'default' }}
          styles={{ body: { padding: '20px 16px' } }}
          onClick={() => (alertStats?.firing || 0) > 0 && onNavigate('/alerts')}
        >
          <Statistic
            title={<span style={{ color: '#6b7280' }}><WarningOutlined style={{ color: '#f59e0b' }} /> {t('stats.alerts')}</span>}
            value={alertStats?.firing || 0}
            valueStyle={{ color: (alertStats?.firing || 0) > 0 ? '#f59e0b' : '#9ca3af', fontSize: 32, fontWeight: 700 }}
            suffix={<span style={{ fontSize: 14, color: '#9ca3af' }}>{t('stats.alertFiring')}</span>}
          />
          {alertStats && alertStats.enabledCount > 0 && (
            <div style={{ marginTop: 4, fontSize: 12, color: '#9ca3af' }}>
              {t('stats.alertConfigured', { count: alertStats.enabledCount })}
            </div>
          )}
        </Card>
      </Col>
    </Row>
  );
};
