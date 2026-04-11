import React from 'react';
import { Button, Space, Tag, Tooltip } from 'antd';
import {
  EyeOutlined,
  EditOutlined,
  DeleteOutlined,
  HistoryOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import type { ColumnsType } from 'antd/es/table';
import type { TFunction } from 'i18next';
import type { ConfigMapListItem } from '../../services/configService';
import { ActionButtons } from '../../components/ActionButtons';

interface ConfigMapColumnParams {
  t: TFunction;
  clusterId: string;
  sortField: string;
  sortOrder: 'ascend' | 'descend' | null;
  colorTextTertiary: string;
  colorTextSecondary: string;
  navigate: (path: string) => void;
  handleDelete: (namespace: string, name: string) => void;
}

export function getConfigMapColumns(params: ConfigMapColumnParams): ColumnsType<ConfigMapListItem> {
  const { t, clusterId, sortField, sortOrder, colorTextTertiary, colorTextSecondary, navigate, handleDelete } = params;

  return [
    {
      title: t('common:table.name'),
      dataIndex: 'name',
      key: 'name',
      width: 250,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text: string, record: ConfigMapListItem) => (
        <Button
          type="link"
          onClick={() => navigate(`/clusters/${clusterId}/configs/configmap/${record.namespace}/${text}`)}
          style={{ padding: 0, height: 'auto', whiteSpace: 'normal', wordBreak: 'break-all', textAlign: 'left' }}
        >
          {text}
        </Button>
      ),
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('common:table.labels'),
      dataIndex: 'labels',
      key: 'labels',
      width: 250,
      render: (labels: Record<string, string>) => {
        const entries = Object.entries(labels ?? {});
        if (entries.length === 0) {
          return <span style={{ color: colorTextTertiary }}>—</span>;
        }
        return (
          <Space size={4} wrap>
            {entries.slice(0, 2).map(([k, v]) => (
              <Tag key={k}>{k}={v}</Tag>
            ))}
            {entries.length > 2 && (
              <Tooltip title={entries.slice(2).map(([k, v]) => `${k}=${v}`).join('\n')}>
                <Tag>+{entries.length - 2}</Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: t('config:list.columns.dataCount'),
      dataIndex: 'dataCount',
      key: 'dataCount',
      width: 120,
      align: 'center',
      sorter: true,
      sortOrder: sortField === 'dataCount' ? sortOrder : null,
      render: (count: number) => (
        <span style={{ color: colorTextSecondary }}>{count}</span>
      ),
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'creationTimestamp',
      key: 'creationTimestamp',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'creationTimestamp' ? sortOrder : null,
      render: (time: string) => {
        if (!time) return '-';
        return new Date(time).toLocaleString('zh-TW', {
          year: 'numeric', month: '2-digit', day: '2-digit',
          hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
        }).replace(/\//g, '-');
      },
    },
    {
      title: t('config:list.columns.age'),
      dataIndex: 'creationTimestamp',
      key: 'age',
      width: 100,
      render: (createdAt: string) => {
        if (!createdAt) return '-';
        const diff = dayjs().diff(dayjs(createdAt), 'minute');
        if (diff < 60) return `${diff}m`;
        if (diff < 1440) return `${Math.floor(diff / 60)}h`;
        return `${Math.floor(diff / 1440)}d`;
      },
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 90,
      fixed: 'right' as const,
      render: (_: unknown, record: ConfigMapListItem) => (
        <ActionButtons
          primary={[
            {
              key: 'view',
              label: t('common:actions.view'),
              icon: <EyeOutlined />,
              onClick: () => navigate(`/clusters/${clusterId}/configs/configmap/${record.namespace}/${record.name}`),
            },
            {
              key: 'edit',
              label: t('common:actions.edit'),
              icon: <EditOutlined />,
              onClick: () => navigate(`/clusters/${clusterId}/configs/configmap/${record.namespace}/${record.name}/edit`),
            },
          ]}
          more={[
            {
              key: 'history',
              label: t('config:list.columns.history'),
              icon: <HistoryOutlined />,
              onClick: () => navigate(`/clusters/${clusterId}/configs/configmap/${record.namespace}/${record.name}/history`),
            },
            {
              key: 'delete',
              label: t('common:actions.delete'),
              icon: <DeleteOutlined />,
              danger: true,
              confirm: {
                title: t('config:list.messages.confirmDeleteConfigMap'),
                description: t('config:list.messages.confirmDeleteDesc', { name: record.name }),
              },
              onClick: () => handleDelete(record.namespace, record.name),
            },
          ]}
        />
      ),
    },
  ];
}
