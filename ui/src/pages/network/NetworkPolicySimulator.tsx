import React, { useState } from 'react';
import { Form, Input, Select, Button, Alert, Tag, Space, Typography, Card, InputNumber } from 'antd';
import { SafetyCertificateOutlined, CheckCircleOutlined, StopOutlined } from '@ant-design/icons';
import { NetworkPolicyService, type SimulateResult } from '../../services/networkPolicyService';

const { Text } = Typography;

interface Props {
  clusterId: string;
  namespaces: string[];
}

function parseLabels(s: string): Record<string, string> {
  const result: Record<string, string> = {};
  if (!s.trim()) return result;
  s.split(',').forEach(pair => {
    const [k, v] = pair.split('=');
    if (k && v !== undefined) result[k.trim()] = v.trim();
  });
  return result;
}

const NetworkPolicySimulator: React.FC<Props> = ({ clusterId, namespaces }) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<SimulateResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSimulate = async (values: { namespace: string; fromLabels: string; toLabels: string; port: number; protocol: string }) => {
    setLoading(true);
    setResult(null);
    setError(null);
    try {
      const res = await NetworkPolicyService.simulate(clusterId, {
        namespace: values.namespace,
        fromPodLabels: parseLabels(values.fromLabels || ''),
        toPodLabels: parseLabels(values.toLabels || ''),
        port: values.port || 0,
        protocol: values.protocol || 'TCP',
      });
      setResult(res as SimulateResult);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  const nsOptions = namespaces.map(n => ({ value: n, label: n }));

  return (
    <div style={{ maxWidth: 640 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
        <SafetyCertificateOutlined style={{ fontSize: 18, color: '#3b82f6' }} />
        <Text strong style={{ fontSize: 15 }}>NetworkPolicy 策略模擬</Text>
      </div>
      <Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 16 }}>
        輸入來源與目標 Pod 的標籤，模擬流量是否被 NetworkPolicy 允許或拒絕。
      </Text>
      <Form form={form} layout="vertical" onFinish={handleSimulate}>
        <Form.Item name="namespace" label="命名空間" rules={[{ required: true, message: '請選擇命名空間' }]}>
          <Select options={nsOptions} placeholder="選擇命名空間" />
        </Form.Item>
        <Form.Item
          name="fromLabels"
          label="來源 Pod 標籤"
          tooltip="格式：key1=val1,key2=val2（留空表示任意來源）"
        >
          <Input placeholder="app=frontend,env=prod" />
        </Form.Item>
        <Form.Item
          name="toLabels"
          label="目標 Pod 標籤"
          rules={[{ required: true, message: '請輸入目標 Pod 標籤' }]}
          tooltip="格式：key1=val1,key2=val2"
        >
          <Input placeholder="app=backend,env=prod" />
        </Form.Item>
        <Space style={{ width: '100%' }} size={12}>
          <Form.Item name="port" label="連線連接埠" style={{ marginBottom: 0, flex: 1 }}>
            <InputNumber placeholder="80（0=任意）" min={0} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="protocol" label="協定" initialValue="TCP" style={{ marginBottom: 0, flex: 1 }}>
            <Select options={[{ value: 'TCP' }, { value: 'UDP' }, { value: 'SCTP' }]} />
          </Form.Item>
        </Space>
        <Form.Item style={{ marginTop: 16 }}>
          <Button type="primary" htmlType="submit" loading={loading} icon={<SafetyCertificateOutlined />}>
            執行模擬
          </Button>
        </Form.Item>
      </Form>

      {error && <Alert type="error" message={error} showIcon style={{ marginTop: 8 }} />}

      {result && (
        <Card size="small" style={{ marginTop: 12, borderColor: result.allowed ? '#22c55e' : '#ef4444' }}>
          <Alert
            type={result.allowed ? 'success' : 'error'}
            icon={result.allowed ? <CheckCircleOutlined /> : <StopOutlined />}
            message={result.allowed ? '流量允許 (ALLOW)' : '流量拒絕 (DENY)'}
            description={result.reason}
            showIcon
            style={{ marginBottom: result.matchedPolicies.length > 0 ? 12 : 0 }}
          />
          {result.matchedPolicies.length > 0 && (
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>匹配的 NetworkPolicy：</Text>
              <div style={{ marginTop: 4 }}>
                {result.matchedPolicies.map(p => <Tag key={p} color="blue">{p}</Tag>)}
              </div>
            </div>
          )}
        </Card>
      )}
    </div>
  );
};

export default NetworkPolicySimulator;
