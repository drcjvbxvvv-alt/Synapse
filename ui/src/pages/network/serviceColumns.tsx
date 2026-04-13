import React from 'react';
import { Tag, Typography, Tooltip } from 'antd';
import { CodeOutlined, EditOutlined, ApiOutlined, DeleteOutlined } from '@ant-design/icons';
import { ActionButtons } from '../../components/ActionButtons';
import { ServiceService } from '../../services/serviceService';
import type { Service } from '../../types';
import type { ColumnsType } from 'antd/es/table';
import type { TFunction } from 'i18next';

const { Text, Link } = Typography;

interface ServiceColumnsOptions {
  sortField: string;
  sortOrder: 'ascend' | 'descend' | null;
  onViewYAML: (service: Service) => void;
  onEdit: (service: Service) => void;
  onViewEndpoints: (service: Service) => void;
  onDelete: (service: Service) => void;
  t: TFunction;
  canDelete?: boolean;
  showActions?: boolean;
}

export function getServiceColumns(options: ServiceColumnsOptions): ColumnsType<Service> {
  const { sortField, sortOrder, onViewYAML, onEdit, onViewEndpoints, onDelete, t, canDelete = true, showActions } = options;

  return [
    {
      title: t('network:service.columns.name'),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left' as const,
      width: 200,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string, record: Service) => (
        <div>
          <Link strong onClick={() => onViewYAML(record)}>
            {name}
          </Link>
          <div style={{ fontSize: 12, color: '#999' }}>
            {record.namespace}
          </div>
        </div>
      ),
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('common:table.type'),
      dataIndex: 'type',
      key: 'type',
      width: 150,
      render: (type: string) => (
        <Tag color={ServiceService.getTypeColor(type)}>
          {ServiceService.getTypeTag(type)}
        </Tag>
      ),
    },
    {
      title: t('network:service.columns.access'),
      key: 'access',
      width: 200,
      render: (_: unknown, record: Service) => {
        const addresses = ServiceService.formatAccessAddress(record);
        return (
          <div>
            {addresses.slice(0, 2).map((addr, idx) => (
              <div key={idx} style={{ fontSize: 12 }}>
                {addr}
              </div>
            ))}
            {addresses.length > 2 && (
              <Tooltip title={addresses.slice(2).join(', ')}>
                <Text type="secondary" style={{ fontSize: 12, cursor: 'pointer' }}>
                  +{addresses.length - 2} {t('network:service.columns.more')}
                </Text>
              </Tooltip>
            )}
          </div>
        );
      },
    },
    {
      title: t('network:service.columns.ports'),
      key: 'ports',
      width: 180,
      render: (_: unknown, record: Service) => (
        <Tooltip title={ServiceService.formatPorts(record)}>
          <Text ellipsis style={{ width: 160, display: 'block' }}>
            {ServiceService.formatPorts(record)}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('network:service.columns.selector'),
      key: 'selector',
      width: 200,
      render: (_: unknown, record: Service) => (
        <Tooltip title={ServiceService.formatSelector(record.selector)}>
          <Text ellipsis style={{ width: 180, display: 'block' }}>
            {ServiceService.formatSelector(record.selector)}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 160,
      ellipsis: true,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (createdAt: string) => {
        if (!createdAt) return '-';
        const date = new Date(createdAt);
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
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 90,
      render: (_: unknown, record: Service) => (
        <ActionButtons
          primary={[
            { key: 'yaml', label: 'YAML', icon: <CodeOutlined />, onClick: () => onViewYAML(record) },
            { key: 'edit', label: t('common:actions.edit'), icon: <EditOutlined />, onClick: () => onEdit(record) },
          ]}
          more={[
            { key: 'endpoints', label: 'Endpoints', icon: <ApiOutlined />, onClick: () => onViewEndpoints(record) },
            ...(canDelete ? [{
              key: 'delete', label: t('common:actions.delete'), icon: <DeleteOutlined />, danger: true as const,
              onClick: () => onDelete(record),
              confirm: {
                title: t('network:service.messages.confirmDeleteItem'),
                description: t('network:service.messages.confirmDeleteDesc', { name: record.name }),
              },
            }] : []),
          ]}
        />
      ),
    }] : []),
  ];
}
