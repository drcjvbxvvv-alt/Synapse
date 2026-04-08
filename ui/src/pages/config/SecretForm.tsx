import React, { useState, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Button,
  Space,
  Tabs,
  Alert,
  Tooltip,
  Select,
  Switch,
  Tag,
  App,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  ExclamationCircleOutlined,
  CheckCircleOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
} from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { secretService, getNamespaces } from '../../services/configService';
import { parseApiError } from '../../utils/api';

const DEFAULT_YAML = `apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: my-secret
  namespace: default
data: {}
`;

const SECRET_TYPES = [
  'Opaque',
  'kubernetes.io/service-account-token',
  'kubernetes.io/dockerconfigjson',
  'kubernetes.io/tls',
  'kubernetes.io/basic-auth',
  'kubernetes.io/ssh-auth',
];

interface DataItem {
  key: string;
  value: string;
  visible: boolean;
}

interface SecretFormProps {
  open: boolean;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

const formToYaml = (name: string, namespace: string, type: string, dataItems: DataItem[]): string => {
  const data: Record<string, string> = {};
  dataItems.forEach((item) => { if (item.key) data[item.key] = item.value; });
  return YAML.stringify({ apiVersion: 'v1', kind: 'Secret', type, metadata: { name, namespace }, data });
};

const yamlToState = (yamlStr: string) => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = obj.metadata as Record<string, string> | undefined;
  const data = (obj.data as Record<string, string> | null) ?? {};
  const dataItems = Object.entries(data).map(([key, value]) => ({ key, value: String(value), visible: false }));
  return {
    name: meta?.name ?? '',
    namespace: meta?.namespace ?? 'default',
    type: (obj.type as string) ?? 'Opaque',
    dataItems,
  };
};

const SecretForm: React.FC<SecretFormProps> = ({ open, clusterId, onClose, onSuccess }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['config', 'common']);
  const [form] = Form.useForm();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);
  const [dataItems, setDataItems] = useState<DataItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>(['default']);

  useEffect(() => {
    if (!open) return;
    getNamespaces(Number(clusterId)).then(setNamespaces).catch(() => {});
    form.resetFields();
    form.setFieldsValue({ name: '', namespace: 'default', type: 'Opaque' });
    setDataItems([]);
    setYamlContent(DEFAULT_YAML);
    setActiveTab('form');
    setDryRunResult(null);
  }, [open, clusterId, form]);

  const handleTabChange = (key: string) => {
    if (key === 'yaml' && activeTab === 'form') {
      const vals = form.getFieldsValue() as { name: string; namespace: string; type: string };
      setYamlContent(formToYaml(vals.name || 'my-secret', vals.namespace || 'default', vals.type || 'Opaque', dataItems));
    } else if (key === 'form' && activeTab === 'yaml') {
      try {
        const state = yamlToState(yamlContent);
        form.setFieldsValue({ name: state.name, namespace: state.namespace, type: state.type });
        setDataItems(state.dataItems);
      } catch {
        message.error(t('config:create.messages.yamlFormatError', { error: 'parse error' }));
      }
    }
    setActiveTab(key);
  };

  const getCurrentPayload = () => {
    if (activeTab === 'form') {
      const vals = form.getFieldsValue() as { name: string; namespace: string; type: string };
      const data: Record<string, string> = {};
      dataItems.forEach((item) => { if (item.key) data[item.key] = item.value; });
      return { name: vals.name, namespace: vals.namespace || 'default', type: vals.type || 'Opaque', data };
    }
    const obj = YAML.parse(yamlContent) as Record<string, unknown>;
    const meta = obj.metadata as Record<string, string> | undefined;
    const data = (obj.data as Record<string, string> | null) ?? {};
    return {
      name: meta?.name ?? '',
      namespace: meta?.namespace ?? 'default',
      type: (obj.type as string) ?? 'Opaque',
      data,
    };
  };

  const handleDryRun = async () => {
    if (activeTab === 'yaml') {
      try { YAML.parse(yamlContent); } catch (err) {
        setDryRunResult({ success: false, message: t('config:create.messages.yamlFormatError', { error: String(err) }) });
        return;
      }
    }
    setDryRunning(true);
    setDryRunResult(null);
    try {
      const payload = getCurrentPayload();
      if (!payload.name) {
        setDryRunResult({ success: false, message: t('config:create.messages.secretNameRequired') });
        setDryRunning(false);
        return;
      }
      await secretService.createSecret(Number(clusterId), { ...payload, dryRun: true });
      setDryRunResult({ success: true, message: t('config:create.messages.dryRunPassed') });
    } catch (err: unknown) {
      setDryRunResult({ success: false, message: parseApiError(err) || t('config:create.messages.dryRunFailed') });
    } finally {
      setDryRunning(false);
    }
  };

  const handleSubmit = async () => {
    if (activeTab === 'form') {
      try { await form.validateFields(); } catch { return; }
    }
    setLoading(true);
    try {
      const payload = getCurrentPayload();
      if (!payload.name) {
        message.error(t('config:create.messages.secretNameRequired'));
        return;
      }
      await secretService.createSecret(Number(clusterId), payload);
      message.success(t('config:create.messages.secretCreateSuccess'));
      onSuccess();
      onClose();
    } catch (err: unknown) {
      message.error(parseApiError(err) || t('config:create.messages.secretCreateError'));
    } finally {
      setLoading(false);
    }
  };

  const footer = (
    <Space>
      <Tooltip title={t('config:create.preCheckTooltip')}>
        <Button
          icon={dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleDryRun}
          loading={dryRunning}
        >
          {t('config:create.preCheck')}
        </Button>
      </Tooltip>
      <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
      <Button type="primary" loading={loading} onClick={handleSubmit}>
        {t('config:list.createSecret')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={
        <Space>
          {t('config:create.createSecret')}
          <Tag color="orange">{t('config:create.sensitiveData')}</Tag>
        </Space>
      }
      open={open}
      onCancel={onClose}
      footer={footer}
      width={860}
      destroyOnClose
    >
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('config:create.dryRunCheckPassed') : t('config:create.dryRunCheckFailed')}
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
            label: t('config:create.formTab'),
            children: (
              <div style={{ maxHeight: 520, overflowY: 'auto', paddingRight: 4 }}>
                <Form form={form} layout="vertical">
                  <Space style={{ width: '100%' }} wrap>
                    <Form.Item
                      name="name"
                      label={t('config:create.name')}
                      rules={[{ required: true, message: t('config:create.messages.secretNameRequired') }]}
                      style={{ marginBottom: 0, minWidth: 200 }}
                    >
                      <Input placeholder={t('config:create.secretNamePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="namespace"
                      label={t('config:create.namespace')}
                      rules={[{ required: true }]}
                      style={{ marginBottom: 0, minWidth: 160 }}
                    >
                      <Select
                        showSearch
                        options={namespaces.map((n) => ({ label: n, value: n }))}
                      />
                    </Form.Item>
                    <Form.Item
                      name="type"
                      label={t('config:create.type')}
                      style={{ marginBottom: 0, minWidth: 240 }}
                    >
                      <Select
                        options={SECRET_TYPES.map((s) => ({ label: s, value: s }))}
                      />
                    </Form.Item>
                  </Space>

                  <div style={{ marginTop: 16, marginBottom: 8, fontWeight: 500 }}>
                    <Space>
                      <span>{t('config:create.dataContent')}</span>
                      <Tag color="orange">{t('config:create.sensitiveData')}</Tag>
                    </Space>
                  </div>
                  <Space direction="vertical" style={{ width: '100%' }} size="small">
                    {dataItems.map((item, idx) => (
                      <Space key={idx} style={{ display: 'flex' }} align="baseline">
                        <Input
                          placeholder={t('config:create.keyPlaceholder')}
                          value={item.key}
                          onChange={(e) => {
                            const next = [...dataItems];
                            next[idx].key = e.target.value;
                            setDataItems(next);
                          }}
                          style={{ width: 180 }}
                        />
                        {item.visible ? (
                          <Input
                            placeholder={t('config:create.valuePlaceholder')}
                            value={item.value}
                            onChange={(e) => {
                              const next = [...dataItems];
                              next[idx].value = e.target.value;
                              setDataItems(next);
                            }}
                            style={{ width: 280 }}
                          />
                        ) : (
                          <Input.Password
                            placeholder={t('config:create.valuePlaceholder')}
                            value={item.value}
                            onChange={(e) => {
                              const next = [...dataItems];
                              next[idx].value = e.target.value;
                              setDataItems(next);
                            }}
                            style={{ width: 280 }}
                            visibilityToggle={false}
                          />
                        )}
                        <Tooltip title={item.visible ? t('config:create.hideContent') : t('config:create.showContent')}>
                          <Switch
                            size="small"
                            checkedChildren={<EyeOutlined />}
                            unCheckedChildren={<EyeInvisibleOutlined />}
                            checked={item.visible}
                            onChange={() => {
                              const next = [...dataItems];
                              next[idx].visible = !next[idx].visible;
                              setDataItems(next);
                            }}
                          />
                        </Tooltip>
                        <Button
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          onClick={() => setDataItems(dataItems.filter((_, i) => i !== idx))}
                        />
                      </Space>
                    ))}
                    <Button
                      type="dashed"
                      icon={<PlusOutlined />}
                      onClick={() => setDataItems([...dataItems, { key: '', value: '', visible: false }])}
                    >
                      {t('config:create.addDataItem')}
                    </Button>
                  </Space>
                </Form>
              </div>
            ),
          },
          {
            key: 'yaml',
            label: t('config:create.yamlTab'),
            children: (
              <MonacoEditor
                height="460px"
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

export default SecretForm;
