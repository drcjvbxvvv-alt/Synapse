import { Tag, Space, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import type { TFunction } from 'i18next';
import type { EventLogEntry, LogEntry } from '../../services/logService';
import { levelTagColors } from './constants';

const { Text } = Typography;

export const getEventColumns = (t: TFunction): ColumnsType<EventLogEntry> => [
  {
    title: t('logs:center.time'),
    dataIndex: 'last_timestamp',
    width: 170,
    render: (time: string) => (
      <Text type="secondary">
        {dayjs(time).format('YYYY-MM-DD HH:mm:ss')}
      </Text>
    ),
  },
  {
    title: t('common:table.type'),
    dataIndex: 'type',
    width: 80,
    render: (type: string) => (
      <Tag color={type === 'Warning' ? 'orange' : 'green'}>{type}</Tag>
    ),
  },
  {
    title: t('logs:center.reason'),
    dataIndex: 'reason',
    width: 120,
  },
  {
    title: t('logs:center.resource'),
    key: 'resource',
    width: 200,
    render: (_, record) => (
      <Space>
        <Tag color="cyan">{record.involved_kind}</Tag>
        <Text ellipsis style={{ maxWidth: 120 }}>
          {record.involved_name}
        </Text>
      </Space>
    ),
  },
  {
    title: t('logs:center.message'),
    dataIndex: 'message',
    ellipsis: true,
  },
  {
    title: t('logs:center.count'),
    dataIndex: 'count',
    width: 60,
    align: 'center',
  },
];

export const getSearchColumns = (t: TFunction): ColumnsType<LogEntry> => [
  {
    title: t('logs:center.time'),
    dataIndex: 'timestamp',
    width: 180,
    render: (time: string) => (
      <Text type="secondary">
        {dayjs(time).format('YYYY-MM-DD HH:mm:ss.SSS')}
      </Text>
    ),
  },
  {
    title: t('logs:center.level'),
    dataIndex: 'level',
    width: 80,
    render: (level: string) => (
      <Tag color={levelTagColors[level] || 'default'}>
        {level.toUpperCase()}
      </Tag>
    ),
  },
  {
    title: t('logs:center.source'),
    key: 'source',
    width: 250,
    render: (_, record) => (
      <Tooltip title={`${record.namespace}/${record.pod_name}:${record.container}`}>
        <Text ellipsis style={{ maxWidth: 230 }}>
          <Tag color="cyan">{record.namespace}</Tag>
          {record.pod_name}
        </Text>
      </Tooltip>
    ),
  },
  {
    title: t('logs:center.logContent'),
    dataIndex: 'message',
    render: (message: string) => (
      <Text
        style={{
          fontFamily: 'monospace',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {message}
      </Text>
    ),
  },
];

export const getExternalLogColumns = (t: TFunction): ColumnsType<LogEntry> => [
  {
    title: t('logs:center.time'),
    dataIndex: 'timestamp',
    width: 180,
    render: (v: string) => new Date(v).toLocaleString('zh-TW'),
  },
  {
    title: t('logs:center.level'),
    dataIndex: 'level',
    width: 80,
    render: (v: string) => (
      <Tag color={levelTagColors[v] || 'default'}>{v?.toUpperCase()}</Tag>
    ),
  },
  {
    title: t('common:table.namespace'),
    dataIndex: 'namespace',
    width: 130,
  },
  { title: 'Pod', dataIndex: 'pod_name', width: 150, ellipsis: true },
  {
    title: t('logs:center.message'),
    dataIndex: 'message',
    ellipsis: true,
  },
];
