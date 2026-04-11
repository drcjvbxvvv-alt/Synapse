import React from 'react';
import { Card, Table, Button, Select, Space, Badge, Progress, Tooltip, Typography } from 'antd';
import { SyncOutlined, BarChartOutlined } from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { ResourceTopResponse, ResourceTopItem } from '../../../services/omService';
import EmptyState from '@/components/EmptyState';
import { formatBytes, formatCPU, formatTime } from './omUtils';

const { Text } = Typography;

interface ResourceTopCardProps {
  resourceTop: ResourceTopResponse | null;
  resourceLoading: boolean;
  resourceType: 'cpu' | 'memory' | 'disk' | 'network';
  setResourceType: (type: 'cpu' | 'memory' | 'disk' | 'network') => void;
  resourceLevel: 'namespace' | 'workload' | 'pod';
  setResourceLevel: (level: 'namespace' | 'workload' | 'pod') => void;
  onRefresh: () => void;
  t: TFunction;
}

const ResourceTopCard: React.FC<ResourceTopCardProps> = ({
  resourceTop,
  resourceLoading,
  resourceType,
  setResourceType,
  resourceLevel,
  setResourceLevel,
  onRefresh,
  t,
}) => {
  const columns = [
    {
      title: t('om:resourceTop.rank'),
      dataIndex: 'rank',
      key: 'rank',
      width: 70,
      render: (rank: number) => (
        <Badge
          count={rank}
          style={{
            backgroundColor: rank <= 3
              ? (rank === 1 ? '#ff4d4f' : rank === 2 ? '#faad14' : '#52c41a')
              : '#d9d9d9',
          }}
        />
      ),
    },
    {
      title: t('common:table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string, record: ResourceTopItem) => (
        <Tooltip title={record.namespace ? `${record.namespace}/${name}` : name}>
          <span>
            {record.namespace && <Text type="secondary">{record.namespace}/</Text>}
            {name}
          </span>
        </Tooltip>
      ),
    },
    {
      title: t('om:resourceTop.usage'),
      dataIndex: 'usage',
      key: 'usage',
      render: (usage: number, record: ResourceTopItem) => {
        if (record.unit === 'bytes' || record.unit === 'bytes/s') return formatBytes(usage);
        if (record.unit === 'cores') return formatCPU(usage);
        return `${usage.toFixed(2)} ${record.unit}`;
      },
    },
    {
      title: t('om:resourceTop.usageRate'),
      dataIndex: 'usage_rate',
      key: 'usage_rate',
      width: 150,
      render: (rate: number) => (
        <Progress
          percent={Math.min(rate, 100)}
          size="small"
          strokeColor={rate > 80 ? '#ff4d4f' : rate > 60 ? '#faad14' : '#52c41a'}
          format={(percent) => `${(percent || 0).toFixed(1)}%`}
        />
      ),
    },
  ];

  return (
    <Card
      title={
        <Space>
          <BarChartOutlined />
          <span>{t('om:resourceTop.title')}</span>
        </Space>
      }
      extra={
        <Space>
          <Select
            value={resourceType}
            onChange={setResourceType}
            style={{ width: 100 }}
            options={[
              { label: 'CPU', value: 'cpu' },
              { label: t('om:resourceTop.memory'), value: 'memory' },
              { label: t('om:resourceTop.disk'), value: 'disk' },
              { label: t('om:resourceTop.network'), value: 'network' },
            ]}
          />
          <Select
            value={resourceLevel}
            onChange={setResourceLevel}
            style={{ width: 110 }}
            options={[
              { label: t('om:resourceTop.namespaceLevel'), value: 'namespace' },
              { label: t('om:resourceTop.workloadLevel'), value: 'workload' },
              { label: 'Pod', value: 'pod' },
            ]}
          />
          <Button icon={<SyncOutlined spin={resourceLoading} />} onClick={onRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Space>
      }
    >
      <Table
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={resourceTop?.items || []}
        loading={resourceLoading}
        rowKey="rank"
        pagination={false}
        size="small"
        locale={{ emptyText: <EmptyState /> }}
      />
      {resourceTop && (
        <div style={{ marginTop: 12, textAlign: 'right' }}>
          <Text type="secondary">{t('om:resourceTop.queryTime')}: {formatTime(resourceTop.query_time)}</Text>
        </div>
      )}
    </Card>
  );
};

export default ResourceTopCard;
