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
  App,
} from 'antd';
import { PlusOutlined, DeleteOutlined, ExclamationCircleOutlined, CheckCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { configMapService, getNamespaces } from '../../services/configService';
import { parseApiError, showApiError } from '../../utils/api';

const DEFAULT_YAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: default
data:
  key: value
`;

interface DataItem {
  key: string;
  value: string;
}

interface ConfigMapFormProps {
  open: boolean;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

const formToYaml = (name: string, namespace: string, dataItems: DataItem[]): string => {
  const data: Record<string, string> = {};
  dataItems.forEach((item) => { if (item.key) data[item.key] = item.value; });
  return YAML.stringify({ apiVersion: 'v1', kind: 'ConfigMap', metadata: { name, namespace }, data });
};

const yamlToState = (yamlStr: string) => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = obj.metadata as Record<string, string> | undefined;
  const data = (obj.data as Record<string, string> | null) ?? {};
  const dataItems = Object.entries(data).map(([key, value]) => ({ key, value: String(value) }));
  return { name: meta?.name ?? '', namespace: meta?.namespace ?? 'default', dataItems };
};

const ConfigMapForm: React.FC<ConfigMapFormProps> = ({ open, clusterId, onClose, onSuccess }) => {
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
    form.setFieldsValue({ name: '', namespace: 'default' });
    setDataItems([]);
    setYamlContent(DEFAULT_YAML);
    setActiveTab('form');
    setDryRunResult(null);
  }, [open, clusterId, form]);

  const handleTabChange = (key: string) => {
    if (key === 'yaml' && activeTab === 'form') {
      const vals = form.getFieldsValue() as { name: string; namespace: string };
      setYamlContent(formToYaml(vals.name || 'my-configmap', vals.namespace || 'default', dataItems));
    } else if (key === 'form' && activeTab === 'yaml') {
      try {
        const state = yamlToState(yamlContent);
        form.setFieldsValue({ name: state.name, namespace: state.namespace });
        setDataItems(state.dataItems);
      } catch {
        message.error(t('create.messages.yamlFormatError', { error: 'parse error' }));
      }
    }
    setActiveTab(key);
  };

  const getCurrentPayload = () => {
    if (activeTab === 'form') {
      const vals = form.getFieldsValue() as { name: string; namespace: string };
      const data: Record<string, string> = {};
      dataItems.forEach((item) => { if (item.key) data[item.key] = item.value; });
      return { name: vals.name, namespace: vals.namespace || 'default', data };
    }
    const obj = YAML.parse(yamlContent) as Record<string, unknown>;
    const meta = obj.metadata as Record<string, string> | undefined;
    const data = (obj.data as Record<string, string> | null) ?? {};
    return { name: meta?.name ?? '', namespace: meta?.namespace ?? 'default', data };
  };

  const handleDryRun = async () => {
    if (activeTab === 'yaml') {
      try { YAML.parse(yamlContent); } catch (err) {
        setDryRunResult({ success: false, message: t('create.messages.yamlFormatError', { error: String(err) }) });
        return;
      }
    }
    setDryRunning(true);
    setDryRunResult(null);
    try {
      const payload = getCurrentPayload();
      if (!payload.name) {
        setDryRunResult({ success: false, message: t('create.messages.configMapNameRequired') });
        setDryRunning(false);
        return;
      }
      await configMapService.createConfigMap(Number(clusterId), { ...payload, dryRun: true });
      setDryRunResult({ success: true, message: t('create.messages.dryRunPassed') });
    } catch (err: unknown) {
      setDryRunResult({ success: false, message: parseApiError(err) || t('create.messages.dryRunFailed') });
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
        message.error(t('create.messages.configMapNameRequired'));
        return;
      }
      await configMapService.createConfigMap(Number(clusterId), payload);
      message.success(t('create.messages.configMapCreateSuccess'));
      onSuccess();
      onClose();
    } catch (err: unknown) {
      showApiError(err, t('create.messages.configMapCreateError'));
    } finally {
      setLoading(false);
    }
  };

  const footer = (
    <Space>
      <Tooltip title={t('create.preCheckTooltip')}>
        <Button
          icon={dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleDryRun}
          loading={dryRunning}
        >
          {t('create.preCheck')}
        </Button>
      </Tooltip>
      <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
      <Button type="primary" loading={loading} onClick={handleSubmit}>
        {t('list.createConfigMap')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={t('create.createConfigMap')}
      open={open}
      onCancel={onClose}
      footer={footer}
      width={860}
      destroyOnClose
    >
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('create.dryRunCheckPassed') : t('create.dryRunCheckFailed')}
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
            label: t('create.formTab'),
            children: (
              <div style={{ maxHeight: 520, overflowY: 'auto', paddingRight: 4 }}>
                <Form form={form} layout="vertical">
                  <Space style={{ width: '100%' }} wrap>
                    <Form.Item
                      name="name"
                      label={t('create.name')}
                      rules={[{ required: true, message: t('create.messages.configMapNameRequired') }]}
                      style={{ marginBottom: 0, minWidth: 220 }}
                    >
                      <Input placeholder={t('create.configMapNamePlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="namespace"
                      label={t('create.namespace')}
                      rules={[{ required: true }]}
                      style={{ marginBottom: 0, minWidth: 180 }}
                    >
                      <Select
                        showSearch
                        options={namespaces.map((n) => ({ label: n, value: n }))}
                      />
                    </Form.Item>
                  </Space>

                  <div style={{ marginTop: 16, marginBottom: 8, fontWeight: 500 }}>{t('create.dataContent')}</div>
                  <Space direction="vertical" style={{ width: '100%' }} size="small">
                    {dataItems.map((item, idx) => (
                      <Space key={idx} style={{ display: 'flex' }} align="baseline">
                        <Input
                          placeholder={t('create.keyPlaceholder')}
                          value={item.key}
                          onChange={(e) => {
                            const next = [...dataItems];
                            next[idx].key = e.target.value;
                            setDataItems(next);
                          }}
                          style={{ width: 200 }}
                        />
                        <Input
                          placeholder={t('create.valuePlaceholder')}
                          value={item.value}
                          onChange={(e) => {
                            const next = [...dataItems];
                            next[idx].value = e.target.value;
                            setDataItems(next);
                          }}
                          style={{ width: 320 }}
                        />
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
                      onClick={() => setDataItems([...dataItems, { key: '', value: '' }])}
                    >
                      {t('create.addDataItem')}
                    </Button>
                  </Space>
                </Form>
              </div>
            ),
          },
          {
            key: 'yaml',
            label: t('create.yamlTab'),
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

export default ConfigMapForm;
