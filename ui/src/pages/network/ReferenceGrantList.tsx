import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Space, Button, Select, App, Modal, Popconfirm } from 'antd';
import { ReloadOutlined, PlusOutlined } from '@ant-design/icons';
import EmptyState from '@/components/EmptyState';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import { showApiError } from '@/utils/api';
import type { ReferenceGrantItem, ReferenceGrantPeer, GatewayTabProps } from './gatewayTypes';
import MonacoEditor from '@monaco-editor/react';
import ReferenceGrantForm from './ReferenceGrantForm';
import { usePermission } from '../../hooks/usePermission';

const ReferenceGrantList: React.FC<GatewayTabProps> = ({ clusterId, onCountChange }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const { canWrite, canDelete } = usePermission();
  const [items, setItems] = useState<ReferenceGrantItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [createVisible, setCreateVisible] = useState(false);
  const [yamlViewItem, setYamlViewItem] = useState<ReferenceGrantItem | null>(null);
  const [viewYaml, setViewYaml] = useState('');

  const namespaces = [...new Set(items.map((i) => i.namespace))].sort();

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await gatewayService.listReferenceGrants(clusterId);
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

  const handleDelete = async (item: ReferenceGrantItem) => {
    try {
      await gatewayService.deleteReferenceGrant(clusterId, item.namespace, item.name);
      message.success(t('gatewayapi.messages.deleteReferenceGrantSuccess'));
      loadData();
    } catch (err) {
      showApiError(err, t('gatewayapi.messages.deleteReferenceGrantError'));
    }
  };

  const handleViewYAML = async (item: ReferenceGrantItem) => {
    setYamlViewItem(item);
    try {
      const r = await gatewayService.getReferenceGrantYAML(clusterId, item.namespace, item.name);
      setViewYaml(r.yaml);
    } catch {
      setViewYaml('# Failed to load YAML');
    }
  };

  const renderPeers = (peers: ReferenceGrantPeer[]) => (
    <Space wrap size={4}>
      {peers.map((p, i) => (
        <Tag key={i} style={{ fontSize: 11 }}>
          {p.kind}{p.namespace ? `@${p.namespace}` : ''}{p.name ? `/${p.name}` : ''}
        </Tag>
      ))}
    </Space>
  );

  const columns = [
    {
      title: t('gatewayapi.columns.name'),
      dataIndex: 'name',
      key: 'name',
      sorter: (a: ReferenceGrantItem, b: ReferenceGrantItem) => a.name.localeCompare(b.name),
    },
    {
      title: t('gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('gatewayapi.refgrant.from'),
      key: 'from',
      render: (_: unknown, r: ReferenceGrantItem) => renderPeers(r.from),
    },
    {
      title: t('gatewayapi.refgrant.to'),
      key: 'to',
      render: (_: unknown, r: ReferenceGrantItem) => renderPeers(r.to),
    },
    {
      title: t('gatewayapi.columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
    ...((canWrite() || canDelete()) ? [{
      title: t('gatewayapi.columns.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 140,
      render: (_: unknown, record: ReferenceGrantItem) => (
        <Space size="small">
          <Button type="link" size="small" onClick={() => handleViewYAML(record)}>
            YAML
          </Button>
          {canDelete() && (
            <Popconfirm
              title={t('gatewayapi.messages.confirmDeleteTitle')}
              description={t('gatewayapi.messages.confirmDeleteReferenceGrant', { name: record.name })}
              onConfirm={() => handleDelete(record)}
              okText={t('common:actions.confirm')}
              cancelText={t('common:actions.cancel')}
            >
              <Button type="link" size="small" danger>
                {t('common:actions.delete')}
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    }] : []),
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
        {canWrite() && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateVisible(true)}>
            {t('gatewayapi.refgrant.create')}
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
          showTotal: (total) => t('gatewayapi.pagination.referencegrantTotal', { total }),
        }}
        size="middle"
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      <ReferenceGrantForm
        open={createVisible}
        clusterId={clusterId}
        onClose={() => setCreateVisible(false)}
        onSuccess={() => { setCreateVisible(false); loadData(); }}
      />

      {/* YAML View Modal */}
      <Modal
        title={`YAML — ${yamlViewItem?.namespace}/${yamlViewItem?.name}`}
        open={!!yamlViewItem}
        onCancel={() => setYamlViewItem(null)}
        footer={null}
        width={860}
        destroyOnClose
      >
        <MonacoEditor
          height="420px"
          language="yaml"
          value={viewYaml}
          options={{ readOnly: true, minimap: { enabled: false }, fontSize: 13, scrollBeyondLastLine: false }}
        />
      </Modal>
    </>
  );
};

export default ReferenceGrantList;
