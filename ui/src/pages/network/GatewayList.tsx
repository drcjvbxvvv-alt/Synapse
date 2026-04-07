import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Space, Button, Select, App } from 'antd';
import { ReloadOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import { parseApiError } from '@/utils/api';
import type { GatewayItem, GatewayListener, GatewayTabProps } from './gatewayTypes';
import GatewayDrawer from './GatewayDrawer';
import GatewayForm from './GatewayForm';

const GatewayList: React.FC<GatewayTabProps> = ({ clusterId, onCountChange }) => {
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const [items, setItems] = useState<GatewayItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [drawerItem, setDrawerItem] = useState<GatewayItem | null>(null);
  const [formVisible, setFormVisible] = useState(false);
  const [editingGateway, setEditingGateway] = useState<GatewayItem | null>(null);

  const namespaces = [...new Set(items.map((i) => i.namespace))].sort();

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await gatewayService.listGateways(clusterId);
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

  const handleDelete = (item: GatewayItem) => {
    modal.confirm({
      title: t('network:gatewayapi.messages.confirmDeleteTitle'),
      content: t('network:gatewayapi.messages.confirmDeleteGateway', { name: item.name }),
      okType: 'danger',
      onOk: async () => {
        try {
          await gatewayService.deleteGateway(clusterId, item.namespace, item.name);
          message.success(t('network:gatewayapi.messages.deleteGatewaySuccess'));
          loadData();
        } catch (err) {
          message.error(parseApiError(err) || t('network:gatewayapi.messages.deleteGatewayError'));
        }
      },
    });
  };

  const filtered = namespaceFilter
    ? items.filter((i) => i.namespace === namespaceFilter)
    : items;

  const renderListeners = (listeners: GatewayListener[]) => (
    <Space wrap size={4}>
      {listeners.map((l) => (
        <Tag key={l.name} color="blue" style={{ fontSize: 12 }}>
          {l.protocol}/{l.port}
        </Tag>
      ))}
    </Space>
  );

  const renderStatus = (item: GatewayItem) => {
    const programmed = item.conditions.find((c) => c.type === 'Programmed');
    if (!programmed) return <Tag color="default">Unknown</Tag>;
    return programmed.status === 'True' ? (
      <Tag color="success">{t('network:gatewayapi.status.programmed')}</Tag>
    ) : (
      <Tag color="warning">{programmed.reason || programmed.status}</Tag>
    );
  };

  const columns = [
    {
      title: t('network:gatewayapi.columns.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: GatewayItem) => (
        <Button type="link" size="small" onClick={() => setDrawerItem(record)} style={{ padding: 0 }}>
          {name}
        </Button>
      ),
      sorter: (a: GatewayItem, b: GatewayItem) => a.name.localeCompare(b.name),
    },
    {
      title: t('network:gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.columns.gatewayClass'),
      dataIndex: 'gatewayClass',
      key: 'gatewayClass',
      render: (v: string) => <Tag color="purple">{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.columns.listeners'),
      key: 'listeners',
      render: (_: unknown, record: GatewayItem) => renderListeners(record.listeners),
    },
    {
      title: t('network:gatewayapi.columns.addresses'),
      key: 'addresses',
      render: (_: unknown, record: GatewayItem) =>
        record.addresses.length > 0 ? (
          <Space wrap size={4}>
            {record.addresses.map((a, i) => (
              <Tag key={i} color="cyan">{a.value}</Tag>
            ))}
          </Space>
        ) : '-',
    },
    {
      title: t('network:gatewayapi.columns.status'),
      key: 'status',
      render: (_: unknown, record: GatewayItem) => renderStatus(record),
    },
    {
      title: t('network:gatewayapi.columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: t('common:actions', 'Actions'),
      key: 'actions',
      width: 100,
      render: (_: unknown, record: GatewayItem) => (
        <Space size={4}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => { setEditingGateway(record); setFormVisible(true); }}
          />
          <Button
            type="link"
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record)}
          />
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
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { setEditingGateway(null); setFormVisible(true); }}
        >
          {t('network:gatewayapi.form.createGateway')}
        </Button>
      </div>

      <Table
        rowKey={(r) => `${r.namespace}/${r.name}`}
        loading={loading}
        dataSource={filtered}
        columns={columns}
        pagination={{
          pageSize: 20,
          showTotal: (total) => t('network:gatewayapi.pagination.gatewayTotal', { total }),
        }}
        size="middle"
      />

      <GatewayDrawer
        open={!!drawerItem}
        clusterId={clusterId}
        item={drawerItem}
        onClose={() => setDrawerItem(null)}
      />

      <GatewayForm
        open={formVisible}
        clusterId={clusterId}
        editing={editingGateway}
        onClose={() => setFormVisible(false)}
        onSuccess={loadData}
      />
    </>
  );
};

export default GatewayList;
