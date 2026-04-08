import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Space, Button, Select, App, Popconfirm } from 'antd';
import { ReloadOutlined, CheckCircleOutlined, QuestionCircleOutlined, PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import { parseApiError } from '@/utils/api';
import type { HTTPRouteItem, GatewayK8sCondition, GatewayTabProps } from './gatewayTypes';
import HTTPRouteDrawer from './HTTPRouteDrawer';
import HTTPRouteForm from './HTTPRouteForm';

const HTTPRouteList: React.FC<GatewayTabProps> = ({ clusterId, onCountChange }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const [items, setItems] = useState<HTTPRouteItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [drawerItem, setDrawerItem] = useState<HTTPRouteItem | null>(null);
  const [formVisible, setFormVisible] = useState(false);
  const [editingRoute, setEditingRoute] = useState<HTTPRouteItem | null>(null);

  const namespaces = [...new Set(items.map((i) => i.namespace))].sort();

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await gatewayService.listHTTPRoutes(clusterId);
      setItems(res.items ?? []);
      onCountChange?.(res.total ?? 0);
    } catch {
      message.error(t('network:gatewayapi.messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t, onCountChange]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleDelete = async (item: HTTPRouteItem) => {
    try {
      await gatewayService.deleteHTTPRoute(clusterId, item.namespace, item.name);
      message.success(t('network:gatewayapi.messages.deleteHTTPRouteSuccess'));
      loadData();
    } catch (err) {
      message.error(parseApiError(err) || t('network:gatewayapi.messages.deleteHTTPRouteError'));
    }
  };

  const filtered = namespaceFilter
    ? items.filter((i) => i.namespace === namespaceFilter)
    : items;

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
      title: t('network:gatewayapi.columns.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: HTTPRouteItem) => (
        <Button type="link" size="small" onClick={() => setDrawerItem(record)} style={{ padding: 0 }}>
          {name}
        </Button>
      ),
      sorter: (a: HTTPRouteItem, b: HTTPRouteItem) => a.name.localeCompare(b.name),
    },
    {
      title: t('network:gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.columns.hostnames'),
      key: 'hostnames',
      render: (_: unknown, record: HTTPRouteItem) =>
        record.hostnames.length > 0 ? (
          <Space wrap size={4}>
            {record.hostnames.map((h) => <Tag key={h} color="blue" style={{ fontSize: 12 }}>{h}</Tag>)}
          </Space>
        ) : '-',
    },
    {
      title: t('network:gatewayapi.columns.parentRefs'),
      key: 'parentRefs',
      render: (_: unknown, record: HTTPRouteItem) =>
        record.parentRefs.length > 0 ? (
          <Space wrap size={4}>
            {record.parentRefs.map((p, i) => (
              <Tag key={i} color="purple" style={{ fontSize: 12 }}>
                {p.gatewayName}
              </Tag>
            ))}
          </Space>
        ) : '-',
    },
    {
      title: t('network:gatewayapi.columns.rules'),
      key: 'rules',
      render: (_: unknown, record: HTTPRouteItem) => record.rules.length,
    },
    {
      title: t('network:gatewayapi.columns.status'),
      key: 'conditions',
      render: (_: unknown, record: HTTPRouteItem) => renderConditions(record.conditions),
    },
    {
      title: t('network:gatewayapi.columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: t('network:gatewayapi.columns.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 120,
      render: (_: unknown, record: HTTPRouteItem) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            onClick={() => { setEditingRoute(record); setFormVisible(true); }}
          >
            {t('common:actions.edit')}
          </Button>
          <Popconfirm
            title={t('network:gatewayapi.messages.confirmDeleteTitle')}
            description={t('network:gatewayapi.messages.confirmDeleteHTTPRoute', { name: record.name })}
            onConfirm={() => handleDelete(record)}
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

  return (
    <>
      <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
        <Select
          allowClear
          placeholder={t('network:gatewayapi.columns.namespace')}
          style={{ width: 200 }}
          value={namespaceFilter || undefined}
          onChange={(v) => setNamespaceFilter(v ?? '')}
          options={namespaces.map((ns) => ({ label: ns, value: ns }))}
        />
        <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading} />
        <div style={{ flex: 1 }} />
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { setEditingRoute(null); setFormVisible(true); }}
        >
          {t('network:gatewayapi.form.createHTTPRoute')}
        </Button>
      </div>

      <Table
        rowKey={(r) => `${r.namespace}/${r.name}`}
        loading={loading}
        dataSource={filtered}
        columns={columns}
        pagination={{
          pageSize: 20,
          showTotal: (total) => t('network:gatewayapi.pagination.httprouteTotal', { total }),
        }}
        size="middle"
      />

      <HTTPRouteDrawer
        open={!!drawerItem}
        clusterId={clusterId}
        item={drawerItem}
        onClose={() => setDrawerItem(null)}
      />

      <HTTPRouteForm
        open={formVisible}
        clusterId={clusterId}
        editing={editingRoute}
        onClose={() => setFormVisible(false)}
        onSuccess={loadData}
      />
    </>
  );
};

export default HTTPRouteList;
