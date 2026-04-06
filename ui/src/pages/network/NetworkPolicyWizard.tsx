import React, { useState } from 'react';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Steps,
  Tag,
  Typography,
  App,
} from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { NetworkPolicyService, type WizardIngressRule, type WizardEgressRule, type WizardPort } from '../../services/networkPolicyService';

const { Text, Title } = Typography;
const { TextArea } = Input;

interface Props {
  clusterId: string;
  namespaces: string[];
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

interface RuleForm {
  id: number;
  type: 'pod' | 'namespace' | 'ipblock' | 'all';
  selector: { key: string; value: string }[];
  cidr: string;
  ports: { protocol: string; port: string }[];
}

const newRuleForm = (id: number): RuleForm => ({
  id,
  type: 'all',
  selector: [],
  cidr: '',
  ports: [],
});

const ruleFormToIngress = (r: RuleForm): WizardIngressRule => ({
  fromType: r.type,
  selector: r.selector.reduce((acc, { key, value }) => ({ ...acc, [key]: value }), {} as Record<string, string>),
  cidr: r.cidr,
  ports: r.ports.map(p => ({ protocol: p.protocol, port: p.port })) as WizardPort[],
});

const ruleFormToEgress = (r: RuleForm): WizardEgressRule => ({
  toType: r.type,
  selector: r.selector.reduce((acc, { key, value }) => ({ ...acc, [key]: value }), {} as Record<string, string>),
  cidr: r.cidr,
  ports: r.ports.map(p => ({ protocol: p.protocol, port: p.port })) as WizardPort[],
});

const NetworkPolicyWizard: React.FC<Props> = ({ clusterId, namespaces, open, onClose, onCreated }) => {
  const { message } = App.useApp();
  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [previewYAML, setPreviewYAML] = useState('');

  // Step 1 state
  const [namespace, setNamespace] = useState('');
  const [name, setName] = useState('');
  const [selectorPairs, setSelectorPairs] = useState<{ key: string; value: string }[]>([]);

  // Step 2 state
  const [policyTypes, setPolicyTypes] = useState<string[]>(['Ingress']);
  const [ingressRules, setIngressRules] = useState<RuleForm[]>([newRuleForm(1)]);
  const [egressRules, setEgressRules] = useState<RuleForm[]>([newRuleForm(1)]);
  let ruleCounter = 100;

  const selectorMap = selectorPairs.reduce((acc, { key, value }) => key ? { ...acc, [key]: value } : acc, {} as Record<string, string>);

  const buildRequest = (s: number) => ({
    step: s,
    namespace,
    name,
    selector: selectorMap,
    policyTypes,
    ingress: policyTypes.includes('Ingress') ? ingressRules.map(ruleFormToIngress) : [],
    egress: policyTypes.includes('Egress') ? egressRules.map(ruleFormToEgress) : [],
  });

  const validate = async (targetStep: number): Promise<boolean> => {
    setValidating(true);
    setValidationError(null);
    try {
      const res = await NetworkPolicyService.wizardValidate(clusterId, buildRequest(targetStep));
      const data = (res as { data?: { valid: boolean; message?: string; yaml?: string } }).data;
      if (!data?.valid) {
        setValidationError(data?.message ?? '驗證失敗');
        return false;
      }
      if (data.yaml) setPreviewYAML(data.yaml);
      return true;
    } catch (e) {
      setValidationError(String(e));
      return false;
    } finally {
      setValidating(false);
    }
  };

  const handleNext = async () => {
    const ok = await validate(step + 1);
    if (ok) { setValidationError(null); setStep(s => s + 1); }
  };

  const handleSubmit = async () => {
    if (!previewYAML) return;
    setLoading(true);
    try {
      await NetworkPolicyService.create(clusterId, namespace, previewYAML);
      message.success('NetworkPolicy 建立成功');
      onCreated();
      handleClose();
    } catch (e) {
      message.error('建立失敗：' + String(e));
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setStep(0);
    setNamespace(''); setName(''); setSelectorPairs([]);
    setPolicyTypes(['Ingress']);
    setIngressRules([newRuleForm(1)]); setEgressRules([newRuleForm(1)]);
    setPreviewYAML(''); setValidationError(null);
    onClose();
  };

  // ---- Rule editor ----
  const RuleEditor: React.FC<{ rules: RuleForm[]; onChange: (r: RuleForm[]) => void; direction: 'ingress' | 'egress' }> =
    ({ rules, onChange, direction }) => {
    const updateRule = (id: number, patch: Partial<RuleForm>) =>
      onChange(rules.map(r => r.id === id ? { ...r, ...patch } : r));

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {rules.map((rule, idx) => (
          <div key={rule.id} style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: 12 }}>
            <Row align="middle" justify="space-between" style={{ marginBottom: 8 }}>
              <Text strong>規則 {idx + 1}</Text>
              {rules.length > 1 && (
                <Button size="small" danger icon={<DeleteOutlined />}
                  onClick={() => onChange(rules.filter(r => r.id !== rule.id))} />
              )}
            </Row>

            <Row gutter={12} style={{ marginBottom: 8 }}>
              <Col span={12}>
                <Text style={{ fontSize: 12 }}>{direction === 'ingress' ? '來源型別' : '目標型別'}</Text>
                <Select
                  value={rule.type}
                  onChange={v => updateRule(rule.id, { type: v })}
                  style={{ width: '100%', marginTop: 4 }}
                  options={[
                    { value: 'all', label: '允許所有' },
                    { value: 'pod', label: 'Pod 選擇器' },
                    { value: 'namespace', label: 'Namespace 選擇器' },
                    { value: 'ipblock', label: 'IP Block (CIDR)' },
                  ]}
                />
              </Col>
              {rule.type === 'ipblock' && (
                <Col span={12}>
                  <Text style={{ fontSize: 12 }}>CIDR</Text>
                  <Input value={rule.cidr} onChange={e => updateRule(rule.id, { cidr: e.target.value })}
                    placeholder="例如: 10.0.0.0/8" style={{ marginTop: 4 }} />
                </Col>
              )}
            </Row>

            {(rule.type === 'pod' || rule.type === 'namespace') && (
              <div style={{ marginBottom: 8 }}>
                <Text style={{ fontSize: 12 }}>標籤選擇器</Text>
                {rule.selector.map((pair, pi) => (
                  <Row key={pi} gutter={8} style={{ marginTop: 4 }}>
                    <Col span={10}><Input placeholder="key" value={pair.key}
                      onChange={e => updateRule(rule.id, { selector: rule.selector.map((p, i) => i === pi ? { ...p, key: e.target.value } : p) })} /></Col>
                    <Col span={10}><Input placeholder="value" value={pair.value}
                      onChange={e => updateRule(rule.id, { selector: rule.selector.map((p, i) => i === pi ? { ...p, value: e.target.value } : p) })} /></Col>
                    <Col span={4}><Button danger size="small" icon={<DeleteOutlined />}
                      onClick={() => updateRule(rule.id, { selector: rule.selector.filter((_, i) => i !== pi) })} /></Col>
                  </Row>
                ))}
                <Button size="small" icon={<PlusOutlined />} style={{ marginTop: 6 }}
                  onClick={() => updateRule(rule.id, { selector: [...rule.selector, { key: '', value: '' }] })}>
                  新增標籤
                </Button>
              </div>
            )}

            <div>
              <Text style={{ fontSize: 12 }}>連線連接埠（留空 = 所有）</Text>
              {rule.ports.map((p, pi) => (
                <Row key={pi} gutter={8} style={{ marginTop: 4 }}>
                  <Col span={8}>
                    <Select value={p.protocol}
                      onChange={v => updateRule(rule.id, { ports: rule.ports.map((pp, i) => i === pi ? { ...pp, protocol: v } : pp) })}
                      style={{ width: '100%' }}
                      options={[{ value: 'TCP', label: 'TCP' }, { value: 'UDP', label: 'UDP' }, { value: 'SCTP', label: 'SCTP' }]} />
                  </Col>
                  <Col span={12}>
                    <Input placeholder="連線連接埠號" value={p.port}
                      onChange={e => updateRule(rule.id, { ports: rule.ports.map((pp, i) => i === pi ? { ...pp, port: e.target.value } : pp) })} />
                  </Col>
                  <Col span={4}><Button danger size="small" icon={<DeleteOutlined />}
                    onClick={() => updateRule(rule.id, { ports: rule.ports.filter((_, i) => i !== pi) })} /></Col>
                </Row>
              ))}
              <Button size="small" icon={<PlusOutlined />} style={{ marginTop: 6 }}
                onClick={() => updateRule(rule.id, { ports: [...rule.ports, { protocol: 'TCP', port: '' }] })}>
                新增連線連接埠
              </Button>
            </div>
          </div>
        ))}
        <Button icon={<PlusOutlined />} onClick={() => { ruleCounter++; onChange([...rules, newRuleForm(ruleCounter)]); }}>
          新增規則
        </Button>
      </div>
    );
  };

  // ---- Steps content ----
  const stepContent = [
    // Step 1: Basic info + PodSelector
    <div key="s1" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={5} style={{ margin: 0 }}>基本設定</Title>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label="命名空間" required style={{ marginBottom: 8 }}>
            <Select value={namespace} onChange={setNamespace} placeholder="選擇命名空間"
              options={namespaces.map(n => ({ value: n, label: n }))} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label="Policy 名稱" required style={{ marginBottom: 8 }}>
            <Input value={name} onChange={e => setName(e.target.value)} placeholder="例如: deny-all-ingress" />
          </Form.Item>
        </Col>
      </Row>
      <div>
        <Text strong>Pod 選擇器</Text>
        <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>（留空 = 套用至命名空間所有 Pod）</Text>
        {selectorPairs.map((pair, i) => (
          <Row key={i} gutter={8} style={{ marginTop: 6 }}>
            <Col span={10}><Input placeholder="key" value={pair.key}
              onChange={e => setSelectorPairs(selectorPairs.map((p, j) => j === i ? { ...p, key: e.target.value } : p))} /></Col>
            <Col span={10}><Input placeholder="value" value={pair.value}
              onChange={e => setSelectorPairs(selectorPairs.map((p, j) => j === i ? { ...p, value: e.target.value } : p))} /></Col>
            <Col span={4}><Button danger size="small" icon={<DeleteOutlined />}
              onClick={() => setSelectorPairs(selectorPairs.filter((_, j) => j !== i))} /></Col>
          </Row>
        ))}
        <Button size="small" icon={<PlusOutlined />} style={{ marginTop: 8 }}
          onClick={() => setSelectorPairs([...selectorPairs, { key: '', value: '' }])}>
          新增標籤
        </Button>
        {Object.keys(selectorMap).length > 0 && (
          <div style={{ marginTop: 8 }}>
            {Object.entries(selectorMap).map(([k, v]) => (
              <Tag key={k} color="blue">{k}={v}</Tag>
            ))}
          </div>
        )}
      </div>
    </div>,

    // Step 2: Policy types + rules
    <div key="s2" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={5} style={{ margin: 0 }}>流量規則</Title>
      <Form.Item label="策略型別" required style={{ marginBottom: 0 }}>
        <Select mode="multiple" value={policyTypes} onChange={setPolicyTypes}
          options={[{ value: 'Ingress', label: 'Ingress（入站）' }, { value: 'Egress', label: 'Egress（出站）' }]} />
      </Form.Item>

      {policyTypes.includes('Ingress') && (
        <div>
          <Text strong style={{ marginBottom: 8, display: 'block' }}>Ingress 規則（允許入站）</Text>
          <RuleEditor rules={ingressRules} onChange={setIngressRules} direction="ingress" />
        </div>
      )}
      {policyTypes.includes('Egress') && (
        <div>
          <Text strong style={{ marginBottom: 8, display: 'block' }}>Egress 規則（允許出站）</Text>
          <RuleEditor rules={egressRules} onChange={setEgressRules} direction="egress" />
        </div>
      )}
    </div>,

    // Step 3: Preview YAML
    <div key="s3" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Title level={5} style={{ margin: 0 }}>預覽 YAML</Title>
      <Alert type="info" showIcon
        message={`即將建立 NetworkPolicy「${name}」至命名空間「${namespace}」`} />
      <TextArea
        value={previewYAML}
        rows={18}
        readOnly
        style={{ fontFamily: 'monospace', fontSize: 12, background: '#f8fafc' }}
      />
    </div>,
  ];

  return (
    <Modal
      open={open}
      title="建立 NetworkPolicy 精靈"
      onCancel={handleClose}
      width={760}
      footer={null}
      destroyOnClose
    >
      <Steps current={step} style={{ marginBottom: 24 }} items={[
        { title: '基本設定' },
        { title: '流量規則' },
        { title: '確認建立' },
      ]} />

      <div style={{ minHeight: 320 }}>
        {stepContent[step]}
      </div>

      {validationError && (
        <Alert type="error" showIcon message={validationError} style={{ marginTop: 12 }} />
      )}

      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 20 }}>
        <Button onClick={handleClose}>取消</Button>
        {step > 0 && <Button onClick={() => { setStep(s => s - 1); setValidationError(null); }}>上一步</Button>}
        {step < 2 && (
          <Button type="primary" onClick={handleNext} loading={validating}>
            下一步
          </Button>
        )}
        {step === 2 && (
          <Button type="primary" onClick={handleSubmit} loading={loading}>
            確認建立
          </Button>
        )}
      </div>
    </Modal>
  );
};

export default NetworkPolicyWizard;
