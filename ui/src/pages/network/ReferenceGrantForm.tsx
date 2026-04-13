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
  Alert,
  Tooltip,
  App,
} from 'antd';
import { PlusOutlined, MinusCircleOutlined, ExclamationCircleOutlined, CheckCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { getNamespaces } from '../../services/configService';
import { gatewayService } from '../../services/gatewayService';
import { parseApiError, showApiError } from '@/utils/api';

const DEFAULT_YAML = `apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-httproute
  namespace: backend
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: frontend
  to:
    - group: ""
      kind: Service
`;

interface ReferenceGrantFormValues {
  name: string;
  namespace: string;
  from: { group: string; kind: string; namespace: string }[];
  to: { group: string; kind: string; name?: string }[];
}

const FROM_KIND_OPTIONS = ['HTTPRoute', 'GRPCRoute', 'TCPRoute', 'TLSRoute', 'UDPRoute'];
const TO_KIND_OPTIONS = ['Service', 'Secret', 'ConfigMap'];
const GW_GROUP = 'gateway.networking.k8s.io';
const CORE_GROUP = '';

const formToYaml = (values: ReferenceGrantFormValues): string => {
  const obj = {
    apiVersion: 'gateway.networking.k8s.io/v1beta1',
    kind: 'ReferenceGrant',
    metadata: { name: values.name, namespace: values.namespace },
    spec: {
      from: (values.from || []).map((f) => ({
        group: f.group ?? GW_GROUP,
        kind: f.kind,
        namespace: f.namespace,
      })),
      to: (values.to || []).map((t) => ({
        group: t.group ?? CORE_GROUP,
        kind: t.kind,
        ...(t.name ? { name: t.name } : {}),
      })),
    },
  };
  return YAML.stringify(obj);
};

const yamlToForm = (yamlStr: string): Partial<ReferenceGrantFormValues> => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = obj.metadata as Record<string, string> | undefined;
  const spec = obj.spec as Record<string, unknown> | undefined;

  const from = ((spec?.from as Record<string, string>[]) ?? []).map((f) => ({
    group: f.group ?? GW_GROUP,
    kind: f.kind ?? '',
    namespace: f.namespace ?? '',
  }));

  const to = ((spec?.to as Record<string, string>[]) ?? []).map((t) => ({
    group: t.group ?? CORE_GROUP,
    kind: t.kind ?? '',
    name: t.name ?? '',
  }));

  return {
    name: meta?.name ?? '',
    namespace: meta?.namespace ?? 'default',
    from,
    to,
  };
};

interface ReferenceGrantFormProps {
  open: boolean;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

const ReferenceGrantForm: React.FC<ReferenceGrantFormProps> = ({
  open,
  clusterId,
  onClose,
  onSuccess,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [form] = Form.useForm<ReferenceGrantFormValues>();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);

  useEffect(() => {
    if (!open) return;
    getNamespaces(Number(clusterId)).then(setNamespaces).catch(() => {});
    form.resetFields();
    setYamlContent(DEFAULT_YAML);
    setActiveTab('form');
    setDryRunResult(null);
  }, [open, clusterId, form]);

  const handleTabChange = (key: string) => {
    if (key === 'yaml' && activeTab === 'form') {
      try { setYamlContent(formToYaml(form.getFieldsValue())); } catch { /* leave existing */ }
    } else if (key === 'form' && activeTab === 'yaml') {
      try { form.setFieldsValue(yamlToForm(yamlContent)); }
      catch { message.error(t('gatewayapi.messages.yamlParseError')); }
    }
    setActiveTab(key);
  };

  const getCurrentYaml = (): string => {
    if (activeTab === 'form') {
      try { return formToYaml(form.getFieldsValue()); } catch { return yamlContent; }
    }
    return yamlContent;
  };

  const handleDryRun = async () => {
    const yaml = getCurrentYaml();
    try { YAML.parse(yaml); } catch (err) {
      setDryRunResult({ success: false, message: t('gatewayapi.messages.yamlParseError') + ': ' + String(err) });
      return;
    }
    setDryRunning(true);
    setDryRunResult(null);
    try {
      const ns = ((YAML.parse(yaml) as Record<string, unknown>)?.metadata as Record<string, string> | undefined)?.namespace ?? 'default';
      await gatewayService.createReferenceGrant(clusterId, ns, yaml, true);
      setDryRunResult({ success: true, message: t('gatewayapi.form.dryRunPassed') });
    } catch (err: unknown) {
      setDryRunResult({ success: false, message: parseApiError(err) || t('gatewayapi.form.dryRunFailed') });
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
        const vals = await form.validateFields();
        yaml = formToYaml(vals);
        ns = vals.namespace;
      } else {
        const parsed = YAML.parse(yaml) as Record<string, unknown>;
        ns = ((parsed?.metadata as Record<string, string> | undefined)?.['namespace']) ?? 'default';
      }
      await gatewayService.createReferenceGrant(clusterId, ns, yaml);
      message.success(t('gatewayapi.messages.createReferenceGrantSuccess'));
      onSuccess();
      onClose();
    } catch (err: unknown) {
      showApiError(err, t('gatewayapi.messages.createReferenceGrantError'));
    } finally {
      setLoading(false);
    }
  };

  const nsOptions = namespaces.map((n) => ({ label: n, value: n }));

  const footer = (
    <Space>
      <Tooltip title={t('gatewayapi.form.preCheckTooltip')}>
        <Button
          icon={dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleDryRun}
          loading={dryRunning}
        >
          {t('gatewayapi.form.preCheck')}
        </Button>
      </Tooltip>
      <Button onClick={onClose}>{t('gatewayapi.form.cancel')}</Button>
      <Button type="primary" loading={loading} onClick={handleSubmit}>
        {t('gatewayapi.form.createBtn')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={t('gatewayapi.form.createReferenceGrant')}
      open={open}
      onCancel={onClose}
      footer={footer}
      width={860}
      destroyOnClose
    >
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('gatewayapi.form.dryRunCheckPassed') : t('gatewayapi.form.dryRunCheckFailed')}
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
            label: t('gatewayapi.form.formTab'),
            children: (
              <div style={{ maxHeight: 540, overflowY: 'auto', paddingRight: 4 }}>
                <Form
                  form={form}
                  layout="vertical"
                  initialValues={{
                    namespace: 'default',
                    from: [{ group: GW_GROUP, kind: 'HTTPRoute', namespace: '' }],
                    to: [{ group: CORE_GROUP, kind: 'Service' }],
                  }}
                >
                  {/* Basic info */}
                  <Space style={{ width: '100%' }} wrap>
                    <Form.Item
                      name="name"
                      label={t('gatewayapi.form.name')}
                      rules={[{ required: true, message: t('gatewayapi.form.nameRequired') }]}
                      style={{ marginBottom: 0, minWidth: 200 }}
                    >
                      <Input placeholder={t('gatewayapi.form.namePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="namespace"
                      label={t('gatewayapi.form.namespace')}
                      tooltip="ReferenceGrant 所在的命名空間（即 to 資源所在的命名空間）"
                      rules={[{ required: true, message: t('gatewayapi.form.namespaceRequired') }]}
                      style={{ marginBottom: 0, minWidth: 180 }}
                    >
                      <Select showSearch options={nsOptions} />
                    </Form.Item>
                  </Space>

                  {/* From */}
                  <Divider orientation="left" plain style={{ marginTop: 16 }}>
                    {t('gatewayapi.form.refGrantFrom')}
                  </Divider>
                  <Form.List name="from">
                    {(fields, { add, remove }) => (
                      <>
                        {fields.map((field) => (
                          <Space key={field.key} wrap style={{ display: 'flex', marginBottom: 8 }} align="start">
                            <Form.Item
                              {...field}
                              name={[field.name, 'group']}
                              label={t('gatewayapi.form.refGrantGroup')}
                              style={{ marginBottom: 0 }}
                            >
                              <Select
                                style={{ width: 220 }}
                                options={[
                                  { label: 'gateway.networking.k8s.io', value: GW_GROUP },
                                ]}
                              />
                            </Form.Item>
                            <Form.Item
                              {...field}
                              name={[field.name, 'kind']}
                              label={t('gatewayapi.form.refGrantKind')}
                              rules={[{ required: true }]}
                              style={{ marginBottom: 0 }}
                            >
                              <Select
                                showSearch
                                style={{ width: 140 }}
                                options={FROM_KIND_OPTIONS.map((k) => ({ label: k, value: k }))}
                              />
                            </Form.Item>
                            <Form.Item
                              {...field}
                              name={[field.name, 'namespace']}
                              label={t('gatewayapi.form.refGrantNamespace')}
                              rules={[{ required: true }]}
                              style={{ marginBottom: 0 }}
                            >
                              <Select showSearch style={{ width: 160 }} options={nsOptions} />
                            </Form.Item>
                            <Form.Item label=" " style={{ marginBottom: 0 }}>
                              <MinusCircleOutlined
                                style={{ color: '#ff4d4f', marginTop: 8 }}
                                onClick={() => remove(field.name)}
                              />
                            </Form.Item>
                          </Space>
                        ))}
                        <Button
                          type="dashed"
                          icon={<PlusOutlined />}
                          onClick={() => add({ group: GW_GROUP, kind: 'HTTPRoute', namespace: '' })}
                        >
                          {t('gatewayapi.form.addFrom')}
                        </Button>
                      </>
                    )}
                  </Form.List>

                  {/* To */}
                  <Divider orientation="left" plain>
                    {t('gatewayapi.form.refGrantTo')}
                  </Divider>
                  <Form.List name="to">
                    {(fields, { add, remove }) => (
                      <>
                        {fields.map((field) => (
                          <Space key={field.key} wrap style={{ display: 'flex', marginBottom: 8 }} align="start">
                            <Form.Item
                              {...field}
                              name={[field.name, 'group']}
                              label={t('gatewayapi.form.refGrantGroup')}
                              style={{ marginBottom: 0 }}
                            >
                              <Select
                                style={{ width: 160 }}
                                options={[
                                  { label: 'core ("")', value: CORE_GROUP },
                                  { label: 'gateway.networking.k8s.io', value: GW_GROUP },
                                ]}
                              />
                            </Form.Item>
                            <Form.Item
                              {...field}
                              name={[field.name, 'kind']}
                              label={t('gatewayapi.form.refGrantKind')}
                              rules={[{ required: true }]}
                              style={{ marginBottom: 0 }}
                            >
                              <Select
                                showSearch
                                style={{ width: 140 }}
                                options={TO_KIND_OPTIONS.map((k) => ({ label: k, value: k }))}
                              />
                            </Form.Item>
                            <Form.Item
                              {...field}
                              name={[field.name, 'name']}
                              label={t('gatewayapi.form.refGrantName')}
                              style={{ marginBottom: 0 }}
                            >
                              <Input placeholder={t('gatewayapi.form.namePlaceholder')} style={{ width: 140 }} />
                            </Form.Item>
                            <Form.Item label=" " style={{ marginBottom: 0 }}>
                              <MinusCircleOutlined
                                style={{ color: '#ff4d4f', marginTop: 8 }}
                                onClick={() => remove(field.name)}
                              />
                            </Form.Item>
                          </Space>
                        ))}
                        <Button
                          type="dashed"
                          icon={<PlusOutlined />}
                          onClick={() => add({ group: CORE_GROUP, kind: 'Service', name: '' })}
                        >
                          {t('gatewayapi.form.addTo')}
                        </Button>
                      </>
                    )}
                  </Form.List>
                </Form>
              </div>
            ),
          },
          {
            key: 'yaml',
            label: t('gatewayapi.form.yamlTab'),
            children: (
              <MonacoEditor
                height="440px"
                language="yaml"
                value={yamlContent}
                onChange={(v) => setYamlContent(v || '')}
                options={{ minimap: { enabled: false }, fontSize: 13, wordWrap: 'on', scrollBeyondLastLine: false }}
              />
            ),
          },
        ]}
      />
    </Modal>
  );
};

export default ReferenceGrantForm;
