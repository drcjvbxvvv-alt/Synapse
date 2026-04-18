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
  Switch,
  Segmented,
} from 'antd';
import { CodeOutlined, FormOutlined } from '@ant-design/icons';
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
import { clusterService } from '../../services/clusterService';

const { Text } = Typography;

// ─── Steps Template ─────────────────────────────────────────────────────────

const STEPS_TEMPLATE = `[
  {
    "name": "build",
    "type": "build-image",
    "config": {
      "context": ".",
      "dockerfile": "Dockerfile",
      "destination": "harbor.example.com/project/myapp:latest",
      "registry": "my-harbor"
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
  const [stepsMode, setStepsMode] = useState<'form' | 'json'>('form');

  // ─── Build-image form state ─────────────────────────────────────────────
  const [buildForm] = Form.useForm();

  // Sync JSON → form (parse build-image step from JSON)
  const syncJsonToForm = (json: string) => {
    try {
      const steps = JSON.parse(json);
      if (!Array.isArray(steps)) return;
      const buildStep = steps.find((s: Record<string, unknown>) => s.type === 'build-image');
      if (buildStep?.config) {
        const cfg = buildStep.config as Record<string, unknown>;
        const registryName = (cfg.registry as string) ?? '';
        let dest = (cfg.destination as string) ?? '';
        // Strip registry URL prefix from destination for form display
        if (registryName) {
          const regUrl = getRegistryUrl(registryName);
          if (regUrl && dest.startsWith(regUrl + '/')) {
            dest = dest.slice(regUrl.length + 1);
          }
        }
        buildForm.setFieldsValue({
          destination: dest,
          registry: registryName || undefined,
          dockerfile: (cfg.dockerfile as string) ?? 'Dockerfile',
          context: (cfg.context as string) ?? '.',
        });
      }
    } catch { /* ignore */ }
  };

  // Get registry URL by name
  const getRegistryUrl = (name: string): string => {
    const reg = (registriesData?.items ?? []).find((r) => r.name === name);
    if (!reg) return '';
    let url = (reg as Record<string, unknown>).url as string ?? '';
    // Remove protocol prefix for docker image address
    url = url.replace(/^https?:\/\//, '');
    // Remove trailing slash
    url = url.replace(/\/$/, '');
    return url;
  };

  // Sync form → JSON
  const syncFormToJson = () => {
    const vals = buildForm.getFieldsValue();
    // Auto-prepend registry URL to destination
    let fullDest = vals.destination || '';
    if (vals.registry) {
      const registryUrl = getRegistryUrl(vals.registry);
      if (registryUrl && !fullDest.includes(registryUrl)) {
        fullDest = `${registryUrl}/${fullDest}`.replace(/\/+/g, '/').replace(/^\//, '');
      }
    }
    const config: Record<string, unknown> = {
      context: vals.context || '.',
      dockerfile: vals.dockerfile || 'Dockerfile',
      destination: fullDest,
    };
    if (vals.registry) config.registry = vals.registry;
    const steps = [{ name: 'build', type: 'build-image', config }];
    const json = JSON.stringify(steps, null, 2);
    setStepsJson(json);
  };

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

  // Load clusters for build environment selector
  const { data: clustersData } = useQuery({
    queryKey: ['clusters-list'],
    queryFn: () => clusterService.getClusters({ pageSize: 200 }),
    staleTime: 60_000,
    enabled: open,
  });
  const clusterOptions = (clustersData?.items ?? []).map((c) => ({
    label: c.name,
    value: Number(c.id),
  }));

  // Load registries for build-image form
  const { data: registriesData } = useQuery({
    queryKey: ['registries-list'],
    queryFn: () => request.get<{ items: Array<{ id: number; name: string; enabled: boolean }> }>('/system/settings/registries'),
    staleTime: 60_000,
    enabled: open,
  });
  const registryOptions = (registriesData?.items ?? [])
    .filter((r) => r.enabled)
    .map((r) => ({ label: r.name, value: r.name }));

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
        build_cluster_id: pipeline.build_cluster_id ?? undefined,
        build_namespace: pipeline.build_namespace ?? '',
        approval_enabled: pipeline.approval_enabled ?? false,
        scan_enabled: pipeline.scan_enabled ?? false,
      });
    } else {
      form.resetFields();
      buildForm.resetFields();
      setStepsJson(STEPS_TEMPLATE);
      syncJsonToForm(STEPS_TEMPLATE);
      setStepsMode('form');
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
      syncJsonToForm(versionData.steps_json);
    }
  }, [versionData]); // eslint-disable-line react-hooks/exhaustive-deps

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

        <Divider orientation="left" plain style={{ margin: `${token.marginSM}px 0` }}>
          {t('pipeline:form.buildEnv', { defaultValue: '構建環境' })}
        </Divider>
        <Flex gap={token.marginMD}>
          <Form.Item
            name="build_cluster_id"
            label={t('pipeline:form.buildCluster', { defaultValue: '構建叢集' })}
            tooltip={t('pipeline:form.buildClusterTip', { defaultValue: '構建 Job 執行的叢集，設定後可一鍵觸發' })}
            style={{ flex: 1 }}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Select
              options={clusterOptions}
              showSearch
              placeholder={t('pipeline:form.buildClusterPlaceholder', { defaultValue: '選擇叢集' })}
              filterOption={(input, opt) =>
                String(opt?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item
            name="build_namespace"
            label={t('pipeline:form.buildNamespace', { defaultValue: '構建 Namespace' })}
            tooltip={t('pipeline:form.buildNamespaceTip', { defaultValue: '構建 Job 執行的 Namespace' })}
            style={{ flex: 1 }}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder="e.g. ci-builds" />
          </Form.Item>
        </Flex>

        <Flex gap={token.marginLG}>
          <Form.Item
            name="scan_enabled"
            label={t('pipeline:form.scanEnabled', { defaultValue: '安全掃描' })}
            valuePropName="checked"
            initialValue={false}
            tooltip={t('pipeline:form.scanEnabledTip', { defaultValue: '開啟後，構建映像完成後自動執行 Trivy 安全掃描（不阻斷流程）' })}
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="approval_enabled"
            label={t('pipeline:form.approvalEnabled', { defaultValue: '部署審核' })}
            valuePropName="checked"
            initialValue={false}
            tooltip={t('pipeline:form.approvalEnabledTip', { defaultValue: '開啟後，deploy 類步驟執行前需人工審核通過' })}
          >
            <Switch />
          </Form.Item>
        </Flex>
      </Form>

      {/* ─── Steps 定義 ──────────────────────────────────────────────── */}
      <Divider style={{ marginTop: 0 }} />

      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginSM }}>
        <Text strong>
          {t('pipeline:editor.stepsTitle', { defaultValue: '構建步驟' })}
        </Text>
        <Segmented
          size="small"
          value={stepsMode}
          onChange={(v) => {
            const mode = v as 'form' | 'json';
            if (mode === 'json') syncFormToJson();
            if (mode === 'form') syncJsonToForm(stepsJson);
            setStepsMode(mode);
          }}
          options={[
            { label: t('pipeline:form.tabs.form'), value: 'form', icon: <FormOutlined /> },
            { label: 'JSON', value: 'json', icon: <CodeOutlined /> },
          ]}
        />
      </Flex>

      <Spin spinning={versionLoading}>
        {stepsMode === 'form' ? (
          <Form
            form={buildForm}
            layout="vertical"
            disabled={saving}
            onValuesChange={() => syncFormToJson()}
            initialValues={{ context: '.', dockerfile: 'Dockerfile' }}
          >
            <Form.Item
              name="registry"
              label={t('pipeline:stepForm.registry', { defaultValue: '映像倉庫' })}
              tooltip={t('pipeline:stepForm.registryTip', { defaultValue: '選擇已配置的 Harbor / Registry，自動注入位址和帳密' })}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Select
                options={registryOptions}
                placeholder={t('pipeline:stepForm.registryPlaceholder', { defaultValue: '選擇映像倉庫' })}
              />
            </Form.Item>

            <Form.Item
              name="destination"
              label={t('pipeline:stepForm.destination', { defaultValue: '映像名稱' })}
              tooltip={t('pipeline:stepForm.destinationTip', { defaultValue: '專案名/映像名:標籤（系統自動拼接 Registry 位址）' })}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Input placeholder="saas/myapp:latest" />
            </Form.Item>

            <Flex gap={token.marginMD}>
              <Form.Item
                name="dockerfile"
                label="Dockerfile"
                style={{ flex: 1 }}
              >
                <Input placeholder="Dockerfile" />
              </Form.Item>
              <Form.Item
                name="context"
                label={t('pipeline:stepForm.context', { defaultValue: 'Build Context' })}
                style={{ flex: 1 }}
              >
                <Input placeholder="." />
              </Form.Item>
            </Flex>
          </Form>
        ) : (
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
        )}
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
