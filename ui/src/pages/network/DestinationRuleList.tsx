import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  App,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  CodeOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { ActionButtons } from '../../components/ActionButtons';
import { useTranslation } from 'react-i18next';
import MonacoEditor from '@monaco-editor/react';
import { MeshService, type DestinationRuleSummary } from '../../services/meshService';
import { usePermission } from '../../hooks/usePermission';

interface DestinationRuleListProps {
  clusterId: string;
  namespaces: string[];
  namespace?: string;
}

interface CreateDRValues {
  name: string;
  namespace: string;
  host: string;
  connectionPoolMaxConnections: number;
  connectionPoolHttp1MaxPending: number;
  outlierConsecutiveErrors: number;
  outlierInterval: string;
  outlierBaseEjectionTime: string;
}

const DestinationRuleList: React.FC<DestinationRuleListProps> = ({
  clusterId,
  namespaces,
  namespace: propNamespace,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const { canWrite, canDelete } = usePermission();
  const [items, setItems] = useState<DestinationRuleSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespace, setNamespace] = useState(propNamespace ?? '');
  const [createOpen, setCreateOpen] = useState(false);
  const [yamlOpen, setYamlOpen] = useState(false);
  const [yamlContent, setYamlContent] = useState('');
  const [form] = Form.useForm<CreateDRValues>();
  const [saving, setSaving] = useState(false);

  const fetchList = useCallback(async () => {
    setLoading(true);
    try {
      const res = await MeshService.listDestinationRules(clusterId, namespace || undefined);
      const data = (res as unknown as { data: { items: DestinationRuleSummary[] } }).data ?? res;
      setItems((data as { items: DestinationRuleSummary[] }).items ?? []);
    } catch {
      message.error(t('network:servicemesh.messages.fetchDRError', 'DestinationRule 列表載入失敗'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, message, t]);

  useEffect(() => {
    fetchList();
  }, [fetchList]);

  const handleViewYAML = async (record: DestinationRuleSummary) => {
    try {
      const res = await MeshService.getDestinationRule(clusterId, record.namespace, record.name);
      const obj = (res as unknown as { data: Record<string, unknown> }).data ?? res;
      setYamlContent(JSON.stringify(obj, null, 2));
      setYamlOpen(true);
    } catch {
      message.error(t('network:servicemesh.messages.fetchDRDetailError', 'DestinationRule 詳情載入失敗'));
    }
  };

  const handleDelete = async (record: DestinationRuleSummary) => {
    try {
      await MeshService.deleteDestinationRule(clusterId, record.namespace, record.name);
      message.success(t('common:messages.deleteSuccess'));
      fetchList();
    } catch {
      message.error(t('common:messages.deleteError'));
    }
  };

  const buildDRSpec = (values: CreateDRValues): Record<string, unknown> => ({
    apiVersion: 'networking.istio.io/v1beta1',
    kind: 'DestinationRule',
    metadata: {
      name: values.name,
      namespace: values.namespace,
    },
    spec: {
      host: values.host,
      trafficPolicy: {
        connectionPool: {
          tcp: { maxConnections: values.connectionPoolMaxConnections },
          http: { http1MaxPendingRequests: values.connectionPoolHttp1MaxPending },
        },
        outlierDetection: {
          consecutiveErrors: values.outlierConsecutiveErrors,
          interval: values.outlierInterval,
          baseEjectionTime: values.outlierBaseEjectionTime,
        },
      },
    },
  });

  const handleCreate = async () => {
    let values: CreateDRValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      const spec = buildDRSpec(values);
      await MeshService.createDestinationRule(clusterId, values.namespace, spec);
      message.success(t('common:messages.saveSuccess'));
      setCreateOpen(false);
      form.resetFields();
      fetchList();
    } catch {
      message.error(t('common:messages.saveError'));
    } finally {
      setSaving(false);
    }
  };

  const getSubsetsCount = (record: DestinationRuleSummary): number => {
    const spec = record.spec as { subsets?: unknown[] } | undefined;
    return spec?.subsets?.length ?? 0;
  };

  const getHost = (record: DestinationRuleSummary): string => {
    const spec = record.spec as { host?: string } | undefined;
    return spec?.host ?? '—';
  };

  const columns = [
    {
      title: t('network:gatewayapi.columns.name'),
      dataIndex: 'name',
      key: 'name',
      render: (v: string) => <strong>{v}</strong>,
    },
    {
      title: t('network:gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: 'Host',
      key: 'host',
      render: (_: unknown, record: DestinationRuleSummary) => getHost(record),
    },
    {
      title: 'Subsets',
      key: 'subsets',
      render: (_: unknown, record: DestinationRuleSummary) => {
        const count = getSubsetsCount(record);
        return count > 0 ? <Tag>{count}</Tag> : <span style={{ color: '#999' }}>—</span>;
      },
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => (v ? new Date(v).toLocaleString() : '—'),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 70,
      render: (_: unknown, record: DestinationRuleSummary) => (
        <ActionButtons
          primary={[
            { key: 'yaml', label: t('common:actions.view') + ' YAML', icon: <CodeOutlined />, onClick: () => handleViewYAML(record) },
          ]}
          more={[
            ...(canDelete() ? [{
              key: 'delete', label: t('common:actions.delete'), icon: <DeleteOutlined />, danger: true as const,
              onClick: () => handleDelete(record),
              confirm: {
                title: t('common:messages.confirmDelete'),
                description: t('network:servicemesh.confirmDeleteDR', { name: record.name }),
              },
            }] : []),
          ]}
        />
      ),
    },
  ];

  return (
    <div>
      {/* Toolbar */}
      <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
        <Select
          allowClear
          placeholder={t('network:gatewayapi.columns.namespace')}
          style={{ width: 200 }}
          value={namespace || undefined}
          onChange={v => setNamespace(v ?? '')}
          options={namespaces.map(ns => ({ value: ns, label: ns }))}
        />
        <Button icon={<ReloadOutlined />} onClick={fetchList} loading={loading} />
        <div style={{ flex: 1 }} />
        {canWrite() && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {t('network:servicemesh.createDestinationRule')}
          </Button>
        )}
      </div>

      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        dataSource={items}
        columns={columns}
        loading={loading}
        size="middle"
        pagination={{ pageSize: 20 }}
        scroll={{ x: 'max-content' }}
      />

      {/* YAML Viewer */}
      <Modal
        title="DestinationRule YAML"
        open={yamlOpen}
        onCancel={() => setYamlOpen(false)}
        footer={null}
        width={700}
      >
        <MonacoEditor
          height={400}
          language="json"
          value={yamlContent}
          options={{ readOnly: true, minimap: { enabled: false }, fontSize: 12 }}
        />
      </Modal>

      {/* Create Modal */}
      <Modal
        title="建立 DestinationRule（熔斷器設定）"
        open={createOpen}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        onOk={handleCreate}
        okText="建立"
        cancelText="取消"
        confirmLoading={saving}
        width={560}
      >
        <Form form={form} layout="vertical" initialValues={{
          connectionPoolMaxConnections: 100,
          connectionPoolHttp1MaxPending: 50,
          outlierConsecutiveErrors: 5,
          outlierInterval: '30s',
          outlierBaseEjectionTime: '30s',
        }}>
          <Form.Item name="name" label="名稱" rules={[{ required: true, message: '請輸入名稱' }]}>
            <Input placeholder="my-destination-rule" />
          </Form.Item>
          <Form.Item name="namespace" label="命名空間" rules={[{ required: true }]}>
            <Select options={namespaces.map(ns => ({ value: ns, label: ns }))} />
          </Form.Item>
          <Form.Item name="host" label="Host" rules={[{ required: true, message: '請輸入 Host' }]}>
            <Input placeholder="my-service" />
          </Form.Item>

          <div style={{ fontWeight: 500, marginBottom: 8 }}>連線池設定</div>
          <Space>
            <Form.Item name="connectionPoolMaxConnections" label="最大連線數">
              <InputNumber min={1} />
            </Form.Item>
            <Form.Item name="connectionPoolHttp1MaxPending" label="HTTP1 最大等待">
              <InputNumber min={1} />
            </Form.Item>
          </Space>

          <div style={{ fontWeight: 500, marginBottom: 8 }}>異常檢測（熔斷器）</div>
          <Space wrap>
            <Form.Item name="outlierConsecutiveErrors" label="連續錯誤數">
              <InputNumber min={1} />
            </Form.Item>
            <Form.Item name="outlierInterval" label="檢測間隔">
              <Input placeholder="30s" style={{ width: 100 }} />
            </Form.Item>
            <Form.Item name="outlierBaseEjectionTime" label="驅逐時間">
              <Input placeholder="30s" style={{ width: 100 }} />
            </Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  );
};

export default DestinationRuleList;
