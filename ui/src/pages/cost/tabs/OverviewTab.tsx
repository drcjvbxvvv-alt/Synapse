import React from 'react';
import { Button, Space, Row, Col, Card, Statistic, Progress } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ClusterResourceSnapshot } from '../../../services/costService';

interface OverviewTabProps {
  snapshot: ClusterResourceSnapshot | null;
  snapshotLoading: boolean;
  onRefresh: () => void;
}

export const OverviewTab: React.FC<OverviewTabProps> = ({
  snapshot,
  snapshotLoading,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={snapshotLoading}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:occupancy.cpuOccupancy')}
              value={snapshot?.occupancy.cpu ?? 0}
              precision={1}
              suffix="%"
              loading={snapshotLoading}
              valueStyle={{ color: (snapshot?.occupancy.cpu ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
            />
            <Progress
              percent={+(snapshot?.occupancy.cpu ?? 0).toFixed(1)}
              showInfo={false}
              status={(snapshot?.occupancy.cpu ?? 0) > 80 ? 'exception' : 'normal'}
              style={{ marginTop: 8 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:occupancy.memOccupancy')}
              value={snapshot?.occupancy.memory ?? 0}
              precision={1}
              suffix="%"
              loading={snapshotLoading}
              valueStyle={{ color: (snapshot?.occupancy.memory ?? 0) > 80 ? '#cf1322' : '#3f8600' }}
            />
            <Progress
              percent={+(snapshot?.occupancy.memory ?? 0).toFixed(1)}
              showInfo={false}
              status={(snapshot?.occupancy.memory ?? 0) > 80 ? 'exception' : 'normal'}
              style={{ marginTop: 8 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:occupancy.nodeCount')}
              value={snapshot?.node_count ?? 0}
              loading={snapshotLoading}
            />
            <Statistic
              title={t('cost:occupancy.podCount')}
              value={snapshot?.pod_count ?? 0}
              loading={snapshotLoading}
              style={{ marginTop: 12 }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card title={t('cost:occupancy.headroom')} size="small">
            <Statistic
              title={t('cost:occupancy.cpuHeadroom')}
              value={+(snapshot?.headroom.cpu_millicores ?? 0).toFixed(0)}
              loading={snapshotLoading}
            />
            <Statistic
              title={t('cost:occupancy.memHeadroom')}
              value={+(snapshot?.headroom.memory_mib ?? 0).toFixed(0)}
              loading={snapshotLoading}
              style={{ marginTop: 12 }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};
