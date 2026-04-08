import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Space, Button, Select, App, Modal, Popconfirm } from 'antd';
import {
  ReloadOutlined,
  PlusOutlined,
  CheckCircleOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import { parseApiError } from '@/utils/api';
import type { GRPCRouteItem, GatewayK8sCondition, GatewayTabProps } from './gatewayTypes';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';

const DEFAULT_YAML = `apiVersion: gateway.networking.k8s.io/v1
kind: GRPCRoute
metadata:
  name: my-grpcroute
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - method:
            service: foo.v1.Foo
            method: Bar
      backendRefs:
        - name: my-service
          port: 50051
`;

const GRPCRouteList: React.FC<GatewayTabProps> = ({ clusterId, onCountChange }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const [items, setItems] = useState<GRPCRouteItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [formVisible, setFormVisible] = useState(false);
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);
  const [editingItem, setEditingItem] = useState<GRPCRouteItem | null>(null);
  const [submitting, setSubmitting] = useState(false);

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
    setYamlContent(DEFAULT_YAML);
    setFormVisible(true);
  };

  const handleEdit = async (item: GRPCRouteItem) => {
    setEditingItem(item);
    try {
      const r = await gatewayService.getGRPCRouteYAML(clusterId, item.namespace, item.name);
      setYamlContent(r.yaml);
    } catch {
      setYamlContent(DEFAULT_YAML);
    }
    setFormVisible(true);
  };

  const handleDelete = async (item: GRPCRouteItem) => {
    try {
      await gatewayService.deleteGRPCRoute(clusterId, item.namespace, item.name);
      message.success(t('gatewayapi.messages.deleteGRPCRouteSuccess'));
      loadData();
    } catch (err) {
      message.error(parseApiError(err) || t('gatewayapi.messages.deleteGRPCRouteError'));
    }
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      const parsed = YAML.parse(yamlContent) as Record<string, unknown>;
      const ns = (parsed?.metadata as Record<string, string>)?.['namespace'] ?? 'default';
      if (editingItem) {
        await gatewayService.updateGRPCRoute(clusterId, editingItem.namespace, editingItem.name, yamlContent);
        message.success(t('gatewayapi.messages.updateGRPCRouteSuccess'));
      } else {
        await gatewayService.createGRPCRoute(clusterId, ns, yamlContent);
        message.success(t('gatewayapi.messages.createGRPCRouteSuccess'));
      }
      setFormVisible(false);
      loadData();
    } catch (err) {
      message.error(parseApiError(err) || t(editingItem ? 'gatewayapi.messages.updateGRPCRouteError' : 'gatewayapi.messages.createGRPCRouteError'));
    } finally {
      setSubmitting(false);
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
      width: 120,
      render: (_: unknown, record: GRPCRouteItem) => (
        <Space size="small">
          <Button type="link" size="small" onClick={() => handleEdit(record)}>
            {t('common:actions.edit')}
          </Button>
          <Popconfirm
            title={t('gatewayapi.messages.confirmDeleteTitle')}
            description={t('gatewayapi.messages.confirmDeleteGRPCRoute', { name: record.name })}
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
          placeholder={t('gatewayapi.columns.namespace')}
          style={{ width: 200 }}
          value={namespaceFilter || undefined}
          onChange={(v) => setNamespaceFilter(v ?? '')}
          options={namespaces.map((ns) => ({ label: ns, value: ns }))}
        />
        <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading} />
        <div style={{ flex: 1 }} />
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('gatewayapi.form.createGRPCRoute')}
        </Button>
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
      />

      <Modal
        title={editingItem ? t('gatewayapi.form.editGRPCRoute', { name: editingItem.name }) : t('gatewayapi.form.createGRPCRoute')}
        open={formVisible}
        onCancel={() => setFormVisible(false)}
        onOk={handleSubmit}
        okText={editingItem ? t('gatewayapi.form.saveBtn') : t('gatewayapi.form.createBtn')}
        cancelText={t('gatewayapi.form.cancel')}
        confirmLoading={submitting}
        width={860}
        destroyOnClose
      >
        <MonacoEditor
          height="500px"
          language="yaml"
          value={yamlContent}
          onChange={(v) => setYamlContent(v || '')}
          options={{ minimap: { enabled: false }, fontSize: 13, wordWrap: 'on', scrollBeyondLastLine: false }}
        />
      </Modal>
    </>
  );
};

export default GRPCRouteList;
