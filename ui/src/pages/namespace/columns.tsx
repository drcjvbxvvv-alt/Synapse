import React from 'react';
import { Button, Space, Tag, Tooltip } from 'antd';
import { EyeOutlined, DeleteOutlined } from '@ant-design/icons';
import { StatusTag } from '../../components/StatusTag';
import { ActionButtons } from '../../components/ActionButtons';
import type { ColumnsType } from 'antd/es/table';
import type { NamespaceData } from '../../services/namespaceService';
import type { TFunction } from 'i18next';

interface CreateNamespaceColumnsParams {
  t: TFunction;
  token: {
    colorTextTertiary: string;
    fontSizeSM: number;
  };
  sortField: string;
  sortOrder: 'ascend' | 'descend' | null;
  SYSTEM_NAMESPACES: string[];
  handleViewDetail: (namespace: string) => void;
  handleDelete: (namespace: string) => void;
  canDelete?: boolean;
  showActions?: boolean;
}

export function createNamespaceColumns({
  t,
  token,
  sortField,
  sortOrder,
  SYSTEM_NAMESPACES,
  handleViewDetail,
  handleDelete,
  canDelete = true,
  showActions,
}: CreateNamespaceColumnsParams): ColumnsType<NamespaceData> {
  return [
    {
      title: t('columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 200,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string) => (
        <Button
          type="link"
          onClick={() => handleViewDetail(name)}
          style={{
            padding: 0,
            height: 'auto',
            whiteSpace: 'normal',
            wordBreak: 'break-all',
            textAlign: 'left',
          }}
        >
          {name}
        </Button>
      ),
    },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      sorter: true,
      sortOrder: sortField === 'status' ? sortOrder : null,
      render: (status: string) => <StatusTag status={status} />,
    },
    {
      title: t('columns.labels'),
      dataIndex: 'labels',
      key: 'labels',
      width: 250,
      render: (labels: Record<string, string>) => {
        if (!labels || Object.keys(labels).length === 0) {
          return (
            <span style={{ color: token.colorTextTertiary, fontSize: token.fontSizeSM }}>--</span>
          );
        }
        const entries = Object.entries(labels);
        const visible = entries.slice(0, 2);
        const rest = entries.slice(2);
        return (
          <Space size={[0, 4]} wrap>
            {visible.map(([k, v]) => (
              <Tag key={k} style={{ fontSize: token.fontSizeSM }}>{k}={v}</Tag>
            ))}
            {rest.length > 0 && (
              <Tooltip title={rest.map(([k, v]) => `${k}=${v}`).join('\n')}>
                <Tag style={{ cursor: 'pointer', fontSize: token.fontSizeSM }}>+{rest.length}</Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: t('columns.createdAt'),
      dataIndex: 'creationTimestamp',
      key: 'creationTimestamp',
      width: 160,
      sorter: true,
      sortOrder: sortField === 'creationTimestamp' ? sortOrder : null,
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
          hour12: false,
        }).replace(/\//g, '-');
        return <span>{formatted}</span>;
      },
    },
    ...(showActions !== false ? [{
      title: t('common:table.actions'),
      key: 'actions',
      width: 90,
      fixed: 'right' as const,
      render: (_: unknown, record: NamespaceData) => {
        const isSystem = SYSTEM_NAMESPACES.includes(record.name);
        return (
          <ActionButtons
            primary={[{
              key: 'view',
              label: t('common:actions.viewDetails'),
              icon: <EyeOutlined />,
              onClick: () => handleViewDetail(record.name),
            }]}
            more={[
              ...(!isSystem && canDelete ? [{
                key: 'delete',
                label: t('common:actions.delete'),
                icon: <DeleteOutlined />,
                danger: true as const,
                confirm: {
                  title: t('actions.confirmDelete'),
                  description: t('actions.confirmDeleteDesc', { name: record.name }),
                },
                onClick: () => handleDelete(record.name),
              }] : []),
            ]}
          />
        );
      },
    }] : []),
  ];
}
