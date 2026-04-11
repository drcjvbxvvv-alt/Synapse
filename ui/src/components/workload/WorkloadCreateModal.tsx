import React, { useState, useEffect, useCallback } from 'react';
import {
  Modal,
  Button,
  Space,
  Segmented,
  Alert,
  Tooltip,
  App,
  Typography,
} from 'antd';
import {
  SaveOutlined,
  FormOutlined,
  CodeOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { WorkloadService } from '../../services/workloadService';
import { parseApiError } from '../../utils/api';
import { useTranslation } from 'react-i18next';
import { getNamespaces } from '../../services/namespaceService';
import { secretService } from '../../services/configService';
import WorkloadForm from './WorkloadForm';
import { WorkloadYamlService } from '../../services/workloadYamlService';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { Form, Card } from 'antd';
import type { WorkloadFormData } from '../../types/workload';

const { Text } = Typography;

type WorkloadType = 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'ArgoRollout' | 'Job' | 'CronJob';

interface WorkloadCreateModalProps {
  open: boolean;
  workloadType: WorkloadType;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

const WorkloadCreateModal: React.FC<WorkloadCreateModalProps> = ({
  open,
  workloadType,
  clusterId,
  onClose,
  onSuccess,
}) => {
  const { message: messageApi, modal } = App.useApp();
  const { t } = useTranslation(['workload', 'common']);

  const [editMode, setEditMode] = useState<'form' | 'yaml'>('form');
  const [submitting, setSubmitting] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);

  const [form] = Form.useForm();
  const [formData, setFormData] = useState<WorkloadFormData | null>(null);

  const getDefaultYaml = useCallback((): string => {
    const defaultData: WorkloadFormData = {
      name: '',
      namespace: 'default',
      replicas: workloadType === 'DaemonSet' ? undefined : 1,
      containers: [
        {
          name: 'main',
          image: '',
          imagePullPolicy: 'IfNotPresent',
          resources: {
            requests: { cpu: '100m', memory: '128Mi' },
            limits: { cpu: '500m', memory: '512Mi' },
          },
        },
      ],
    };
    return WorkloadYamlService.formDataToYAML(workloadType, defaultData);
  }, [workloadType]);

  const [yamlContent, setYamlContent] = useState(() => getDefaultYaml());
  const [namespaces, setNamespaces] = useState<string[]>(['default']);
  const [imagePullSecretsList, setImagePullSecretsList] = useState<string[]>([]);
  const [rolloutCRDEnabled, setRolloutCRDEnabled] = useState<boolean | null>(null);

  const currentNamespace = Form.useWatch('namespace', form) || 'default';

  // Reset state when modal opens
  useEffect(() => {
    if (open) {
      setEditMode('form');
      setDryRunResult(null);
      setFormData(null);
      setYamlContent(getDefaultYaml());
      form.resetFields();
    }
  }, [open, getDefaultYaml, form]);

  // Load namespaces
  useEffect(() => {
    if (!open || !clusterId) return;
    getNamespaces(Number(clusterId))
      .then((nsList) => {
        if (nsList && nsList.length > 0) {
          setNamespaces(nsList.map((ns) => ns.name));
        } else {
          setNamespaces(['default']);
        }
      })
      .catch(() => setNamespaces(['default']));
  }, [open, clusterId]);

  // Load image pull secrets when namespace changes
  useEffect(() => {
    if (!open || !clusterId || !currentNamespace) return;
    secretService
      .getSecrets(Number(clusterId), {
        namespace: currentNamespace,
        type: 'kubernetes.io/dockerconfigjson',
      })
      .then((data) => {
        setImagePullSecretsList(data?.items?.map((s) => s.name) ?? []);
      })
      .catch(() => setImagePullSecretsList([]));
  }, [open, clusterId, currentNamespace]);

  // Check Rollout CRD
  useEffect(() => {
    if (!open || workloadType !== 'Rollout' || !clusterId) return;
    WorkloadService.checkRolloutCRD(clusterId)
      .then((res: { enabled: boolean }) => setRolloutCRDEnabled(res.enabled))
      .catch(() => setRolloutCRDEnabled(false));
  }, [open, workloadType, clusterId]);

  const formToYaml = useCallback((): string => {
    try {
      const values = form.getFieldsValue(true);
      const fd: WorkloadFormData = {
        name: values.name || '',
        namespace: values.namespace || 'default',
        replicas: values.replicas,
        description: values.description,
        labels: values.labels,
        annotations: values.annotations,
        containers: values.containers || [],
        initContainers: values.initContainers,
        volumes: values.volumes,
        imagePullSecrets: values.imagePullSecrets,
        scheduling: values.scheduling,
        tolerations: values.tolerations,
        strategy: values.strategy,
        minReadySeconds: values.minReadySeconds,
        revisionHistoryLimit: values.revisionHistoryLimit,
        progressDeadlineSeconds: values.progressDeadlineSeconds,
        terminationGracePeriodSeconds: values.terminationGracePeriodSeconds,
        dnsPolicy: values.dnsPolicy,
        dnsConfig: values.dnsConfig,
        hostNetwork: values.hostNetwork,
        serviceName: values.serviceName,
        schedule: values.schedule,
        suspend: values.suspend,
        concurrencyPolicy: values.concurrencyPolicy,
        completions: values.completions,
        parallelism: values.parallelism,
        backoffLimit: values.backoffLimit,
        activeDeadlineSeconds: values.activeDeadlineSeconds,
        rolloutStrategy: values.rolloutStrategy,
      };
      return WorkloadYamlService.formDataToYAML(workloadType, fd);
    } catch {
      return yamlContent;
    }
  }, [form, workloadType, yamlContent]);

  const yamlToForm = useCallback((): boolean => {
    try {
      const parsed = WorkloadYamlService.yamlToFormData(yamlContent);
      if (parsed) {
        form.setFieldsValue(parsed);
        setFormData(parsed);
        return true;
      }
      return false;
    } catch (err) {
      messageApi.error(
        t('messages.yamlFormatError') + ': ' + (err instanceof Error ? err.message : ''),
      );
      return false;
    }
  }, [yamlContent, form, messageApi, t]);

  const handleModeChange = (newMode: string) => {
    if (newMode === editMode) return;
    if (editMode === 'form' && newMode === 'yaml') {
      setYamlContent(formToYaml());
    } else if (editMode === 'yaml' && newMode === 'form') {
      if (!yamlToForm()) return;
    }
    setEditMode(newMode as 'form' | 'yaml');
    setDryRunResult(null);
  };

  const handleDryRun = async () => {
    const yaml = editMode === 'form' ? formToYaml() : yamlContent;
    try {
      YAML.parse(yaml);
    } catch (err) {
      setDryRunResult({
        success: false,
        message:
          t('workload:create.yamlFormatError') +
          ': ' +
          (err instanceof Error ? err.message : t('common:unknown')),
      });
      return;
    }
    setDryRunning(true);
    setDryRunResult(null);
    try {
      await WorkloadService.applyYAML(clusterId, yaml, true);
      setDryRunResult({ success: true, message: t('create.dryRunPassed') });
    } catch (error: unknown) {
      setDryRunResult({ success: false, message: parseApiError(error) });
    } finally {
      setDryRunning(false);
    }
  };

  const submitYaml = async (yaml: string) => {
    setSubmitting(true);
    try {
      await WorkloadService.applyYAML(clusterId, yaml, false);
      messageApi.success(t('messages.createSuccess'));
      onSuccess();
    } catch (error: unknown) {
      messageApi.error(error instanceof Error ? error.message : t('messages.operationFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  const handleSubmit = async () => {
    let yaml: string;
    if (editMode === 'form') {
      try {
        await form.validateFields();
        yaml = formToYaml();
      } catch {
        messageApi.error(t('messages.checkForm'));
        return;
      }
    } else {
      yaml = yamlContent;
    }

    try {
      YAML.parse(yaml);
    } catch (err) {
      messageApi.error(
        t('messages.yamlFormatError') + ': ' + (err instanceof Error ? err.message : ''),
      );
      return;
    }

    modal.confirm({
      title: t('create.confirmCreate'),
      content: (
        <div>
          <p>{t('create.confirmCreateDesc')}</p>
          <p>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t('create.confirmCreateHint')}
            </Text>
          </p>
        </div>
      ),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: () => submitYaml(yaml),
    });
  };

  const isRolloutDisabled = workloadType === 'Rollout' && rolloutCRDEnabled === false;

  const footer = (
    <Space>
      <Tooltip title={t('create.preCheckTooltip')}>
        <Button
          onClick={handleDryRun}
          loading={dryRunning}
          disabled={isRolloutDisabled}
          icon={
            dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />
          }
        >
          {t('create.preCheck')}
        </Button>
      </Tooltip>
      <Button onClick={onClose}>{t('create.cancel')}</Button>
      <Button
        type="primary"
        icon={<SaveOutlined />}
        onClick={handleSubmit}
        loading={submitting}
        disabled={isRolloutDisabled}
      >
        {t('create.create')}
      </Button>
    </Space>
  );

  return (
    <Modal
      title={
        <Space>
          <span>
            {t('create.create')} {workloadType}
          </span>
          <Segmented
            value={editMode}
            onChange={handleModeChange}
            options={[
              { value: 'form', icon: <FormOutlined />, label: t('create.formMode') },
              { value: 'yaml', icon: <CodeOutlined />, label: t('create.yamlMode') },
            ]}
          />
        </Space>
      }
      open={open}
      onCancel={onClose}
      width="90%"
      style={{ top: 20, maxWidth: 1400 }}
      footer={footer}
      destroyOnHidden
    >
      {/* Rollout CRD 未安裝警告 */}
      {workloadType === 'Rollout' && rolloutCRDEnabled === false && (
        <Alert
          message="Argo Rollouts 未安裝"
          description="此叢集尚未安裝 Argo Rollouts，無法建立 Rollout 資源。請先在叢集中安裝 Argo Rollouts。"
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 預檢結果 */}
      {dryRunResult && (
        <Alert
          message={
            dryRunResult.success
              ? t('create.dryRunCheckPassed')
              : t('create.dryRunCheckFailed')
          }
          description={dryRunResult.message}
          type={dryRunResult.success ? 'success' : 'error'}
          showIcon
          closable
          onClose={() => setDryRunResult(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 內容區域 */}
      {editMode === 'form' ? (
        <WorkloadForm
          workloadType={workloadType}
          namespaces={namespaces}
          imagePullSecretsList={imagePullSecretsList}
          initialData={formData || undefined}
          onValuesChange={(_, allValues) => {
            setFormData(allValues);
            setDryRunResult(null);
          }}
          form={form}
          isEdit={false}
        />
      ) : (
        <Card title={t('create.yamlEdit')}>
          <MonacoEditor
            height="500px"
            language="yaml"
            value={yamlContent}
            onChange={(value) => {
              setYamlContent(value || '');
              setDryRunResult(null);
            }}
            options={{
              minimap: { enabled: false },
              fontSize: 14,
              lineNumbers: 'on',
              wordWrap: 'on',
              automaticLayout: true,
              scrollBeyondLastLine: false,
            }}
          />
        </Card>
      )}
    </Modal>
  );
};

export default WorkloadCreateModal;
