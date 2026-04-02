import React, { useState, useEffect, useCallback } from 'react';
import {
  App, Badge, Button, Card, Form, InputNumber, Popconfirm, Space, Table, Tag, Tooltip, Typography,
} from 'antd';
import {
  ApiOutlined, CopyOutlined, DeleteOutlined, PlusOutlined, ReloadOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { portforwardService, type PortForwardSession } from '../../services/portforwardService';

const { Text } = Typography;

interface PortForwardPanelProps {
  clusterId: string;
  namespace: string;
  podName: string;
  /** 若 undefined 則顯示全域列表模式（無「建立」功能） */
  embedded?: boolean;
}

const PortForwardPanel: React.FC<PortForwardPanelProps> = ({
  clusterId, namespace, podName, embedded = true,
}) => {
  const { message } = App.useApp();
  const [sessions, setSessions] = useState<PortForwardSession[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await portforwardService.list('active');
      const filtered = embedded
        ? res.data.items.filter(s => s.namespace === namespace && s.podName === podName)
        : res.data.items;
      setSessions(filtered || []);
    } catch {
      message.error('載入 Port-Forward 列表失敗');
    } finally {
      setLoading(false);
    }
  }, [namespace, podName, embedded, message]);

  useEffect(() => { load(); }, [load]);

  const handleStart = async () => {
    const values = await form.validateFields();
    setCreating(true);
    try {
      const res = await portforwardService.start(clusterId, namespace, podName, values.podPort);
      message.success(res.data.message);
      form.resetFields();
      load();
    } catch (e) {
      message.error('建立 Port-Forward 失敗：' + String(e));
    } finally {
      setCreating(false);
    }
  };

  const handleStop = async (sessionId: number) => {
    try {
      await portforwardService.stop(sessionId);
      message.success('Port-Forward 已停止');
      load();
    } catch {
      message.error('停止失敗');
    }
  };

  const copyURL = (localPort: number) => {
    const url = `http://localhost:${localPort}`;
    navigator.clipboard.writeText(url);
    message.success(`已複製：${url}`);
  };

  const columns: ColumnsType<PortForwardSession> = [
    ...(embedded ? [] : [
      { title: '叢集', dataIndex: 'clusterName', width: 120 },
      { title: 'Pod', dataIndex: 'podName', width: 150 },
    ] as ColumnsType<PortForwardSession>),
    {
      title: '命名空間',
      dataIndex: 'namespace',
      width: 140,
    },
    {
      title: '轉發規則',
      width: 180,
      render: (_: unknown, r: PortForwardSession) => (
        <Text code>{r.localPort} → Pod:{r.podPort}</Text>
      ),
    },
    {
      title: '狀態',
      dataIndex: 'status',
      width: 90,
      render: (s: string) =>
        s === 'active'
          ? <Badge status="success" text="運行中" />
          : <Badge status="default" text="已停止" />,
    },
    {
      title: '建立者',
      dataIndex: 'username',
      width: 100,
    },
    {
      title: '操作',
      width: 140,
      render: (_: unknown, r: PortForwardSession) => (
        <Space>
          <Tooltip title={`複製連結 http://localhost:${r.localPort}`}>
            <Button
              type="link" size="small" icon={<CopyOutlined />}
              onClick={() => copyURL(r.localPort)}
            />
          </Tooltip>
          {r.status === 'active' && (
            <Popconfirm title="確定停止此 Port-Forward？" onConfirm={() => handleStop(r.id)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>停止</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Card
      size="small"
      title={<Space><ApiOutlined /> Port-Forward</Space>}
      extra={<Button icon={<ReloadOutlined />} size="small" onClick={load} loading={loading} />}
    >
      {embedded && (
        <Form form={form} layout="inline" style={{ marginBottom: 12 }}>
          <Form.Item
            name="podPort"
            label="Pod 埠"
            rules={[{ required: true, message: '請輸入 Pod 埠號' }]}
          >
            <InputNumber min={1} max={65535} placeholder="例如 8080" style={{ width: 120 }} />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary" icon={<PlusOutlined />}
              onClick={handleStart} loading={creating}
            >
              建立轉發
            </Button>
          </Form.Item>
        </Form>
      )}

      <Table
        rowKey="id"
        columns={columns}
        dataSource={sessions}
        loading={loading}
        pagination={false}
        size="small"
        locale={{ emptyText: '目前無活躍的 Port-Forward' }}
      />

      <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 8 }}>
        <Tag color="blue">提示</Tag>
        Port-Forward 在後端伺服器執行，複製的連結需從後端伺服器所在主機存取。
      </Text>
    </Card>
  );
};

export default PortForwardPanel;
