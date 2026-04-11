import React from 'react';
import { Button, Space, Row, Col, Card, Statistic, Progress, Table } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartTooltip, ResponsiveContainer,
} from 'recharts';
import type { ClusterResourceSnapshot, NamespaceOccupancy } from '../../../services/costService';
import { BAR_PROPS, GRID_STYLE, TOOLTIP_STYLE } from '../constants';
import { getOccupancyColumns } from '../columns';

interface OccupancyTabProps {
  snapshot: ClusterResourceSnapshot | null;
  snapshotLoading: boolean;
  nsOccupancy: NamespaceOccupancy[];
  nsOccLoading: boolean;
  onRefresh: () => void;
}

export const OccupancyTab: React.FC<OccupancyTabProps> = ({
  snapshot,
  snapshotLoading,
  nsOccupancy,
  nsOccLoading,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  const barData = nsOccupancy.slice(0, 15).map(n => ({
    name: n.namespace,
    cpu: +n.cpu_occupancy_percent.toFixed(2),
  }));

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh}>
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
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={t('cost:occupancy.podCount')}
              value={snapshot?.pod_count ?? 0}
              loading={snapshotLoading}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} lg={14}>
          <Card title={t('cost:occupancy.nsBreakdown')} size="small">
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={barData} margin={{ top: 5, right: 20, left: 10, bottom: 60 }}>
                <CartesianGrid {...GRID_STYLE} />
                <XAxis dataKey="name" angle={-30} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                <YAxis unit="%" />
                <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${v}%`, t('cost:occupancy.cpuRateLabel')]} />
                <Bar dataKey="cpu" fill="#5B8FF9" {...BAR_PROPS} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
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
              style={{ marginTop: 16 }}
            />
          </Card>
        </Col>
      </Row>

      <Table
        rowKey="namespace"
        columns={getOccupancyColumns(t)}
        dataSource={nsOccupancy}
        loading={nsOccLoading}
        size="small"
        scroll={{ x: 700 }}
        pagination={{ pageSize: 20 }}
      />
    </div>
  );
};
