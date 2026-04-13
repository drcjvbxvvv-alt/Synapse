import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Space, Button, Select, App } from 'antd';
import {
  ReloadOutlined,
  PlusOutlined,
  CheckCircleOutlined,
  QuestionCircleOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import EmptyState from '@/components/EmptyState';
import { ActionButtons } from '../../components/ActionButtons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import { showApiError } from '@/utils/api';
import type { GRPCRouteItem, GatewayK8sCondition, GatewayTabProps } from './gatewayTypes';
import GRPCRouteForm from './GRPCRouteForm';
import { usePermission } from '../../hooks/usePermission';

const GRPCRouteList: React.FC<GatewayTabProps> = ({ clusterId, onCountChange }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const { hasFeature } = usePermission();
  const [items, setItems] = useState<GRPCRouteItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [formVisible, setFormVisible] = useState(false);
  const [editingItem, setEditingItem] = useState<GRPCRouteItem | null>(null);

  const namespaces = [...new Set(items.map((i) => i.namespace))].sort();

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await gatewayService.listGRPCRoutes(clusterId);
      setItems(res.items ?? []);
      onCountChange?.(res.total ?? 0);
    } catch {
      message.error(t('gatewayapi.messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t, onCountChange]);

  useEffect(() => { loadData(); }, [loadData]);

  const filtered = namespaceFilter ? items.filter((i) => i.namespace === namespaceFilter) : items;

  const handleCreate = () => {
    setEditingItem(null);
    setFormVisible(true);
  };

  const handleEdit = (item: GRPCRouteItem) => {
    setEditingItem(item);
    setFormVisible(true);
  };

  const handleDelete = async (item: GRPCRouteItem) => {
    try {
      await gatewayService.deleteGRPCRoute(clusterId, item.namespace, item.name);
      message.success(t('gatewayapi.messages.deleteGRPCRouteSuccess'));
      loadData();
    } catch (err) {
      showApiError(err, t('gatewayapi.messages.deleteGRPCRouteError'));
    }
  };

  const renderConditions = (conditions: GatewayK8sCondition[]) => (
    <Space wrap size={4}>
      {conditions.slice(0, 3).map((c) => {
        const color = c.status === 'True' ? 'success' : c.status === 'False' ? 'error' : 'default';
        const icon = c.status === 'True' ? <CheckCircleOutlined /> : <QuestionCircleOutlined />;
        return (
          <Tag key={c.type} icon={icon} color={color} title={c.message} style={{ fontSize: 11 }}>
            {c.type}
          </Tag>
        );
      })}
    </Space>
  );

  const columns = [
    {
      title: t('gatewayapi.columns.name'),
      dataIndex: 'name',
      key: 'name',
      sorter: (a: GRPCRouteItem, b: GRPCRouteItem) => a.name.localeCompare(b.name),
    },
    {
      title: t('gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('gatewayapi.columns.hostnames'),
      key: 'hostnames',
      render: (_: unknown, r: GRPCRouteItem) =>
        r.hostnames.length > 0 ? (
          <Space wrap size={4}>
            {r.hostnames.map((h) => <Tag key={h} color="blue" style={{ fontSize: 12 }}>{h}</Tag>)}
          </Space>
        ) : '-',
    },
    {
      title: t('gatewayapi.columns.parentRefs'),
      key: 'parentRefs',
      render: (_: unknown, r: GRPCRouteItem) =>
        r.parentRefs.length > 0 ? (
          <Space wrap size={4}>
            {r.parentRefs.map((p, i) => <Tag key={i} color="purple" style={{ fontSize: 12 }}>{p.gatewayName}</Tag>)}
          </Space>
        ) : '-',
    },
    {
      title: t('gatewayapi.columns.rules'),
      key: 'rules',
      render: (_: unknown, r: GRPCRouteItem) => r.rules.length,
    },
    {
      title: t('gatewayapi.columns.status'),
      key: 'conditions',
      render: (_: unknown, r: GRPCRouteItem) => renderConditions(r.conditions),
    },
    {
      title: t('gatewayapi.columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: t('gatewayapi.columns.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 70,
      render: (_: unknown, record: GRPCRouteItem) => (
        <ActionButtons
          primary={[
            { key: 'edit', label: t('common:actions.edit'), icon: <EditOutlined />, onClick: () => handleEdit(record) },
          ]}
          more={[
            ...(hasFeature('network:delete') ? [{
              key: 'delete', label: t('common:actions.delete'), icon: <DeleteOutlined />, danger: true as const,
              onClick: () => handleDelete(record),
              confirm: {
                title: t('gatewayapi.messages.confirmDeleteTitle'),
                description: t('gatewayapi.messages.confirmDeleteGRPCRoute', { name: record.name }),
              },
            }] : []),
          ]}
        />
      ),
    },
  ];

  return (
    <>
      <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
        <Select
          allowClear
          placeholder={t('gatewayapi.columns.namespace')}
          style={{ width: 200 }}
          value={namespaceFilter || undefined}
          onChange={(v) => setNamespaceFilter(v ?? '')}
          options={namespaces.map((ns) => ({ label: ns, value: ns }))}
        />
        <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading} />
        <div style={{ flex: 1 }} />
        {hasFeature('network:write') && (
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            {t('gatewayapi.form.createGRPCRoute')}
          </Button>
        )}
      </div>

      <Table
        rowKey={(r) => `${r.namespace}/${r.name}`}
        loading={loading}
        dataSource={filtered}
        columns={columns}
        pagination={{
          pageSize: 20,
          showTotal: (total) => t('gatewayapi.pagination.grpcrouteTotal', { total }),
        }}
        size="middle"
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
        scroll={{ x: 'max-content' }}
      />

      <GRPCRouteForm
        open={formVisible}
        clusterId={clusterId}
        editing={editingItem}
        onClose={() => setFormVisible(false)}
        onSuccess={() => { setFormVisible(false); loadData(); }}
      />
    </>
  );
};

export default GRPCRouteList;
