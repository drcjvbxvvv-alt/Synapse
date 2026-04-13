import React from 'react';
import { Button, Space, Tag, Tooltip, Badge, Dropdown, Popconfirm } from 'antd';
import type { MenuProps } from 'antd';
import {
  LineChartOutlined,
  EditOutlined,
  MoreOutlined,
  ColumnWidthOutlined,
  ReloadOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { WorkloadInfo } from '../../services/workloadService';
import type { TFunction } from 'i18next';
import type { WorkloadType } from './hooks/useWorkloadTab';

interface WorkloadColumnParams {
  t: TFunction;
  workloadType: WorkloadType;
  sortField: string;
  sortOrder: 'ascend' | 'descend' | null;
  navigateToDetail: (workload: WorkloadInfo) => void;
  handleMonitor: (workload: WorkloadInfo) => void;
  handleEdit: (workload: WorkloadInfo) => void;
  openScaleModal: (workload: WorkloadInfo) => void;
  handleRestart: (workload: WorkloadInfo) => void;
  handleDelete: (workload: WorkloadInfo) => void;
  canDelete?: boolean;
  showActions?: boolean;
}

export const createWorkloadColumns = (params: WorkloadColumnParams): ColumnsType<WorkloadInfo> => {
  const {
    t, workloadType, sortField, sortOrder,
    navigateToDetail, handleMonitor, handleEdit,
    openScaleModal, handleRestart, handleDelete, canDelete = true,
    showActions,
  } = params;

  const moreActions = (record: WorkloadInfo): MenuProps['items'] => [
    {
      key: 'scale',
      label: t('actions.scale'),
      icon: <ColumnWidthOutlined />,
      onClick: () => openScaleModal(record),
    },
    {
      key: 'restart',
      label: t('actions.restart'),
      icon: <ReloadOutlined />,
      onClick: () => handleRestart(record),
    },
    ...(canDelete ? [
      { type: 'divider' as const },
      {
        key: 'delete',
        danger: true,
        icon: <DeleteOutlined />,
        label: (
          <Popconfirm
            title={t('actions.confirmDelete', { type: workloadType })}
            description={t('actions.confirmDeleteDesc', { name: record.name })}
            onConfirm={() => handleDelete(record)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            {t('common:actions.delete')}
          </Popconfirm>
        ),
      },
    ] : []),
  ];

  return [
    {
      title: t('columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 200,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text: string, record: WorkloadInfo) => (
        <Button
          type="link"
          onClick={() => navigateToDetail(record)}
          style={{
            padding: 0,
            height: 'auto',
            whiteSpace: 'normal',
            wordBreak: 'break-all',
            textAlign: 'left'
          }}
        >
          {text}
        </Button>
      ),
    },
    {
      title: t('columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        let color: 'success' | 'error' | 'default' | 'warning' = 'success';
        if (status === 'Stopped') {
          color = 'default';
        } else if (status === 'Degraded') {
          color = 'warning';
        } else if (status === 'Running') {
          color = 'success';
        }
        return <Badge status={color} text={status} />;
      },
    },
    {
      title: t('columns.replicas'),
      dataIndex: 'replicas',
      key: 'replicas',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'replicas' ? sortOrder : null,
      render: (_: unknown, record: WorkloadInfo) => (
        <span>{record.readyReplicas || 0} / {record.replicas || 0}</span>
      ),
    },
    {
      title: t('columns.cpuLimit'),
      dataIndex: 'cpuLimit',
      key: 'cpuLimit',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.cpuRequest'),
      dataIndex: 'cpuRequest',
      key: 'cpuRequest',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.memoryLimit'),
      dataIndex: 'memoryLimit',
      key: 'memoryLimit',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.memoryRequest'),
      dataIndex: 'memoryRequest',
      key: 'memoryRequest',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.images'),
      dataIndex: 'images',
      key: 'images',
      width: 250,
      render: (images: string[]) => {
        if (!images || images.length === 0) return '-';

        const firstImage = images[0];
        const imageNameVersion = firstImage.split('/').pop() || firstImage;

        return (
          <div>
            <Tooltip title={firstImage}>
              <Tag style={{ marginBottom: 2, maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {imageNameVersion}
              </Tag>
            </Tooltip>
            {images.length > 1 && (
              <Tooltip title={images.slice(1).map(img => img.split('/').pop()).join('\n')}>
                <Tag style={{ marginBottom: 2 }}>+{images.length - 1}</Tag>
              </Tooltip>
            )}
          </div>
        );
      },
    },
    {
      title: t('columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (text: string) => {
        if (!text) return '-';
        const date = new Date(text);
        const formatted = date.toLocaleString('zh-TW', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-');
        return <span>{formatted}</span>;
      },
    },
    ...(showActions !== false ? [{
      title: t('columns.actions'),
      key: 'actions',
      width: 120,
      fixed: 'right' as const,
      render: (record: WorkloadInfo) => (
        <Space size={0}>
          <Tooltip title={t('actions.monitoring')}>
            <Button type="link" size="small" icon={<LineChartOutlined />} onClick={() => handleMonitor(record)} />
          </Tooltip>
          <Tooltip title={t('common:actions.edit')}>
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Dropdown menu={{ items: moreActions(record) }} trigger={['click']}>
            <Button type="link" size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    }] : []),
  ];
};
