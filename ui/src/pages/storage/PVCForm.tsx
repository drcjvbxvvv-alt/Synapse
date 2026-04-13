import React, { useState, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Select,
  Button,
  Space,
  Tabs,
  Alert,
  Tooltip,
  App,
} from 'antd';
import { ExclamationCircleOutlined, CheckCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { getNamespaces } from '../../services/configService';
import { StorageService } from '../../services/storageService';
import { parseApiError } from '@/utils/api';
import type { PVC } from '../../types';

const DEFAULT_YAML = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard
  volumeMode: Filesystem
`;

interface PVCFormValues {
  name: string;
  namespace: string;
  storageClassName?: string;
  capacity: string;
  accessModes: string[];
  volumeMode: string;
}

interface PVCFormProps {
  open: boolean;
  clusterId: string;
  editing?: PVC | null;
  onClose: () => void;
  onSuccess: () => void;
}

const formToYaml = (values: PVCFormValues): string => {
  const obj: Record<string, unknown> = {
    apiVersion: 'v1',
    kind: 'PersistentVolumeClaim',
    metadata: { name: values.name, namespace: values.namespace },
    spec: {
      accessModes: values.accessModes,
      resources: { requests: { storage: values.capacity } },
      volumeMode: values.volumeMode,
      ...(values.storageClassName ? { storageClassName: values.storageClassName } : {}),
    },
  };
  return YAML.stringify(obj);
};

const yamlToForm = (yamlStr: string): Partial<PVCFormValues> => {
  const obj = YAML.parse(yamlStr) as Record<string, unknown>;
  const meta = obj.metadata as Record<string, string> | undefined;
  const spec = obj.spec as Record<string, unknown> | undefined;
  const resources = spec?.resources as Record<string, unknown> | undefined;
  const requests = resources?.requests as Record<string, string> | undefined;
  return {
    name: meta?.name ?? '',
    namespace: meta?.namespace ?? 'default',
    storageClassName: (spec?.storageClassName as string) ?? undefined,
    capacity: requests?.storage ?? '1Gi',
    accessModes: (spec?.accessModes as string[]) ?? ['ReadWriteOnce'],
    volumeMode: (spec?.volumeMode as string) ?? 'Filesystem',
  };
};

const PVCForm: React.FC<PVCFormProps> = ({
  open,
  clusterId,
  editing,
  onClose,
  onSuccess,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['storage', 'common']);
  const [form] = Form.useForm<PVCFormValues>();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [storageClasses, setStorageClasses] = useState<string[]>([]);

  const isEdit = !!editing;

  useEffect(() => {
    if (!open) return;
    getNamespaces(Number(clusterId)).then(setNamespaces).catch(() => {});
    StorageService.getStorageClasses(clusterId, undefined, 1, 200)
      .then((r) => {
        const items = (r as unknown as { items: { name: string }[] }).items ?? [];
        setStorageClasses(items.map((sc) => sc.name));
      })
      .catch(() => {});

    if (editing) {
      StorageService.getPVCYAML(clusterId, editing.namespace, editing.name)
        .then((r) => {
          const yaml = (r as unknown as { yaml: string }).yaml;
          setYamlContent(yaml);
          try { form.setFieldsValue(yamlToForm(yaml)); } catch { /* ignore */ }
        })
        .catch(() => {});
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
      catch { message.error(t('storage:form.yamlParseError')); }
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
      setDryRunResult({ success: false, message: t('storage:form.yamlParseError') + ': ' + String(err) });
      return;
    }
    setDryRunning(true);
    setDryRunResult(null);
    try {
      await StorageService.applyPVCYAML(clusterId, yaml, true);
      setDryRunResult({ success: true, message: t('storage:form.dryRunPassed') });
    } catch (err: unknown) {
      setDryRunResult({ success: false, message: parseApiError(err) || t('storage:form.dryRunFailed') });
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
      await StorageService.applyPVCYAML(clusterId, yaml);
      message.success(t(isEdit ? 'storage:form.updateSuccess' : 'storage:form.createSuccess'));
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg = parseApiError(err);
      message.error(msg || t(isEdit ? 'storage:form.updateError' : 'storage:form.createError'));
    } finally {
      setLoading(false);
    }
  };

  const title = isEdit
    ? t('storage:form.editPVC', { name: editing!.name })
    : t('storage:form.createPVC');

  const footer = (
    <Space>
      <Tooltip title={t('storage:form.preCheckTooltip')}>
        <Button
          icon={dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
          onClick={handleDryRun}
          loading={dryRunning}
        >
          {t('storage:form.preCheck')}
        </Button>
      </Tooltip>
      <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
      <Button type="primary" loading={loading} onClick={handleSubmit}>
        {isEdit ? t('common:actions.save') : t('common:actions.create')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      footer={footer}
      width={760}
      destroyOnClose
    >
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('storage:form.dryRunCheckPassed') : t('storage:form.dryRunCheckFailed')}
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
            label: t('storage:form.formTab'),
            children: (
              <div style={{ maxHeight: 480, overflowY: 'auto', paddingRight: 4 }}>
                <Form
                  form={form}
                  layout="vertical"
                  initialValues={{
                    namespace: 'default',
                    capacity: '1Gi',
                    accessModes: ['ReadWriteOnce'],
                    volumeMode: 'Filesystem',
                  }}
                >
                  <Space style={{ width: '100%' }} wrap>
                    <Form.Item
                      name="name"
                      label={t('storage:form.name')}
                      rules={[{ required: true, message: t('storage:form.nameRequired') }]}
                      style={{ marginBottom: 0, minWidth: 200 }}
                    >
                      <Input placeholder={t('storage:form.namePlaceholder')} disabled={isEdit} />
                    </Form.Item>
                    <Form.Item
                      name="namespace"
                      label={t('storage:form.namespace')}
                      rules={[{ required: true, message: t('storage:form.namespaceRequired') }]}
                      style={{ marginBottom: 0, minWidth: 180 }}
                    >
                      <Select
                        showSearch
                        disabled={isEdit}
                        options={namespaces.map((n) => ({ label: n, value: n }))}
                      />
                    </Form.Item>
                  </Space>

                  <Space style={{ width: '100%', marginTop: 16 }} wrap>
                    <Form.Item
                      name="capacity"
                      label={t('storage:form.capacity')}
                      rules={[{ required: true, message: t('storage:form.capacityRequired') }]}
                      style={{ marginBottom: 0, minWidth: 140 }}
                    >
                      <Input placeholder={t('storage:form.capacityPlaceholder')} />
                    </Form.Item>
                    <Form.Item
                      name="storageClassName"
                      label={t('storage:form.storageClass')}
                      style={{ marginBottom: 0, minWidth: 200 }}
                    >
                      <Select
                        allowClear
                        showSearch
                        placeholder={t('storage:form.storageClassPlaceholder')}
                        options={storageClasses.map((sc) => ({ label: sc, value: sc }))}
                      />
                    </Form.Item>
                  </Space>

                  <Space style={{ width: '100%', marginTop: 16 }} wrap>
                    <Form.Item
                      name="accessModes"
                      label={t('storage:form.accessModes')}
                      rules={[{ required: true, message: t('storage:form.accessModesRequired') }]}
                      style={{ marginBottom: 0, minWidth: 300 }}
                    >
                      <Select
                        mode="multiple"
                        options={[
                          { label: t('storage:form.accessModeRWO'), value: 'ReadWriteOnce' },
                          { label: t('storage:form.accessModeROX'), value: 'ReadOnlyMany' },
                          { label: t('storage:form.accessModeRWX'), value: 'ReadWriteMany' },
                          { label: t('storage:form.accessModeRWOP'), value: 'ReadWriteOncePod' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item
                      name="volumeMode"
                      label={t('storage:form.volumeMode')}
                      style={{ marginBottom: 0, minWidth: 160 }}
                    >
                      <Select
                        options={[
                          { label: t('storage:form.volumeModeFilesystem'), value: 'Filesystem' },
                          { label: t('storage:form.volumeModeBlock'), value: 'Block' },
                        ]}
                      />
                    </Form.Item>
                  </Space>
                </Form>
              </div>
            ),
          },
          {
            key: 'yaml',
            label: t('storage:form.yamlTab'),
            children: (
              <MonacoEditor
                height={400}
                language="yaml"
                value={yamlContent}
                onChange={(v) => setYamlContent(v ?? '')}
                options={{ minimap: { enabled: false }, fontSize: 13, scrollBeyondLastLine: false }}
              />
            ),
          },
        ]}
      />
    </Modal>
  );
};

export default PVCForm;
