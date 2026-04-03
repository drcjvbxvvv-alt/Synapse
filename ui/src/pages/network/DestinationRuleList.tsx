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
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import { MeshService, type DestinationRuleSummary } from '../../services/meshService';

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
  const { message, modal } = App.useApp();
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
      message.error('取得 DestinationRule 列表失敗');
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, message]);

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
      message.error('取得 DestinationRule 詳情失敗');
    }
  };

  const handleDelete = (record: DestinationRuleSummary) => {
    modal.confirm({
      title: '確認刪除',
      content: `確定要刪除 DestinationRule "${record.name}" 嗎？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await MeshService.deleteDestinationRule(clusterId, record.namespace, record.name);
          message.success('DestinationRule 刪除成功');
          fetchList();
        } catch {
          message.error('刪除失敗');
        }
      },
    });
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
      message.success('DestinationRule 建立成功');
      setCreateOpen(false);
      form.resetFields();
      fetchList();
    } catch {
      message.error('建立 DestinationRule 失敗');
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
      title: '名稱',
      dataIndex: 'name',
      render: (v: string) => <strong>{v}</strong>,
    },
    {
      title: '命名空間',
      dataIndex: 'namespace',
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
      title: '建立時間',
      dataIndex: 'createdAt',
      render: (v: string) => (v ? new Date(v).toLocaleString() : '—'),
    },
    {
      title: '操作',
      key: 'actions',
      fixed: 'right' as const,
      width: 120,
      render: (_: unknown, record: DestinationRuleSummary) => (
        <Space>
          <Tooltip title="檢視 YAML">
            <Button size="small" icon={<EyeOutlined />} onClick={() => handleViewYAML(record)} />
          </Tooltip>
          <Tooltip title="刪除">
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* Toolbar */}
      <Space style={{ marginBottom: 12 }} wrap>
        <Select
          allowClear
          placeholder="命名空間"
          style={{ width: 180 }}
          value={namespace || undefined}
          onChange={v => setNamespace(v ?? '')}
          options={namespaces.map(ns => ({ value: ns, label: ns }))}
        />
        <Button icon={<ReloadOutlined />} onClick={fetchList}>重新整理</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          建立 DestinationRule
        </Button>
      </Space>

      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        dataSource={items}
        columns={columns}
        loading={loading}
        size="middle"
        pagination={{ pageSize: 20 }}
        scroll={{ x: 800 }}
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
