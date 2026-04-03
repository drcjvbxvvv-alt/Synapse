import React, { useEffect, useState } from 'react';
import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  App,
} from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { MeshService } from '../../services/meshService';

interface TrafficSplit {
  key: string;
  subset: string;
  weight: number;
}

interface VirtualServiceFormValues {
  name: string;
  namespace: string;
  hosts: string; // comma-separated
  splits: TrafficSplit[];
}

interface Props {
  clusterId: string;
  namespaces: string[];
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

const VirtualServiceForm: React.FC<Props> = ({ clusterId, namespaces, open, onClose, onCreated }) => {
  const { message } = App.useApp();
  const [form] = Form.useForm<VirtualServiceFormValues>();
  const [splits, setSplits] = useState<TrafficSplit[]>([
    { key: '1', subset: 'v1', weight: 80 },
    { key: '2', subset: 'v2', weight: 20 },
  ]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) {
      form.resetFields();
      setSplits([
        { key: '1', subset: 'v1', weight: 80 },
        { key: '2', subset: 'v2', weight: 20 },
      ]);
    }
  }, [open, form]);

  const addSplit = () => {
    setSplits(prev => [
      ...prev,
      { key: String(Date.now()), subset: '', weight: 0 },
    ]);
  };

  const removeSplit = (key: string) => {
    setSplits(prev => prev.filter(s => s.key !== key));
  };

  const updateSplit = (key: string, field: 'subset' | 'weight', value: string | number) => {
    setSplits(prev =>
      prev.map(s => (s.key === key ? { ...s, [field]: value } : s))
    );
  };

  const buildVSSpec = (values: VirtualServiceFormValues): Record<string, unknown> => {
    const hosts = values.hosts.split(',').map(h => h.trim()).filter(Boolean);
    const route = splits.map(s => ({
      destination: {
        host: hosts[0] ?? values.name,
        subset: s.subset,
      },
      weight: s.weight,
    }));

    return {
      apiVersion: 'networking.istio.io/v1beta1',
      kind: 'VirtualService',
      metadata: {
        name: values.name,
        namespace: values.namespace,
      },
      spec: {
        hosts,
        http: [{ route }],
      },
    };
  };

  const handleOk = async () => {
    let values: VirtualServiceFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }

    const totalWeight = splits.reduce((acc, s) => acc + (s.weight ?? 0), 0);
    if (totalWeight !== 100) {
      message.error(`流量權重總和必須為 100（目前 ${totalWeight}）`);
      return;
    }

    setSaving(true);
    try {
      const spec = buildVSSpec(values);
      await MeshService.createVirtualService(clusterId, values.namespace, spec);
      message.success('VirtualService 建立成功');
      onCreated();
    } catch {
      message.error('建立 VirtualService 失敗');
    } finally {
      setSaving(false);
    }
  };

  const splitColumns = [
    {
      title: 'Subset 名稱',
      dataIndex: 'subset',
      render: (_: unknown, record: TrafficSplit) => (
        <Input
          value={record.subset}
          placeholder="v1"
          onChange={e => updateSplit(record.key, 'subset', e.target.value)}
        />
      ),
    },
    {
      title: '權重 (%)',
      dataIndex: 'weight',
      render: (_: unknown, record: TrafficSplit) => (
        <InputNumber
          min={0}
          max={100}
          value={record.weight}
          onChange={v => updateSplit(record.key, 'weight', v ?? 0)}
          style={{ width: 100 }}
        />
      ),
    },
    {
      title: '',
      render: (_: unknown, record: TrafficSplit) => (
        <Button
          size="small"
          danger
          icon={<DeleteOutlined />}
          onClick={() => removeSplit(record.key)}
          disabled={splits.length <= 1}
        />
      ),
    },
  ];

  return (
    <Modal
      title="建立 VirtualService（金絲雀流量分割）"
      open={open}
      onCancel={onClose}
      onOk={handleOk}
      okText="建立"
      cancelText="取消"
      confirmLoading={saving}
      width={600}
    >
      <Form form={form} layout="vertical">
        <Form.Item name="name" label="名稱" rules={[{ required: true, message: '請輸入名稱' }]}>
          <Input placeholder="my-virtual-service" />
        </Form.Item>
        <Form.Item name="namespace" label="命名空間" rules={[{ required: true, message: '請選擇命名空間' }]}>
          <Select placeholder="選擇命名空間" options={namespaces.map(ns => ({ value: ns, label: ns }))} />
        </Form.Item>
        <Form.Item
          name="hosts"
          label="Hosts（逗號分隔）"
          rules={[{ required: true, message: '請輸入 Hosts' }]}
        >
          <Input placeholder="my-service, my-service.default.svc.cluster.local" />
        </Form.Item>
      </Form>

      <div style={{ marginTop: 16 }}>
        <div style={{ fontWeight: 500, marginBottom: 8 }}>流量分割設定</div>
        <Table
          dataSource={splits}
          columns={splitColumns}
          rowKey="key"
          pagination={false}
          size="small"
        />
        <Space style={{ marginTop: 8 }}>
          <Button size="small" icon={<PlusOutlined />} onClick={addSplit}>
            新增 Subset
          </Button>
          <span style={{ color: '#999', fontSize: 12 }}>
            總權重：{splits.reduce((a, s) => a + (s.weight ?? 0), 0)} / 100
          </span>
        </Space>
      </div>
    </Modal>
  );
};

export default VirtualServiceForm;
