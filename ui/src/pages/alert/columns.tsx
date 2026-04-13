import React from 'react';
import {
  Tag,
  Badge,
  Button,
  Space,
  Tooltip,
  Typography,
  Popconfirm,
} from 'antd';
import {
  StopOutlined,
  SearchOutlined,
  DeleteOutlined,
  ClockCircleOutlined,
  FireOutlined,
  WarningOutlined,
  InfoCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { TFunction } from 'i18next';
import dayjs from 'dayjs';
import type { Alert, Silence } from '../../services/alertService';

const { Text } = Typography;

export function getSeverityColor(severity: string): string {
  switch (severity?.toLowerCase()) {
    case 'critical': return 'red';
    case 'warning':  return 'orange';
    case 'info':     return 'blue';
    default:         return 'default';
  }
}

export function getSeverityIcon(severity: string): React.ReactElement {
  switch (severity?.toLowerCase()) {
    case 'critical': return <FireOutlined />;
    case 'warning':  return <WarningOutlined />;
    case 'info':     return <InfoCircleOutlined />;
    default:         return <ExclamationCircleOutlined />;
  }
}

export function getStatusColor(state: string): string {
  switch (state) {
    case 'active':     return 'red';
    case 'suppressed': return 'orange';
    case 'resolved':   return 'green';
    default:           return 'default';
  }
}

interface AlertColumnsParams {
  t: TFunction;
  onSilence: (record: Alert) => void;
}

export function createAlertColumns({ t, onSilence }: AlertColumnsParams): ColumnsType<Alert> {
  return [
    {
      title: t('alert:center.alertName'),
      key: 'alertname',
      width: 200,
      render: (_, record) => (
        <Space>
          {getSeverityIcon(record.labels.severity)}
          <Text strong>{record.labels.alertname || t('alert:center.unknownAlert')}</Text>
        </Space>
      ),
    },
    {
      title: t('alert:center.severity'),
      key: 'severity',
      width: 100,
      render: (_, record) => (
        <Tag color={getSeverityColor(record.labels.severity)}>
          {record.labels.severity?.toUpperCase() || 'UNKNOWN'}
        </Tag>
      ),
    },
    {
      title: t('common:table.status'),
      key: 'status',
      width: 100,
      render: (_, record) => (
        <Badge
          status={record.status.state === 'active' ? 'error' : 'warning'}
          text={
            <Tag color={getStatusColor(record.status.state)}>
              {record.status.state === 'active'
                ? t('alert:center.statusFiring')
                : record.status.state === 'suppressed'
                ? t('alert:center.statusSuppressed')
                : t('alert:center.statusResolved')}
            </Tag>
          }
        />
      ),
    },
    {
      title: t('common:table.description'),
      key: 'description',
      ellipsis: true,
      render: (_, record) => (
        <Tooltip title={record.annotations?.description || record.annotations?.summary}>
          <Text ellipsis style={{ maxWidth: 300 }}>
            {record.annotations?.summary || record.annotations?.description || '-'}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('alert:center.triggerTime'),
      key: 'startsAt',
      width: 180,
      render: (_, record) => (
        <Tooltip title={dayjs(record.startsAt).format('YYYY-MM-DD HH:mm:ss')}>
          <Space>
            <ClockCircleOutlined />
            <Text>{dayjs(record.startsAt).fromNow()}</Text>
          </Space>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'action',
      width: 120,
      render: (_, record) => (
        <Space>
          <Tooltip title={t('alert:center.createSilence')}>
            <Button
              type="link"
              size="small"
              icon={<StopOutlined />}
              onClick={() => onSilence(record)}
              disabled={record.status.state === 'suppressed'}
            />
          </Tooltip>
          {record.generatorURL && (
            <Tooltip title={t('common:actions.detail')}>
              <Button
                type="link"
                size="small"
                icon={<SearchOutlined />}
                onClick={() => window.open(record.generatorURL, '_blank')}
              />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ];
}

interface SilenceColumnsParams {
  t: TFunction;
  onDelete: (silenceId: string) => void;
  canDelete: () => boolean;
}

export function createSilenceColumns({ t, onDelete, canDelete }: SilenceColumnsParams): ColumnsType<Silence> {
  return [
    {
      title: t('alert:center.matchRules'),
      key: 'matchers',
      render: (_, record) => (
        <Space direction="vertical" size="small">
          {record.matchers.map((matcher, index) => (
            <Tag key={index}>
              {matcher.name}
              {matcher.isEqual ? '=' : '!='}
              {matcher.isRegex ? '~' : ''}
              {matcher.value}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('common:table.status'),
      key: 'status',
      width: 100,
      render: (_, record) => (
        <Tag
          color={
            record.status.state === 'active'
              ? 'green'
              : record.status.state === 'pending'
              ? 'orange'
              : 'default'
          }
        >
          {record.status.state === 'active'
            ? t('alert:center.statusEffective')
            : record.status.state === 'pending'
            ? t('alert:center.statusPending')
            : t('alert:center.statusExpired')}
        </Tag>
      ),
    },
    {
      title: t('alert:center.effectiveTime'),
      key: 'startsAt',
      width: 180,
      render: (_, record) => dayjs(record.startsAt).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('alert:center.endTime'),
      key: 'endsAt',
      width: 180,
      render: (_, record) => dayjs(record.endsAt).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('alert:center.creator'),
      key: 'createdBy',
      width: 120,
      render: (_, record) => record.createdBy || '-',
    },
    {
      title: t('alert:center.remark'),
      key: 'comment',
      ellipsis: true,
      render: (_, record) => (
        <Tooltip title={record.comment}>
          <Text ellipsis style={{ maxWidth: 200 }}>
            {record.comment || '-'}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'action',
      width: 80,
      render: (_, record) => (
        canDelete() ? (
          <Popconfirm
            title={t('alert:center.deleteSilenceConfirm')}
            onConfirm={() => onDelete(record.id)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button type="link" danger size="small" icon={<DeleteOutlined />} />
          </Popconfirm>
        ) : null
      ),
    },
  ];
}
