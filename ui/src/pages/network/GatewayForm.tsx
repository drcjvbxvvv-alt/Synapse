import React, { useState, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Button,
  Space,
  Tabs,
  App,
} from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { getNamespaces } from '../../services/configService';
import { gatewayService } from '../../services/gatewayService';
import type {
  GatewayClassItem,
  GatewayItem,
  GatewayFormValues,
  GatewayListenerFormValue,
} from './gatewayTypes';

const PROTOCOLS = ['HTTP', 'HTTPS', 'TLS', 'TCP', 'UDP'];
const TLS_MODES = ['Terminate', 'Passthrough'];

const DEFAULT_YAML = `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  gatewayClassName: nginx
  listeners:
    - name: http
      port: 80
      protocol: HTTP
`;

interface GatewayFormProps {
  open: boolean;
  clusterId: string;
  editing?: GatewayItem | null;
  onClose: () => void;
  onSuccess: () => void;
}

const formToYaml = (values: GatewayFormValues): string => {
  const obj: Record<string, unknown> = {
    apiVersion: 'gateway.networking.k8s.io/v1',
    kind: 'Gateway',
    metadata: { name: values.name, namespace: values.namespace },
    spec: {
      gatewayClassName: values.gatewayClass,
      listeners: (values.listeners || []).map((l: GatewayListenerFormValue) => {
        const listener: Record<string, unknown> = {
          name: l.name,
          port: l.port,
          protocol: l.protocol,
        };
        if (l.hostname) listener.hostname = l.hostname;
        if (l.tlsMode) {
          listener.tls = {
            mode: l.tlsMode,
            ...(l.tlsSecretName
              ? {
                  certificateRefs: [
                    {
                      name: l.tlsSecretName,
                      namespace: l.tlsSecretNamespace || values.namespace,
                    },
                  ],
                }
              : {}),
          };
        }
        return listener;
      }),
    },
  };
  return YAML.stringify(obj);
};

const yamlToForm = (yamlStr: string): Partial<GatewayFormValues> => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = obj.metadata as Record<string, string> | undefined;
  const spec = obj.spec as Record<string, unknown> | undefined;
  const rawListeners = (spec?.listeners as Record<string, unknown>[]) ?? [];

  return {
    name: meta?.name ?? '',
    namespace: meta?.namespace ?? 'default',
    gatewayClass: spec?.gatewayClassName as string ?? '',
    listeners: rawListeners.map((l) => {
      const tls = l.tls as Record<string, unknown> | undefined;
      const certRef = (tls?.certificateRefs as Record<string, string>[] | undefined)?.[0];
      return {
        name: l.name as string,
        port: l.port as number,
        protocol: l.protocol as string,
        hostname: l.hostname as string | undefined,
        tlsMode: tls?.mode as string | undefined,
        tlsSecretName: certRef?.name,
        tlsSecretNamespace: certRef?.namespace,
      };
    }),
  };
};

const GatewayForm: React.FC<GatewayFormProps> = ({
  open,
  clusterId,
  editing,
  onClose,
  onSuccess,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [form] = Form.useForm<GatewayFormValues>();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);
  const [loading, setLoading] = useState(false);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [gatewayClasses, setGatewayClasses] = useState<GatewayClassItem[]>([]);

  const isEdit = !!editing;

  useEffect(() => {
    if (!open) return;
    getNamespaces(Number(clusterId)).then(setNamespaces).catch(() => {});
    gatewayService.listGatewayClasses(clusterId).then((r) => setGatewayClasses(r.items ?? [])).catch(() => {});

    if (editing) {
      gatewayService.getGatewayYAML(clusterId, editing.namespace, editing.name).then((r) => {
        setYamlContent(r.yaml);
        try {
          const vals = yamlToForm(r.yaml);
          form.setFieldsValue(vals);
        } catch {
          // fall through to YAML tab
        }
      }).catch(() => {});
    } else {
      form.resetFields();
      setYamlContent(DEFAULT_YAML);
    }
    setActiveTab('form');
  }, [open, clusterId, editing, form]);

  const handleTabChange = (key: string) => {
    if (key === 'yaml' && activeTab === 'form') {
      try {
        const vals = form.getFieldsValue();
        setYamlContent(formToYaml(vals));
      } catch { /* leave existing yaml */ }
    } else if (key === 'form' && activeTab === 'yaml') {
      try {
        form.setFieldsValue(yamlToForm(yamlContent));
      } catch {
        message.error(t('gatewayapi.messages.yamlParseError'));
      }
    }
    setActiveTab(key);
  };

  const handleSubmit = async () => {
    setLoading(true);
    try {
      let yaml = yamlContent;
      if (activeTab === 'form') {
        const vals = await form.validateFields();
        yaml = formToYaml(vals);
      }
      const ns = activeTab === 'form'
        ? form.getFieldValue('namespace') as string
        : ((YAML.parse(yaml) as Record<string, unknown>)?.metadata as Record<string, string> | undefined)?.['namespace'] ?? 'default';

      if (isEdit && editing) {
        await gatewayService.updateGateway(clusterId, editing.namespace, editing.name, yaml);
        message.success(t('gatewayapi.messages.updateGatewaySuccess'));
      } else {
        await gatewayService.createGateway(clusterId, ns, yaml);
        message.success(t('gatewayapi.messages.createGatewaySuccess'));
      }
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error(msg || t(isEdit ? 'gatewayapi.messages.updateGatewayError' : 'gatewayapi.messages.createGatewayError'));
    } finally {
      setLoading(false);
    }
  };

  const title = isEdit
    ? t('gatewayapi.form.editGateway', { name: editing!.name })
    : t('gatewayapi.form.createGateway');

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      onOk={handleSubmit}
      okText={isEdit ? t('gatewayapi.form.saveBtn') : t('gatewayapi.form.createBtn')}
      cancelText={t('gatewayapi.form.cancel')}
      confirmLoading={loading}
      width={860}
      destroyOnClose
    >
      <Tabs
        activeKey={activeTab}
        onChange={handleTabChange}
        items={[
          {
            key: 'form',
            label: t('gatewayapi.form.formTab'),
            children: (
              <div style={{ maxHeight: 560, overflowY: 'auto', paddingRight: 4 }}>
                <Form
                  form={form}
                  layout="vertical"
                  initialValues={{
                    namespace: 'default',
                    listeners: [{ name: 'http', port: 80, protocol: 'HTTP' }],
                  }}
                >
                  <Form.Item
                    name="name"
                    label={t('gatewayapi.form.name')}
                    rules={[{ required: true, message: t('gatewayapi.form.nameRequired') }]}
                  >
                    <Input placeholder={t('gatewayapi.form.namePlaceholder')} disabled={isEdit} />
                  </Form.Item>

                  <Form.Item
                    name="namespace"
                    label={t('gatewayapi.form.namespace')}
                    rules={[{ required: true, message: t('gatewayapi.form.namespaceRequired') }]}
                  >
                    <Select
                      showSearch
                      disabled={isEdit}
                      options={namespaces.map((n) => ({ label: n, value: n }))}
                    />
                  </Form.Item>

                  <Form.Item
                    name="gatewayClass"
                    label={t('gatewayapi.form.gatewayClass')}
                    rules={[{ required: true, message: t('gatewayapi.form.gatewayClassRequired') }]}
                  >
                    <Select
                      placeholder={t('gatewayapi.form.gatewayClassPlaceholder')}
                      options={gatewayClasses.map((gc) => ({ label: gc.name, value: gc.name }))}
                      showSearch
                    />
                  </Form.Item>

                  {/* Listeners */}
                  <Form.Item label={t('gatewayapi.form.listeners')}>
                    <Form.List name="listeners">
                      {(fields, { add, remove }) => (
                        <>
                          {fields.map((field) => (
                            <div
                              key={field.key}
                              style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 12, marginBottom: 8 }}
                            >
                              <Space wrap align="start" style={{ width: '100%' }}>
                                <Form.Item
                                  {...field}
                                  name={[field.name, 'name']}
                                  label={t('gatewayapi.form.listenerName')}
                                  rules={[{ required: true }]}
                                  style={{ marginBottom: 0 }}
                                >
                                  <Input placeholder={t('gatewayapi.form.listenerNamePlaceholder')} style={{ width: 100 }} />
                                </Form.Item>
                                <Form.Item
                                  {...field}
                                  name={[field.name, 'port']}
                                  label={t('gatewayapi.form.listenerPort')}
                                  rules={[{ required: true }]}
                                  style={{ marginBottom: 0 }}
                                >
                                  <InputNumber min={1} max={65535} style={{ width: 90 }} />
                                </Form.Item>
                                <Form.Item
                                  {...field}
                                  name={[field.name, 'protocol']}
                                  label={t('gatewayapi.form.listenerProtocol')}
                                  rules={[{ required: true }]}
                                  style={{ marginBottom: 0 }}
                                >
                                  <Select
                                    style={{ width: 100 }}
                                    options={PROTOCOLS.map((p) => ({ label: p, value: p }))}
                                  />
                                </Form.Item>
                                <Form.Item
                                  {...field}
                                  name={[field.name, 'hostname']}
                                  label={t('gatewayapi.form.listenerHostname')}
                                  style={{ marginBottom: 0 }}
                                >
                                  <Input placeholder="*.example.com" style={{ width: 150 }} />
                                </Form.Item>
                                <Form.Item label=" " style={{ marginBottom: 0 }}>
                                  <MinusCircleOutlined
                                    style={{ color: '#ff4d4f', cursor: 'pointer', marginTop: 8 }}
                                    onClick={() => remove(field.name)}
                                  />
                                </Form.Item>
                              </Space>

                              {/* TLS fields — shown when protocol is HTTPS or TLS */}
                              <Form.Item
                                noStyle
                                shouldUpdate={(prev, curr) =>
                                  prev.listeners?.[field.name]?.protocol !== curr.listeners?.[field.name]?.protocol
                                }
                              >
                                {({ getFieldValue }) => {
                                  const proto = getFieldValue(['listeners', field.name, 'protocol']);
                                  if (proto !== 'HTTPS' && proto !== 'TLS') return null;
                                  return (
                                    <Space wrap style={{ marginTop: 8 }}>
                                      <Form.Item
                                        {...field}
                                        name={[field.name, 'tlsMode']}
                                        label={t('gatewayapi.form.listenerTLSMode')}
                                        style={{ marginBottom: 0 }}
                                      >
                                        <Select
                                          style={{ width: 130 }}
                                          options={TLS_MODES.map((m) => ({ label: m, value: m }))}
                                        />
                                      </Form.Item>
                                      <Form.Item
                                        {...field}
                                        name={[field.name, 'tlsSecretName']}
                                        label={t('gatewayapi.form.listenerTLSSecret')}
                                        style={{ marginBottom: 0 }}
                                      >
                                        <Input placeholder="tls-secret" style={{ width: 140 }} />
                                      </Form.Item>
                                    </Space>
                                  );
                                }}
                              </Form.Item>
                            </div>
                          ))}
                          <Button
                            type="dashed"
                            block
                            icon={<PlusOutlined />}
                            onClick={() => add({ name: '', port: 80, protocol: 'HTTP' })}
                          >
                            {t('gatewayapi.form.addListener')}
                          </Button>
                        </>
                      )}
                    </Form.List>
                  </Form.Item>
                </Form>
              </div>
            ),
          },
          {
            key: 'yaml',
            label: t('gatewayapi.form.yamlTab'),
            children: (
              <MonacoEditor
                height="500px"
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

export default GatewayForm;
