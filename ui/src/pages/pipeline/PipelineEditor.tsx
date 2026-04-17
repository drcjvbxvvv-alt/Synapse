/**
 * PipelineEditor — Pipeline 建立 / 編輯 Drawer（P3-1）
 *
 * 建立模式：Form 填寫基本設定 → pipelineService.create
 * 編輯模式：
 *   Form Tab — 修改 description / 並行設定 → pipelineService.update
 *   YAML Tab — Monaco Editor 定義步驟   → pipelineService.createVersion
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Drawer,
  Form,
  Input,
  InputNumber,
  Select,
  Button,
  Tabs,
  App,
  theme,
  Spin,
  Typography,
  Flex,
} from 'antd';
import { Editor } from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery } from '@tanstack/react-query';

import pipelineService, {
  type Pipeline,
  type CreatePipelineRequest,
  type UpdatePipelineRequest,
} from '../../services/pipelineService';

const { Text } = Typography;

// ─── YAML Template ──────────────────────────────────────────────────────────

const PIPELINE_YAML_TEMPLATE = `[
  {
    "name": "build",
    "type": "build-image",
    "config": "{\"context\":\".\",\"dockerfile\":\"Dockerfile\",\"destination\":\"localhost:5001/myapp:latest\"}"
  },
  {
    "name": "deploy",
    "type": "deploy",
    "depends_on": ["build"],
    "config": "{\"manifests\":[\"k8s/deployment.yaml\",\"k8s/service.yaml\"],\"namespace\":\"default\"}"
  }
]`;

// ─── Props ───────────────────────────────────────────────────────────────────

interface PipelineEditorProps {
  open: boolean;
  pipeline: Pipeline | null;
  onClose: () => void;
  onSuccess: () => void;
}

// ─── Main Component ──────────────────────────────────────────────────────────

const PipelineEditor: React.FC<PipelineEditorProps> = ({
  open,
  pipeline,
  onClose,
  onSuccess,
}) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['pipeline', 'common']);
  const [form] = Form.useForm();

  const isEdit = pipeline !== null;

  const [activeTab, setActiveTab] = useState<'form' | 'yaml'>('form');
  const [yamlContent, setYamlContent] = useState(PIPELINE_YAML_TEMPLATE);

  // ─── Reset on open/close ─────────────────────────────────────────────────

  useEffect(() => {
    if (!open) return;
    setActiveTab('form');
    if (pipeline) {
      form.setFieldsValue({
        description: pipeline.description,
        concurrency_group: pipeline.concurrency_group,
        concurrency_policy: pipeline.concurrency_policy,
        max_concurrent_runs: pipeline.max_concurrent_runs,
      });
    } else {
      form.resetFields();
      setYamlContent(PIPELINE_YAML_TEMPLATE);
    }
  }, [open, pipeline, form]);

  // ─── Load existing version ────────────────────────────────────────────────

  const { isLoading: versionLoading } = useQuery({
    queryKey: ['pipeline-version', pipeline?.id, pipeline?.current_version_id],
    queryFn: () =>
      pipelineService.getVersion(pipeline!.id, pipeline!.current_version_id!),
    enabled: isEdit && pipeline?.current_version_id != null && activeTab === 'yaml',
    staleTime: 60_000,
    select: (data) => {
      // Use steps_json as the YAML content (stored raw by the backend)
      setYamlContent(data.steps_json || PIPELINE_YAML_TEMPLATE);
      return data;
    },
  });

  // ─── Mutations ────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (data: CreatePipelineRequest) => pipelineService.create(data),
    onSuccess: () => {
      message.success(t('pipeline:messages.createSuccess'));
      onSuccess();
    },
    onError: () => message.error(t('pipeline:messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: (data: UpdatePipelineRequest) =>
      pipelineService.update(pipeline!.id, data),
    onSuccess: () => {
      message.success(t('pipeline:messages.updateSuccess'));
      onSuccess();
    },
    onError: () => message.error(t('pipeline:messages.updateFailed')),
  });

  const createVersionMutation = useMutation({
    mutationFn: () =>
      pipelineService.createVersion(pipeline!.id, {
        steps_json: yamlContent,
      }),
    onSuccess: () => {
      message.success(t('pipeline:messages.versionSaveSuccess'));
      onSuccess();
    },
    onError: () => message.error(t('pipeline:messages.versionSaveFailed')),
  });

  const isMutating =
    createMutation.isPending || updateMutation.isPending || createVersionMutation.isPending;

  // ─── Handlers ─────────────────────────────────────────────────────────────

  const handleFormSubmit = useCallback(async () => {
    try {
      const values = await form.validateFields();
      if (isEdit) {
        updateMutation.mutate(values as UpdatePipelineRequest);
      } else {
        createMutation.mutate(values as CreatePipelineRequest);
      }
    } catch {
      // Ant Design shows inline validation errors — no extra handling needed
    }
  }, [form, isEdit, updateMutation, createMutation]);

  const handleYamlSave = useCallback(() => {
    createVersionMutation.mutate();
  }, [createVersionMutation]);

  // ─── Form content ─────────────────────────────────────────────────────────

  const formContent = (
    <div>
      <Form form={form} layout="vertical" disabled={isMutating}>
        {!isEdit && (
          <Form.Item
            name="name"
            label={t('pipeline:form.name')}
            tooltip={t('pipeline:form.nameTooltip')}
            rules={[
              { required: true, message: t('common:validation.required') },
              { max: 128 },
              {
                pattern: /^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$/,
                message: t('pipeline:form.namePattern', {
                  defaultValue: 'Lowercase letters, numbers and hyphens only',
                }),
              },
            ]}
          >
            <Input placeholder={t('pipeline:form.namePlaceholder')} />
          </Form.Item>
        )}

        <Form.Item name="description" label={t('pipeline:form.description')}>
          <Input.TextArea
            rows={3}
            placeholder={t('pipeline:form.descriptionPlaceholder')}
          />
        </Form.Item>

        <Form.Item
          name="concurrency_group"
          label={t('pipeline:form.concurrencyGroup')}
          tooltip={t('pipeline:form.concurrencyGroupTooltip')}
        >
          <Input placeholder={t('pipeline:form.concurrencyGroupPlaceholder')} />
        </Form.Item>

        <Form.Item
          name="concurrency_policy"
          label={t('pipeline:form.concurrencyPolicy')}
          initialValue="queue"
        >
          <Select
            options={[
              { label: t('pipeline:policy.cancel_previous'), value: 'cancel_previous' },
              { label: t('pipeline:policy.queue'), value: 'queue' },
              { label: t('pipeline:policy.reject'), value: 'reject' },
            ]}
          />
        </Form.Item>

        <Form.Item
          name="max_concurrent_runs"
          label={t('pipeline:form.maxConcurrentRuns')}
          initialValue={1}
        >
          <InputNumber min={1} max={50} style={{ width: '100%' }} />
        </Form.Item>
      </Form>

      <Flex justify="flex-end" gap={token.marginSM} style={{ marginTop: token.marginLG }}>
        <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
        <Button type="primary" loading={isMutating} onClick={handleFormSubmit}>
          {isEdit ? t('common:actions.save') : t('common:actions.create')}
        </Button>
      </Flex>
    </div>
  );

  // ─── YAML content ─────────────────────────────────────────────────────────

  const yamlTabContent = (
    <div>
      {pipeline?.current_version_id && (
        <Text
          type="secondary"
          style={{ fontSize: token.fontSizeSM, display: 'block', marginBottom: token.marginSM }}
        >
          {t('pipeline:editor.title')} — v{pipeline.current_version_id}
        </Text>
      )}
      <Spin spinning={versionLoading}>
        <Editor
          height="480px"
          defaultLanguage="json"
          value={yamlContent}
          onChange={(val) => setYamlContent(val ?? '')}
          options={{
            minimap: { enabled: false },
            fontSize: 13,
            lineNumbers: 'on',
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 2,
            insertSpaces: true,
            wordWrap: 'on',
            folding: true,
            foldingStrategy: 'indentation',
          }}
        />
      </Spin>
      <Flex
        justify="flex-end"
        gap={token.marginSM}
        style={{ marginTop: token.marginMD }}
      >
        <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
        <Button type="primary" loading={isMutating} onClick={handleYamlSave}>
          {t('pipeline:editor.saveVersion')}
        </Button>
      </Flex>
    </div>
  );

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <Drawer
      title={isEdit ? t('pipeline:form.editTitle') : t('pipeline:form.createTitle')}
      open={open}
      onClose={onClose}
      width={800}
      destroyOnClose
      footer={null}
    >
      {isEdit ? (
        <Tabs
          activeKey={activeTab}
          onChange={(k) => setActiveTab(k as 'form' | 'yaml')}
          items={[
            {
              key: 'form',
              label: t('pipeline:form.tabs.form'),
              children: formContent,
            },
            {
              key: 'yaml',
              label: t('pipeline:form.tabs.yaml'),
              children: yamlTabContent,
            },
          ]}
        />
      ) : (
        formContent
      )}
    </Drawer>
  );
};

export default PipelineEditor;
