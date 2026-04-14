import React from 'react';
import {
  Card,
  Table,
  Button,
  Select,
  Space,
  Progress,
  Tooltip,
  Typography,
  theme,
  Flex,
} from 'antd';
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

const RankBadge: React.FC<{ rank: number }> = ({ rank }) => {
  const { token } = theme.useToken();

  if (rank === 1) {
    return (
      <div
        style={{
          width: 24,
          height: 24,
          borderRadius: '50%',
          background: '#FFD700',
          color: '#7a6000',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 12,
          fontWeight: 700,
          flexShrink: 0,
        }}
      >
        {rank}
      </div>
    );
  }
  if (rank === 2) {
    return (
      <div
        style={{
          width: 24,
          height: 24,
          borderRadius: '50%',
          background: '#C0C0C0',
          color: '#555',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 12,
          fontWeight: 700,
          flexShrink: 0,
        }}
      >
        {rank}
      </div>
    );
  }
  if (rank === 3) {
    return (
      <div
        style={{
          width: 24,
          height: 24,
          borderRadius: '50%',
          background: '#CD7F32',
          color: '#fff',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 12,
          fontWeight: 700,
          flexShrink: 0,
        }}
      >
        {rank}
      </div>
    );
  }
  return (
    <Text
      type="secondary"
      style={{
        width: 24,
        display: 'inline-block',
        textAlign: 'center',
        fontSize: token.fontSizeSM,
      }}
    >
      {rank}
    </Text>
  );
};

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
  const { token } = theme.useToken();

  const columns = [
    {
      title: t('om:resourceTop.rank'),
      dataIndex: 'rank',
      key: 'rank',
      width: 56,
      render: (rank: number) => <RankBadge rank={rank} />,
    },
    {
      title: t('common:table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string, record: ResourceTopItem) => (
        <Tooltip title={record.namespace ? `${record.namespace}/${name}` : name}>
          <span>
            {record.namespace && (
              <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {record.namespace}/
              </Text>
            )}
            <Text style={{ fontSize: token.fontSizeSM }}>{name}</Text>
          </span>
        </Tooltip>
      ),
    },
    {
      title: t('om:resourceTop.usage'),
      dataIndex: 'usage',
      key: 'usage',
      width: 100,
      render: (usage: number, record: ResourceTopItem) => {
        let formatted: string;
        if (record.unit === 'bytes' || record.unit === 'bytes/s') {
          formatted = formatBytes(usage);
        } else if (record.unit === 'cores') {
          formatted = formatCPU(usage);
        } else {
          formatted = `${usage.toFixed(2)} ${record.unit}`;
        }
        return <Text style={{ fontSize: token.fontSizeSM }}>{formatted}</Text>;
      },
    },
    {
      title: t('om:resourceTop.usageRate'),
      dataIndex: 'usage_rate',
      key: 'usage_rate',
      width: 130,
      render: (rate: number) => {
        const color =
          rate > 80
            ? token.colorError
            : rate > 60
            ? token.colorWarning
            : token.colorSuccess;
        return (
          <Flex align="center" gap={token.marginXS}>
            <Progress
              percent={Math.min(rate, 100)}
              showInfo={false}
              strokeColor={color}
              trailColor={token.colorFillSecondary}
              size={['100%', 4]}
              style={{ flex: 1 }}
            />
            <Text style={{ fontSize: token.fontSizeSM, color, flexShrink: 0, width: 40 }}>
              {rate.toFixed(1)}%
            </Text>
          </Flex>
        );
      },
    },
  ];

  return (
    <Card
      variant="borderless"
      title={
        <Flex align="center" gap={token.marginSM}>
          <BarChartOutlined />
          <span>{t('om:resourceTop.title')}</span>
        </Flex>
      }
      extra={
        <Space size={token.marginXS}>
          <Select
            value={resourceType}
            onChange={setResourceType}
            size="small"
            style={{ width: 90 }}
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
            size="small"
            style={{ width: 100 }}
            options={[
              { label: t('om:resourceTop.namespaceLevel'), value: 'namespace' },
              { label: t('om:resourceTop.workloadLevel'), value: 'workload' },
              { label: 'Pod', value: 'pod' },
            ]}
          />
          <Button
            size="small"
            icon={<SyncOutlined spin={resourceLoading} />}
            onClick={onRefresh}
          />
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
        <div style={{ marginTop: token.marginSM, textAlign: 'right' }}>
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {t('om:resourceTop.queryTime')}: {formatTime(resourceTop.query_time)}
          </Text>
        </div>
      )}
    </Card>
  );
};

export default ResourceTopCard;
