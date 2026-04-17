/**
 * PipelineEditor — Pipeline 建立 / 編輯 Drawer
 *
 * 統一介面：上方為 Pipeline 基本設定，下方為 Steps JSON 編輯器。
 * 建立模式：先建立 Pipeline → 再建立 Version（steps_json）
 * 編輯模式：更新 Pipeline metadata + 建立新 Version（若 steps 有變動）
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Drawer,
  Form,
  Input,
  InputNumber,
  Select,
  Button,
  App,
  theme,
  Spin,
  Typography,
  Flex,
  Divider,
} from 'antd';
import { Editor } from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import pipelineService, {
  type Pipeline,
  type CreatePipelineRequest,
  type UpdatePipelineRequest,
} from '../../services/pipelineService';
import { request } from '../../utils/api';
import { parseApiError } from '../../utils/api';
import type { Project } from '../../services/projectService';

const { Text } = Typography;

// ─── Steps Template ─────────────────────────────────────────────────────────

const STEPS_TEMPLATE = `[
  {
    "name": "build",
    "type": "build-image",
    "config": {
      "context": ".",
      "dockerfile": "Dockerfile",
      "destination": "localhost:5001/myapp:latest"
    }
  },
  {
    "name": "deploy",
    "type": "deploy",
    "depends_on": ["build"],
    "config": {
      "manifests": ["k8s/deployment.yaml", "k8s/service.yaml"],
      "namespace": "default"
    }
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
  const queryClient = useQueryClient();
  const [form] = Form.useForm();

  const isEdit = pipeline !== null;

  const [stepsJson, setStepsJson] = useState(STEPS_TEMPLATE);
  const [saving, setSaving] = useState(false);

  // Load all projects for the selector
  const { data: projectsData } = useQuery({
    queryKey: ['projects-all'],
    queryFn: () => request.get<{ items: Project[]; total: number }>('/projects'),
    staleTime: 60_000,
    enabled: open,
  });
  const projectOptions = (projectsData?.items ?? []).map((p) => ({
    label: `${p.name} (${p.repo_url})`,
    value: p.id,
  }));

  // ─── Reset on open/close ─────────────────────────────────────────────────

  useEffect(() => {
    if (!open) return;
    if (pipeline) {
      form.setFieldsValue({
        description: pipeline.description,
        project_id: pipeline.project_id ?? undefined,
        concurrency_group: pipeline.concurrency_group,
        concurrency_policy: pipeline.concurrency_policy,
        max_concurrent_runs: pipeline.max_concurrent_runs,
      });
    } else {
      form.resetFields();
      setStepsJson(STEPS_TEMPLATE);
    }
  }, [open, pipeline, form]);

  // ─── Load existing version ────────────────────────────────────────────────

  const { data: versionData, isLoading: versionLoading } = useQuery({
    queryKey: ['pipeline-version', pipeline?.id, pipeline?.current_version_id],
    queryFn: async () => {
      const res = await pipelineService.listVersions(pipeline!.id);
      const versions = res.items ?? [];
      const current =
        versions.find((v) => v.id === pipeline!.current_version_id) ??
        versions[versions.length - 1];
      return current ?? null;
    },
    enabled: isEdit && pipeline?.current_version_id != null && open,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (versionData?.steps_json) {
      setStepsJson(versionData.steps_json);
    }
  }, [versionData]);

  // ─── Unified save ─────────────────────────────────────────────────────────

  const handleSave = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);

      if (isEdit) {
        // 1. Update Pipeline metadata
        await pipelineService.update(pipeline!.id, values as UpdatePipelineRequest);
        // 2. Create new Version (steps)
        await pipelineService.createVersion(pipeline!.id, { steps_json: stepsJson });
        message.success(t('pipeline:messages.updateSuccess'));
      } else {
        // 1. Create Pipeline
        const created = await pipelineService.create(values as CreatePipelineRequest);
        // 2. Create Version for the new Pipeline
        await pipelineService.createVersion(created.id, { steps_json: stepsJson });
        message.success(t('pipeline:messages.createSuccess'));
      }

      queryClient.invalidateQueries({ queryKey: ['pipelines'] });
      onSuccess();
    } catch (err) {
      const msg = parseApiError(err);
      if (msg) {
        message.error(msg);
      }
      // else: Ant Design form validation shows inline errors
    } finally {
      setSaving(false);
    }
  }, [form, isEdit, pipeline, stepsJson, t, message, queryClient, onSuccess]);

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
      {/* ─── Pipeline 基本設定 ──────────────────────────────────────────── */}
      <Form form={form} layout="vertical" disabled={saving}>
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

        <Form.Item
          name="project_id"
          label={t('pipeline:form.project', { defaultValue: '關聯 Project（Git Repo）' })}
          tooltip={t('pipeline:form.projectTooltip', { defaultValue: '選擇 Git 倉庫，Pipeline 執行時會自動 clone 原始碼' })}
        >
          <Select
            options={projectOptions}
            allowClear
            showSearch
            placeholder={t('pipeline:form.projectPlaceholder', { defaultValue: '選擇 Project' })}
            filterOption={(input, opt) =>
              String(opt?.label ?? '').toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>

        <Form.Item name="description" label={t('pipeline:form.description')}>
          <Input.TextArea
            rows={2}
            placeholder={t('pipeline:form.descriptionPlaceholder')}
          />
        </Form.Item>

        <Flex gap={token.marginMD}>
          <Form.Item
            name="concurrency_policy"
            label={t('pipeline:form.concurrencyPolicy')}
            initialValue="queue"
            style={{ flex: 1 }}
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
            style={{ width: 140 }}
          >
            <InputNumber min={1} max={50} style={{ width: '100%' }} />
          </Form.Item>
        </Flex>
      </Form>

      {/* ─── Steps 定義 ──────────────────────────────────────────────── */}
      <Divider style={{ marginTop: 0 }} />

      <Text strong style={{ display: 'block', marginBottom: token.marginSM }}>
        {t('pipeline:editor.stepsTitle', { defaultValue: 'Steps 定義（JSON）' })}
      </Text>

      <Spin spinning={versionLoading}>
        <Editor
          height="380px"
          defaultLanguage="json"
          value={stepsJson}
          onChange={(val) => setStepsJson(val ?? '')}
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

      {/* ─── 操作按鈕 ──────────────────────────────────────────────── */}
      <Flex
        justify="flex-end"
        gap={token.marginSM}
        style={{ marginTop: token.marginMD }}
      >
        <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
        <Button type="primary" loading={saving} onClick={handleSave}>
          {isEdit ? t('common:actions.save') : t('common:actions.create')}
        </Button>
      </Flex>
    </Drawer>
  );
};

export default PipelineEditor;
