import React, { useState, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Select,
  Button,
  Space,
  Tabs,
  Divider,
  Row,
  Col,
  Alert,
  Tooltip,
  App,
} from 'antd';
import { PlusOutlined, DeleteOutlined, ExclamationCircleOutlined, CheckCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { getNamespaces } from '../../services/configService';
import { NetworkPolicyService } from '../../services/networkPolicyService';
import { parseApiError } from '@/utils/api';
import { RuleBuilder, newRule } from '../../components/RuleBuilder';
import type { RuleState, LabelPair, RulePort } from '../../components/RuleBuilder';

// ─── Types ─────────────────────────────────────────────────────────────────

interface FormValues {
  name: string;
  namespace: string;
  podSelector: LabelPair[];
  policyTypes: string[];
}

interface NetworkPolicyFormProps {
  open: boolean;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

// ─── Helpers ───────────────────────────────────────────────────────────────

const labelsToMap = (pairs: LabelPair[]) =>
  pairs.reduce<Record<string, string>>((acc, { key, value }) => {
    if (key) acc[key] = value;
    return acc;
  }, {});

const mapToLabels = (obj: Record<string, string> = {}): LabelPair[] =>
  Object.entries(obj).map(([key, value]) => ({ key, value }));

const buildPeer = (rule: RuleState): Record<string, unknown> => {
  if (rule.type === 'pod') return { podSelector: { matchLabels: labelsToMap(rule.selector) } };
  if (rule.type === 'namespace') return { namespaceSelector: { matchLabels: labelsToMap(rule.selector) } };
  if (rule.type === 'ipblock') return { ipBlock: { cidr: rule.cidr } };
  return {};
};

const ruleToIngress = (rule: RuleState): Record<string, unknown> => {
  const obj: Record<string, unknown> = {};
  if (rule.type !== 'all') obj.from = [buildPeer(rule)];
  const ports = rule.ports.filter(p => p.port).map(p => ({
    protocol: p.protocol,
    port: /^\d+$/.test(p.port) ? parseInt(p.port) : p.port,
  }));
  if (ports.length > 0) obj.ports = ports;
  return obj;
};

const ruleToEgress = (rule: RuleState): Record<string, unknown> => {
  const obj: Record<string, unknown> = {};
  if (rule.type !== 'all') obj.to = [buildPeer(rule)];
  const ports = rule.ports.filter(p => p.port).map(p => ({
    protocol: p.protocol,
    port: /^\d+$/.test(p.port) ? parseInt(p.port) : p.port,
  }));
  if (ports.length > 0) obj.ports = ports;
  return obj;
};

const formToYaml = (
  values: FormValues,
  ingressRules: RuleState[],
  egressRules: RuleState[],
): string => {
  const selectorLabels = labelsToMap(values.podSelector ?? []);
  const spec: Record<string, unknown> = {
    podSelector: Object.keys(selectorLabels).length > 0 ? { matchLabels: selectorLabels } : {},
    policyTypes: values.policyTypes ?? ['Ingress'],
  };
  if ((values.policyTypes ?? []).includes('Ingress')) {
    spec.ingress = ingressRules.map(ruleToIngress);
  }
  if ((values.policyTypes ?? []).includes('Egress')) {
    spec.egress = egressRules.map(ruleToEgress);
  }
  return YAML.stringify({
    apiVersion: 'networking.k8s.io/v1',
    kind: 'NetworkPolicy',
    metadata: { name: values.name || '', namespace: values.namespace || '' },
    spec,
  });
};

const parsePeer = (peer: Record<string, unknown>): RuleState => {
  const rule = newRule();
  if (peer.podSelector) {
    rule.type = 'pod';
    rule.selector = mapToLabels(((peer.podSelector as Record<string, unknown>).matchLabels ?? {}) as Record<string, string>);
  } else if (peer.namespaceSelector) {
    rule.type = 'namespace';
    rule.selector = mapToLabels(((peer.namespaceSelector as Record<string, unknown>).matchLabels ?? {}) as Record<string, string>);
  } else if (peer.ipBlock) {
    rule.type = 'ipblock';
    rule.cidr = (peer.ipBlock as Record<string, string>).cidr ?? '';
  } else {
    rule.type = 'all';
  }
  return rule;
};

const parseK8sPorts = (raw: Record<string, unknown>[]): RulePort[] =>
  raw.map(p => ({ protocol: String(p.protocol ?? 'TCP'), port: String(p.port ?? '') }));

const yamlToState = (yamlStr: string): {
  values: Partial<FormValues>;
  ingressRules: RuleState[];
  egressRules: RuleState[];
} => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = (obj.metadata ?? {}) as Record<string, string>;
  const spec = (obj.spec ?? {}) as Record<string, unknown>;
  const podSel = (spec.podSelector ?? {}) as Record<string, unknown>;
  const matchLabels = (podSel.matchLabels ?? {}) as Record<string, string>;

  const values: Partial<FormValues> = {
    name: meta.name ?? '',
    namespace: meta.namespace ?? 'default',
    podSelector: mapToLabels(matchLabels),
    policyTypes: (spec.policyTypes as string[]) ?? ['Ingress'],
  };

  const parseRules = (rawRules: Record<string, unknown>[], dir: 'from' | 'to'): RuleState[] => {
    if (!rawRules || rawRules.length === 0) return [newRule()];
    return rawRules.map(ruleObj => {
      const peers = (ruleObj[dir] as Record<string, unknown>[] | undefined) ?? [];
      const rule = peers.length > 0 ? parsePeer(peers[0]) : newRule();
      rule.ports = parseK8sPorts((ruleObj.ports as Record<string, unknown>[] | undefined) ?? []);
      return rule;
    });
  };

  return {
    values,
    ingressRules: parseRules((spec.ingress as Record<string, unknown>[]) ?? [], 'from'),
    egressRules: parseRules((spec.egress as Record<string, unknown>[]) ?? [], 'to'),
  };
};

// ─── Main Component ────────────────────────────────────────────────────────

const NetworkPolicyForm: React.FC<NetworkPolicyFormProps> = ({
  open,
  clusterId,
  onClose,
  onSuccess,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [form] = Form.useForm<FormValues>();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [ingressRules, setIngressRules] = useState<RuleState[]>([newRule()]);
  const [egressRules, setEgressRules] = useState<RuleState[]>([newRule()]);

  const policyTypes: string[] = Form.useWatch('policyTypes', form) ?? ['Ingress'];

  useEffect(() => {
    if (!open) return;
    getNamespaces(Number(clusterId)).then(setNamespaces).catch(() => {});
    form.resetFields();
    form.setFieldsValue({ policyTypes: ['Ingress'], podSelector: [] });
    setIngressRules([newRule()]);
    setEgressRules([newRule()]);
    setYamlContent('');
    setActiveTab('form');
    setDryRunResult(null);
  }, [open, clusterId, form]);

  const handleTabChange = (key: string) => {
    if (key === 'yaml' && activeTab === 'form') {
      try {
        const vals = form.getFieldsValue() as FormValues;
        setYamlContent(formToYaml(vals, ingressRules, egressRules));
      } catch { /* leave existing */ }
    } else if (key === 'form' && activeTab === 'yaml') {
      try {
        const { values, ingressRules: ir, egressRules: er } = yamlToState(yamlContent);
        form.setFieldsValue(values as FormValues);
        setIngressRules(ir);
        setEgressRules(er);
      } catch {
        message.error(t('networkpolicy.messages.yamlParseError'));
      }
    }
    setActiveTab(key);
  };

  const getCurrentYaml = (): string => {
    if (activeTab === 'form') {
      try {
        return formToYaml(form.getFieldsValue() as FormValues, ingressRules, egressRules);
      } catch { return yamlContent; }
    }
    return yamlContent;
  };

  const handleDryRun = async () => {
    const yaml = getCurrentYaml();
    try { YAML.parse(yaml); } catch (err) {
      setDryRunResult({ success: false, message: t('networkpolicy.messages.yamlParseError') + ': ' + String(err) });
      return;
    }
    setDryRunning(true);
    setDryRunResult(null);
    try {
      const parsed = YAML.parse(yaml) as Record<string, unknown>;
      const ns = ((parsed?.metadata as Record<string, string> | undefined)?.namespace) ?? 'default';
      await NetworkPolicyService.create(clusterId, ns, yaml, true);
      setDryRunResult({ success: true, message: t('networkpolicy.form.dryRunPassed') });
    } catch (err: unknown) {
      setDryRunResult({ success: false, message: parseApiError(err) || t('networkpolicy.form.dryRunFailed') });
    } finally {
      setDryRunning(false);
    }
  };

  const handleSubmit = async () => {
    setLoading(true);
    try {
      let yaml = yamlContent;
      let ns: string;
      if (activeTab === 'form') {
        const vals = await form.validateFields() as FormValues;
        yaml = formToYaml(vals, ingressRules, egressRules);
        ns = vals.namespace;
      } else {
        const parsed = YAML.parse(yaml) as Record<string, unknown>;
        ns = ((parsed?.metadata as Record<string, string> | undefined)?.namespace) ?? 'default';
      }
      await NetworkPolicyService.create(clusterId, ns, yaml);
      message.success(t('networkpolicy.messages.createSuccess'));
      onSuccess();
      onClose();
    } catch (err: unknown) {
      message.error(parseApiError(err) || t('networkpolicy.messages.createError'));
    } finally {
      setLoading(false);
    }
  };

  const nsOptions = namespaces.map(n => ({ label: n, value: n }));

  const footer = (
    <Space>
      <Tooltip title={t('networkpolicy.form.preCheckTooltip')}>
        <Button
          icon={dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleDryRun}
          loading={dryRunning}
        >
          {t('networkpolicy.form.preCheck')}
        </Button>
      </Tooltip>
      <Button onClick={onClose}>{t('networkpolicy.form.cancel')}</Button>
      <Button type="primary" loading={loading} onClick={handleSubmit}>
        {t('networkpolicy.form.createBtn')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={t('networkpolicy.createTitle')}
      open={open}
      onCancel={onClose}
      footer={footer}
      width={860}
      destroyOnHidden
    >
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('networkpolicy.form.dryRunCheckPassed') : t('networkpolicy.form.dryRunCheckFailed')}
          description={dryRunResult.message}
          type={dryRunResult.success ? 'success' : 'error'}
          showIcon
          closable
          onClose={() => setDryRunResult(null)}
          style={{ marginBottom: 12 }}
        />
      )}
      <Tabs
        activeKey={activeTab}
        onChange={handleTabChange}
        items={[
          {
            key: 'form',
            label: t('networkpolicy.form.formTab'),
            children: (
              <div style={{ maxHeight: 560, overflowY: 'auto', paddingRight: 4 }}>
                <Form
                  form={form}
                  layout="vertical"
                  initialValues={{ policyTypes: ['Ingress'], podSelector: [] }}
                >
                  <Space style={{ width: '100%' }} wrap>
                    <Form.Item
                      name="name"
                      label={t('networkpolicy.form.name')}
                      rules={[{ required: true, message: t('networkpolicy.form.nameRequired') }]}
                      style={{ marginBottom: 0, minWidth: 220 }}
                    >
                      <Input placeholder={t('networkpolicy.form.namePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="namespace"
                      label={t('networkpolicy.form.namespace')}
                      rules={[{ required: true, message: t('networkpolicy.form.namespaceRequired') }]}
                      style={{ marginBottom: 0, minWidth: 180 }}
                    >
                      <Select showSearch options={nsOptions} placeholder={t('networkpolicy.form.namespace')} />
                    </Form.Item>
                    <Form.Item
                      name="policyTypes"
                      label={t('networkpolicy.form.policyTypes')}
                      rules={[{ required: true, message: t('networkpolicy.form.policyTypesRequired') }]}
                      style={{ marginBottom: 0, minWidth: 240 }}
                    >
                      <Select
                        mode="multiple"
                        options={[
                          { value: 'Ingress', label: 'Ingress' },
                          { value: 'Egress', label: 'Egress' },
                        ]}
                      />
                    </Form.Item>
                  </Space>

                  {/* Pod Selector */}
                  <Divider orientation="left" plain style={{ marginTop: 16 }}>
                    {t('networkpolicy.form.podSelector')}
                  </Divider>
                  <Form.List name="podSelector">
                    {(fields, { add, remove }) => (
                      <>
                        {fields.map(field => (
                          <Row key={field.key} gutter={8} style={{ marginBottom: 4 }}>
                            <Col span={10}>
                              <Form.Item {...field} name={[field.name, 'key']} noStyle>
                                <Input placeholder={t('networkpolicy.form.keyPlaceholder')} />
                              </Form.Item>
                            </Col>
                            <Col span={10}>
                              <Form.Item {...field} name={[field.name, 'value']} noStyle>
                                <Input placeholder={t('networkpolicy.form.valuePlaceholder')} />
                              </Form.Item>
                            </Col>
                            <Col span={4}>
                              <Button
                                danger
                                size="small"
                                icon={<DeleteOutlined />}
                                onClick={() => remove(field.name)}
                              />
                            </Col>
                          </Row>
                        ))}
                        <Button
                          type="dashed"
                          size="small"
                          icon={<PlusOutlined />}
                          onClick={() => add({ key: '', value: '' })}
                        >
                          {t('networkpolicy.form.addLabel')}
                        </Button>
                      </>
                    )}
                  </Form.List>

                  {/* Ingress Rules */}
                  {policyTypes.includes('Ingress') && (
                    <>
                      <Divider orientation="left" plain style={{ marginTop: 16 }}>
                        {t('networkpolicy.form.ingressRules')}
                      </Divider>
                      <RuleBuilder
                        rules={ingressRules}
                        onChange={setIngressRules}
                        direction="ingress"
                      />
                    </>
                  )}

                  {/* Egress Rules */}
                  {policyTypes.includes('Egress') && (
                    <>
                      <Divider orientation="left" plain style={{ marginTop: 16 }}>
                        {t('networkpolicy.form.egressRules')}
                      </Divider>
                      <RuleBuilder
                        rules={egressRules}
                        onChange={setEgressRules}
                        direction="egress"
                      />
                    </>
                  )}
                </Form>
              </div>
            ),
          },
          {
            key: 'yaml',
            label: t('networkpolicy.form.yamlTab'),
            children: (
              <MonacoEditor
                height="480px"
                language="yaml"
                value={yamlContent}
                onChange={v => setYamlContent(v || '')}
                options={{ minimap: { enabled: false }, fontSize: 13, wordWrap: 'on', scrollBeyondLastLine: false }}
              />
            ),
          },
        ]}
      />
    </Modal>
  );
};

export default NetworkPolicyForm;
