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
import { parseApiError } from '@/utils/api';
import type {
  HTTPRouteItem,
  HTTPRouteFormValues,
  GatewayItem,
} from './gatewayTypes';

const PATH_TYPES = ['PathPrefix', 'Exact', 'RegularExpression'];

const DEFAULT_YAML = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
      namespace: default
  hostnames:
    - example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: my-service
          port: 80
          weight: 1
`;

interface HTTPRouteFormProps {
  open: boolean;
  clusterId: string;
  editing?: HTTPRouteItem | null;
  onClose: () => void;
  onSuccess: () => void;
}

const formToYaml = (values: HTTPRouteFormValues): string => {
  const obj = {
    apiVersion: 'gateway.networking.k8s.io/v1',
    kind: 'HTTPRoute',
    metadata: { name: values.name, namespace: values.namespace },
    spec: {
      parentRefs: (values.parentRefs || []).map((p) => ({
        name: p.gatewayName,
        namespace: p.gatewayNamespace,
        ...(p.sectionName ? { sectionName: p.sectionName } : {}),
      })),
      hostnames: (values.hostnames || []).filter(Boolean),
      rules: (values.rules || []).map((rule) => ({
        matches: (rule.matches || []).map((m) => ({
          path: { type: m.pathType || 'PathPrefix', value: m.pathValue || '/' },
        })),
        backendRefs: (rule.backends || []).map((b) => ({
          name: b.name,
          port: b.port,
          weight: b.weight ?? 1,
        })),
      })),
    },
  };
  return YAML.stringify(obj);
};

const yamlToForm = (yamlStr: string): Partial<HTTPRouteFormValues> => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = obj.metadata as Record<string, string> | undefined;
  const spec = obj.spec as Record<string, unknown> | undefined;

  const parentRefs = ((spec?.parentRefs as Record<string, unknown>[]) ?? []).map((p) => ({
    gatewayName: p.name as string,
    gatewayNamespace: p.namespace as string ?? '',
    sectionName: p.sectionName as string | undefined,
  }));

  const hostnames = (spec?.hostnames as string[]) ?? [];

  const rules = ((spec?.rules as Record<string, unknown>[]) ?? []).map((r) => ({
    matches: ((r.matches as Record<string, unknown>[]) ?? []).map((m) => {
      const path = m.path as Record<string, string> | undefined;
      return {
        pathType: (path?.type ?? 'PathPrefix') as 'PathPrefix' | 'Exact' | 'RegularExpression',
        pathValue: path?.value ?? '/',
      };
    }),
    backends: ((r.backendRefs as Record<string, unknown>[]) ?? []).map((b) => ({
      name: b.name as string,
      port: b.port as number,
      weight: (b.weight as number) ?? 1,
    })),
  }));

  return {
    name: meta?.name ?? '',
    namespace: meta?.namespace ?? 'default',
    hostnames,
    parentRefs,
    rules,
  };
};

const HTTPRouteForm: React.FC<HTTPRouteFormProps> = ({
  open,
  clusterId,
  editing,
  onClose,
  onSuccess,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [form] = Form.useForm<HTTPRouteFormValues>();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [gateways, setGateways] = useState<GatewayItem[]>([]);

  const isEdit = !!editing;

  useEffect(() => {
    if (!open) return;
    getNamespaces(Number(clusterId)).then(setNamespaces).catch(() => {});
    gatewayService.listGateways(clusterId).then((r) => setGateways(r.items ?? [])).catch(() => {});

    if (editing) {
      gatewayService.getHTTPRouteYAML(clusterId, editing.namespace, editing.name).then((r) => {
        setYamlContent(r.yaml);
        try { form.setFieldsValue(yamlToForm(r.yaml)); } catch { /* ignore */ }
      }).catch(() => {});
    } else {
      form.resetFields();
      setYamlContent(DEFAULT_YAML);
    }
    setActiveTab('form');
    setDryRunResult(null);
  }, [open, clusterId, editing, form]);

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
      if (isEdit && editing) {
        await gatewayService.updateHTTPRoute(clusterId, editing.namespace, editing.name, yaml, true);
      } else {
        await gatewayService.createHTTPRoute(clusterId, ns, yaml, true);
      }
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
      if (activeTab === 'form') {
        const vals = await form.validateFields();
        yaml = formToYaml(vals);
      }
      const ns = activeTab === 'form'
        ? form.getFieldValue('namespace') as string
        : ((YAML.parse(yaml) as Record<string, unknown>)?.metadata as Record<string, string> | undefined)?.['namespace'] ?? 'default';

      if (isEdit && editing) {
        await gatewayService.updateHTTPRoute(clusterId, editing.namespace, editing.name, yaml);
        message.success(t('gatewayapi.messages.updateHTTPRouteSuccess'));
      } else {
        await gatewayService.createHTTPRoute(clusterId, ns, yaml);
        message.success(t('gatewayapi.messages.createHTTPRouteSuccess'));
      }
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      message.error(msg || t(isEdit ? 'gatewayapi.messages.updateHTTPRouteError' : 'gatewayapi.messages.createHTTPRouteError'));
    } finally {
      setLoading(false);
    }
  };

  const title = isEdit
    ? t('gatewayapi.form.editHTTPRoute', { name: editing!.name })
    : t('gatewayapi.form.createHTTPRoute');

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
        {isEdit ? t('gatewayapi.form.saveBtn') : t('gatewayapi.form.createBtn')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      footer={footer}
      width={900}
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
              <div style={{ maxHeight: 580, overflowY: 'auto', paddingRight: 4 }}>
                <Form
                  form={form}
                  layout="vertical"
                  initialValues={{
                    namespace: 'default',
                    hostnames: [''],
                    parentRefs: [{ gatewayName: '', gatewayNamespace: 'default' }],
                    rules: [{ matches: [{ pathType: 'PathPrefix', pathValue: '/' }], backends: [{ name: '', port: 80, weight: 1 }] }],
                  }}
                >
                  <Space style={{ width: '100%' }} wrap>
                    <Form.Item
                      name="name"
                      label={t('gatewayapi.form.name')}
                      rules={[{ required: true, message: t('gatewayapi.form.nameRequired') }]}
                      style={{ marginBottom: 0, minWidth: 200 }}
                    >
                      <Input placeholder={t('gatewayapi.form.namePlaceholder')} disabled={isEdit} />
                    </Form.Item>
                    <Form.Item
                      name="namespace"
                      label={t('gatewayapi.form.namespace')}
                      rules={[{ required: true, message: t('gatewayapi.form.namespaceRequired') }]}
                      style={{ marginBottom: 0, minWidth: 180 }}
                    >
                      <Select
                        showSearch
                        disabled={isEdit}
                        options={namespaces.map((n) => ({ label: n, value: n }))}
                      />
                    </Form.Item>
                  </Space>

                  {/* Hostnames */}
                  <Divider orientation="left" plain style={{ marginTop: 16 }}>
                    {t('gatewayapi.form.hostnames')}
                  </Divider>
                  <Form.List name="hostnames">
                    {(fields, { add, remove }) => (
                      <>
                        {fields.map((field) => (
                          <Space key={field.key} style={{ display: 'flex', marginBottom: 6 }}>
                            <Form.Item {...field} noStyle>
                              <Input placeholder={t('gatewayapi.form.hostnameValue')} style={{ width: 280 }} />
                            </Form.Item>
                            <MinusCircleOutlined style={{ color: '#ff4d4f' }} onClick={() => remove(field.name)} />
                          </Space>
                        ))}
                        <Button type="dashed" icon={<PlusOutlined />} onClick={() => add('')}>
                          {t('gatewayapi.form.addHostname')}
                        </Button>
                      </>
                    )}
                  </Form.List>

                  {/* Parent Refs */}
                  <Divider orientation="left" plain>{t('gatewayapi.form.parentRefs')}</Divider>
                  <Form.List name="parentRefs">
                    {(fields, { add, remove }) => (
                      <>
                        {fields.map((field) => (
                          <Space key={field.key} wrap style={{ display: 'flex', marginBottom: 8 }} align="start">
                            <Form.Item
                              {...field}
                              name={[field.name, 'gatewayName']}
                              label={t('gatewayapi.form.gatewayName')}
                              rules={[{ required: true }]}
                              style={{ marginBottom: 0 }}
                            >
                              <Select
                                showSearch
                                style={{ width: 180 }}
                                options={gateways.map((g) => ({
                                  label: `${g.namespace}/${g.name}`,
                                  value: g.name,
                                }))}
                                onSelect={(val) => {
                                  const gw = gateways.find((g) => g.name === val);
                                  if (gw) {
                                    const refs = form.getFieldValue('parentRefs') as Record<string, string>[];
                                    refs[field.name] = { ...refs[field.name], gatewayNamespace: gw.namespace };
                                    form.setFieldsValue({ parentRefs: refs });
                                  }
                                }}
                              />
                            </Form.Item>
                            <Form.Item
                              {...field}
                              name={[field.name, 'gatewayNamespace']}
                              label={t('gatewayapi.form.gatewayNamespace')}
                              style={{ marginBottom: 0 }}
                            >
                              <Input style={{ width: 140 }} placeholder="default" />
                            </Form.Item>
                            <Form.Item
                              {...field}
                              name={[field.name, 'sectionName']}
                              label={t('gatewayapi.form.sectionName')}
                              style={{ marginBottom: 0 }}
                            >
                              <Input style={{ width: 120 }} placeholder="http" />
                            </Form.Item>
                            <Form.Item label=" " style={{ marginBottom: 0 }}>
                              <MinusCircleOutlined style={{ color: '#ff4d4f', marginTop: 8 }} onClick={() => remove(field.name)} />
                            </Form.Item>
                          </Space>
                        ))}
                        <Button
                          type="dashed"
                          icon={<PlusOutlined />}
                          onClick={() => add({ gatewayName: '', gatewayNamespace: 'default' })}
                        >
                          {t('gatewayapi.form.addParentRef')}
                        </Button>
                      </>
                    )}
                  </Form.List>

                  {/* Rules */}
                  <Divider orientation="left" plain>{t('gatewayapi.form.rules')}</Divider>
                  <Form.List name="rules">
                    {(ruleFields, { add: addRule, remove: removeRule }) => (
                      <>
                        {ruleFields.map((ruleField, rIdx) => (
                          <div
                            key={ruleField.key}
                            style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 12, marginBottom: 12 }}
                          >
                            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                              <strong>Rule {rIdx + 1}</strong>
                              <Button type="link" danger size="small" onClick={() => removeRule(ruleField.name)}>
                                {t('gatewayapi.form.deleteRule')}
                              </Button>
                            </div>

                            {/* Matches */}
                            <div style={{ marginBottom: 8, color: '#666', fontSize: 12 }}>{t('gatewayapi.form.matches')}</div>
                            <Form.List name={[ruleField.name, 'matches']}>
                              {(matchFields, { add: addMatch, remove: removeMatch }) => (
                                <>
                                  {matchFields.map((mf) => (
                                    <Space key={mf.key} style={{ display: 'flex', marginBottom: 6 }} align="baseline">
                                      <Form.Item {...mf} name={[mf.name, 'pathType']} noStyle>
                                        <Select
                                          style={{ width: 160 }}
                                          options={PATH_TYPES.map((p) => ({ label: p, value: p }))}
                                        />
                                      </Form.Item>
                                      <Form.Item {...mf} name={[mf.name, 'pathValue']} noStyle>
                                        <Input placeholder="/" style={{ width: 200 }} />
                                      </Form.Item>
                                      <MinusCircleOutlined style={{ color: '#ff4d4f' }} onClick={() => removeMatch(mf.name)} />
                                    </Space>
                                  ))}
                                  <Button
                                    size="small"
                                    type="dashed"
                                    icon={<PlusOutlined />}
                                    onClick={() => addMatch({ pathType: 'PathPrefix', pathValue: '/' })}
                                    style={{ marginBottom: 8 }}
                                  >
                                    {t('gatewayapi.form.addMatch')}
                                  </Button>
                                </>
                              )}
                            </Form.List>

                            {/* Backends */}
                            <div style={{ marginBottom: 8, color: '#666', fontSize: 12 }}>{t('gatewayapi.form.backends')}</div>
                            <Form.List name={[ruleField.name, 'backends']}>
                              {(backendFields, { add: addBackend, remove: removeBackend }) => (
                                <>
                                  {backendFields.map((bf) => (
                                    <Space key={bf.key} style={{ display: 'flex', marginBottom: 6 }} align="baseline">
                                      <Form.Item {...bf} name={[bf.name, 'name']} noStyle rules={[{ required: true }]}>
                                        <Input placeholder={t('gatewayapi.form.backendName')} style={{ width: 160 }} />
                                      </Form.Item>
                                      <Form.Item {...bf} name={[bf.name, 'port']} noStyle rules={[{ required: true }]}>
                                        <InputNumber min={1} max={65535} placeholder={t('gatewayapi.form.backendPort')} style={{ width: 80 }} />
                                      </Form.Item>
                                      <Form.Item {...bf} name={[bf.name, 'weight']} noStyle>
                                        <InputNumber min={0} max={1000000} placeholder={t('gatewayapi.form.backendWeight')} style={{ width: 70 }} />
                                      </Form.Item>
                                      <MinusCircleOutlined style={{ color: '#ff4d4f' }} onClick={() => removeBackend(bf.name)} />
                                    </Space>
                                  ))}
                                  <Button
                                    size="small"
                                    type="dashed"
                                    icon={<PlusOutlined />}
                                    onClick={() => addBackend({ name: '', port: 80, weight: 1 })}
                                  >
                                    {t('gatewayapi.form.addBackend')}
                                  </Button>
                                </>
                              )}
                            </Form.List>
                          </div>
                        ))}
                        <Button
                          type="dashed"
                          block
                          icon={<PlusOutlined />}
                          onClick={() => addRule({
                            matches: [{ pathType: 'PathPrefix', pathValue: '/' }],
                            backends: [{ name: '', port: 80, weight: 1 }],
                          })}
                        >
                          {t('gatewayapi.form.addRule')}
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
                height="520px"
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

export default HTTPRouteForm;
