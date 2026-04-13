import React from 'react';
import { Button, Space, Tag, Tooltip, Badge, Dropdown, Typography } from 'antd';
import type { MenuProps } from 'antd';
import { CodeOutlined, FileTextOutlined, MoreOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { PodInfo } from '../../services/podService';
import { PodService } from '../../services/podService';
import type { TFunction } from 'i18next';
import { getPodResources } from './podUtils';

interface PodColumnParams {
  t: TFunction;
  tc: TFunction;
  sortField: string;
  sortOrder: 'ascend' | 'descend' | null;
  clusterId: string;
  handleViewDetail: (pod: PodInfo) => void;
  handleLogs: (pod: PodInfo) => void;
  handleTerminal: (pod: PodInfo) => void;
  handleViewEvents: (pod: PodInfo) => void;
  confirmDelete: (pod: PodInfo) => void;
  canTerminalPod?: boolean;
  canDelete?: boolean;
  showActions?: boolean;
}

export const createPodColumns = (params: PodColumnParams): ColumnsType<PodInfo> => {
  const {
    t, tc, sortField, sortOrder,
    handleViewDetail, handleLogs, handleTerminal,
    handleViewEvents, confirmDelete,
    canTerminalPod = true,
    canDelete = true,
    showActions,
  } = params;

  const morePodActions = (record: PodInfo): MenuProps['items'] => [
    {
      key: 'monitor',
      label: tc('menu.monitoring'),
      onClick: () => handleViewDetail(record),
    },
    {
      key: 'events',
      label: t('detail.events'),
      onClick: () => handleViewEvents(record),
    },
    ...(canDelete ? [
      { type: 'divider' as const },
      {
        key: 'delete',
        label: tc('actions.delete'),
        danger: true,
        onClick: () => confirmDelete(record),
      },
    ] : []),
  ];

  return [
    {
      title: t('columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 220,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text: string, record: PodInfo) => (
        <Space size={4}>
          <Button
            type="link"
            onClick={() => handleViewDetail(record)}
            style={{
              padding: 0,
              height: 'auto',
              whiteSpace: 'normal',
              wordBreak: 'break-all',
              textAlign: 'left',
            }}
          >
            {text}
          </Button>
          <Typography.Text copyable={{ text }} style={{ fontSize: 12 }} />
        </Space>
      ),
    },
    {
      title: tc('table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      sorter: true,
      sortOrder: sortField === 'status' ? sortOrder : null,
      render: (_: unknown, record: PodInfo) => {
        const { status, color } = PodService.formatStatus(record);
        const getBadgeStatus = (color: string): 'success' | 'error' | 'default' | 'processing' | 'warning' => {
          switch (color) {
            case 'green': return 'success';
            case 'orange': return 'warning';
            case 'red': return 'error';
            case 'blue': return 'processing';
            default: return 'default';
          }
        };
        return <Badge status={getBadgeStatus(color)} text={status} />;
      },
    },
    {
      title: tc('table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('columns.podIP'),
      dataIndex: 'podIP',
      key: 'podIP',
      width: 130,
      render: (text: string) => text || '-',
    },
    {
      title: t('columns.nodeName'),
      dataIndex: 'nodeName',
      key: 'nodeName',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'nodeName' ? sortOrder : null,
      render: (text: string) => text || '-',
    },
    {
      title: t('columns.restarts'),
      dataIndex: 'restartCount',
      key: 'restartCount',
      width: 80,
      sorter: true,
      sortOrder: sortField === 'restartCount' ? sortOrder : null,
      render: (count: number) => (
        <Tag color={count > 0 ? 'orange' : 'green'}>{count}</Tag>
      ),
    },
    {
      title: 'CPU Request',
      key: 'cpuRequest',
      width: 110,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.cpuRequest}</span>;
      },
    },
    {
      title: 'CPU Limit',
      key: 'cpuLimit',
      width: 100,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.cpuLimit}</span>;
      },
    },
    {
      title: 'MEM Request',
      key: 'memoryRequest',
      width: 120,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.memoryRequest}</span>;
      },
    },
    {
      title: 'MEM Limit',
      key: 'memoryLimit',
      width: 110,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.memoryLimit}</span>;
      },
    },
    {
      title: tc('table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (text: string) => {
        if (!text) return '-';
        return <span>{new Date(text).toLocaleString()}</span>;
      },
    },
    {
      title: t('columns.age'),
      key: 'age',
      width: 100,
      render: (_: unknown, record: PodInfo) => PodService.getAge(record.createdAt),
    },
    ...(showActions !== false ? [{
      title: tc('table.actions'),
      key: 'actions',
      width: 120,
      fixed: 'right' as const,
      render: (_: unknown, record: PodInfo) => (
        <Space size={0}>
          {canTerminalPod && (
            <Tooltip title={t('actions.terminal')}>
              <Button
                type="link"
                size="small"
                icon={<CodeOutlined />}
                onClick={() => handleTerminal(record)}
                disabled={record.status !== 'Running'}
              />
            </Tooltip>
          )}
          <Tooltip title={t('actions.logs')}>
            <Button
              type="link"
              size="small"
              icon={<FileTextOutlined />}
              onClick={() => handleLogs(record)}
            />
          </Tooltip>
          <Dropdown menu={{ items: morePodActions(record) }} trigger={['click']}>
            <Button type="link" size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    }] : []),
  ];
};
