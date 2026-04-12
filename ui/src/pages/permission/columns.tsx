import React from 'react';
import { Space, Tag, Button, Tooltip, Popconfirm } from 'antd';
import { UserOutlined, TeamOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { TFunction } from 'i18next';
import { Typography } from 'antd';
import type { ClusterPermission, Cluster } from '../../types';

const { Text } = Typography;

interface CreatePermissionColumnsParams {
  t: TFunction;
  clusters: Cluster[];
  getPermissionTypeColor: (type: string) => string;
  onEdit: (record: ClusterPermission) => void;
  onDelete: (record: ClusterPermission) => void;
}

export function createPermissionColumns({
  t,
  clusters,
  getPermissionTypeColor,
  onEdit,
  onDelete,
}: CreatePermissionColumnsParams): ColumnsType<ClusterPermission> {
  return [
    {
      title: t('permission:columns.subject'),
      key: 'subject',
      width: 200,
      render: (_, record) => (
        <Space>
          {record.user_id ? (
            <>
              <Tag color="blue" icon={<UserOutlined />}>{t('permission:columns.user')}</Tag>
              <Text>{record.username}</Text>
            </>
          ) : (
            <>
              <Tag color="green" icon={<TeamOutlined />}>{t('permission:columns.userGroup')}</Tag>
              <Text>{record.user_group_name}</Text>
            </>
          )}
        </Space>
      ),
    },
    {
      title: t('permission:columns.clusterName'),
      dataIndex: 'cluster_name',
      key: 'cluster_name',
      width: 150,
      render: (clusterName: string, record) => {
        const name = clusterName || clusters.find(c => parseInt(c.id) === record.cluster_id)?.name || '-';
        return <Text>{name}</Text>;
      },
    },
    {
      title: t('permission:columns.permissionType'),
      dataIndex: 'permission_type',
      key: 'permission_type',
      width: 150,
      render: (type: string) => (
        <Tag color={getPermissionTypeColor(type)}>
          {t(`permission:types.${type}.name`)}
        </Tag>
      ),
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespaces',
      key: 'namespaces',
      width: 200,
      render: (namespaces: string[]) => (
        <Text type={namespaces.includes('*') ? 'success' : undefined}>
          {namespaces.includes('*') ? t('permission:form.allNamespaces') : namespaces.join(', ')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'action',
      width: 150,
      render: (_, record) => (
        <Space>
          <Tooltip title={t('permission:actions.editTooltip')}>
            <Button
              type="link"
              size="small"
              onClick={() => onEdit(record)}
            >
              {t('permission:actions.editTooltip')}
            </Button>
          </Tooltip>
          <Popconfirm
            title={t('permission:actions.confirmDeletePermission')}
            onConfirm={() => onDelete(record)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button type="link" size="small" danger>
              {t('common:actions.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];
}
