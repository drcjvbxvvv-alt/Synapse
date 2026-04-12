import { Tag, Space, Tooltip, Typography, Popover, Button } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import type { TFunction } from 'i18next';
import type { EventLogEntry, LogEntry } from '../../services/logService';
import { levelTagColors } from './constants';
import { CopyOutlined } from '@ant-design/icons';

const { Text } = Typography;

// 長文本懸停卡片
const TextPopover: React.FC<{ content: string; t: TFunction }> = ({ content, t }) => (
  <Popover
    content={
      <div style={{ maxWidth: 500, wordBreak: 'break-all', maxHeight: 300, overflow: 'auto' }}>
        <div style={{ marginBottom: 8 }}>{content}</div>
        <Button
          type="primary"
          size="small"
          icon={<CopyOutlined />}
          onClick={() => navigator.clipboard.writeText(content)}
        >
          {t('common:actions.copy')}
        </Button>
      </div>
    }
    title={t('logs:center.preview')}
  >
    <Text ellipsis style={{ cursor: 'pointer', color: '#1890ff' }}>
      {content}
    </Text>
  </Popover>
);

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
    render: (reason: string) => <TextPopover content={reason} t={t} />,
  },
  {
    title: t('logs:center.resource'),
    key: 'resource',
    width: 200,
    render: (_, record) => (
      <div style={{ display: 'flex', gap: 6, alignItems: 'center', minWidth: 0 }}>
        <Tag color="cyan" style={{ marginRight: 0, flexShrink: 0 }}>{record.involved_kind}</Tag>
        <div style={{ flex: 1, minWidth: 0 }}>
          <TextPopover content={record.involved_name} t={t} />
        </div>
      </div>
    ),
  },
  {
    title: t('logs:center.message'),
    dataIndex: 'message',
    render: (message: string) => <TextPopover content={message} t={t} />,
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
    render: (_, record) => {
      const sourceStr = `${record.namespace}/${record.pod_name}:${record.container}`;
      return (
        <Space>
          <Tag color="cyan">{record.namespace}</Tag>
          <TextPopover content={sourceStr} t={t} />
        </Space>
      );
    },
  },
  {
    title: t('logs:center.logContent'),
    dataIndex: 'message',
    render: (message: string) => (
      <Text style={{ fontFamily: 'monospace' }}>
        <TextPopover content={message} t={t} />
      </Text>
    ),
  },
];

export const getExternalLogColumns = (t: TFunction): ColumnsType<LogEntry> => [
  {
    title: t('logs:center.time'),
    dataIndex: 'timestamp',
    width: 180,
    render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm:ss.SSS'),
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
    render: (message: string) => <TextPopover content={message} t={t} />,
  },
];
